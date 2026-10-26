package diag

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"networktoolkit/internal/deviceinfo"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

const (
	maxSubnetHosts     = 1024
	maxSubnetHostPorts = 16384
)

// SubnetHost retains every started address, including unsuccessful probes and
// partial work on cancellation. Responded describes ICMP, TCP, or UPnP evidence; a
// cancelled row can retain true response evidence from its completed probes.
type SubnetHost struct {
	HostIdentity
	DeviceIdentity
	HostnameIdentity
	Address    string               `json:"address"`
	Status     string               `json:"status"`
	Enriching  bool                 `json:"enriching,omitempty"`
	Completed  bool                 `json:"completed"`
	Responded  bool                 `json:"responded"`
	Evidence   []string             `json:"evidence"`
	LatencyMs  *float64             `json:"latencyMs"`
	OpenPorts  []int                `json:"openPorts"`
	Samples    []platform.PingReply `json:"samples,omitempty"`
	Statistics *Statistics          `json:"statistics,omitempty"`
	Ports      []PortResult         `json:"ports,omitempty"`
	ICMPError  string               `json:"icmpError,omitempty"`
}

type SubnetProgress struct {
	Total           int `json:"total"`
	Started         int `json:"started"`
	Completed       int `json:"completed"`
	Responded       int `json:"responded"`
	NotStarted      int `json:"notStarted"`
	RequestedProbes int `json:"requestedProbes"`
	CompletedProbes int `json:"completedProbes"`
}

// Discovery keeps its established event names. The scope marker distinguishes
// all-host live results from older responder-only reports before a final result
// exists; every progress event repeats it so bounded event histories retain it.
type discoveryProgress struct {
	Scope string `json:"scope"`
	SubnetProgress
}

type SubnetResult struct {
	Source *SourceMetadata `json:"source,omitempty"`
	SubnetProgress
	Scope             string         `json:"scope"`
	Tool              string         `json:"tool"`
	Range             string         `json:"range"`
	Network           string         `json:"network"`
	Scopes            []string       `json:"scopes,omitempty"`
	Cancelled         bool           `json:"cancelled"`
	ElapsedMs         float64        `json:"elapsedMs"`
	Hosts             []SubnetHost   `json:"hosts"`
	PortsRequested    []int          `json:"portsRequested,omitempty"`
	LocalContext      map[string]any `json:"localContext,omitempty"`
	LocalContextError string         `json:"localContextError,omitempty"`
	NeighborNote      string         `json:"neighborNote,omitempty"`
	Identify          bool           `json:"identify"`
	ResolveHostnames  bool           `json:"resolveHostnames"`
	Note              string         `json:"note"`
}

type subnetOptions struct {
	targets           []netip.Addr
	network           string
	scopes            []string
	ports             []int
	icmp              bool
	identify          bool
	resolveHostnames  bool
	timeout           time.Duration
	concurrency       int
	parallelDiscovery bool
}

// Injectable operations keep scheduler tests deterministic and require no
// external network traffic. Production always uses the existing probe helpers.
type subnetProbes struct {
	ping      func(context.Context, string, time.Duration, int) (platform.PingReply, error)
	tcp       func(context.Context, string, int, time.Duration) PortResult
	system    func(context.Context) (map[string]any, error)
	neighbors func(context.Context) ([]platform.Neighbor, error)
	device    deviceProbe
	hostname  reverseLookup
}

func parseSubnetOptions(req model.Request) (subnetOptions, error) {
	var options subnetOptions
	if req.Tool == "quick" {
		return options, errDeviceCheckSubnet
	}
	plan, err := parseIPv4Scopes(req.Target, maxSubnetHosts)
	if err != nil {
		return options, err
	}
	options.targets, options.network, options.scopes = plan.targets, plan.network, plan.scopes
	defaultTimeout, minimumTimeout, defaultConcurrency := 1000, 100, 32
	defaultPorts, portLimit := "", 0
	switch req.Tool {
	case "ping":
		options.icmp = true
		defaultTimeout, minimumTimeout = 1500, 200
	case "discover":
		options.parallelDiscovery = true
		options.icmp = true
		defaultTimeout, defaultConcurrency = 400, 16
		defaultPorts, portLimit = "80,443,22,445,3389", 64
	case "tcp":
		defaultPorts, portLimit = "22,53,80,443,445,3389", 1024
	default:
		return options, errors.New("subnet scope is supported by Network discovery, Ping, and TCP ports")
	}
	if portLimit > 0 {
		// Preserve raw text until the parser enforces its input-size limit.
		portSpec, ok := req.Options["ports"].(string)
		if !ok {
			portSpec = defaultPorts
		}
		options.ports, err = parsePorts(portSpec, portLimit)
		if err != nil {
			return options, err
		}
	}
	// Ping subnet mode remains exactly one ICMP probe, even if an old or
	// manually constructed request also carries the identify option.
	options.identify = req.Tool != "ping" && boolOption(req.Options, "identify", req.Tool == "discover")
	options.resolveHostnames = req.Tool == "discover" && boolOption(req.Options, "resolveHostnames", true)
	if len(options.targets)*len(options.ports) > maxSubnetHostPorts {
		return options, fmt.Errorf("the selected scopes and port list require %d TCP connections; network scans are limited to %d host-port connections per run; select fewer ports or addresses", len(options.targets)*len(options.ports), maxSubnetHostPorts)
	}
	options.timeout, err = durationOption(req.Options, "timeoutMs", defaultTimeout, minimumTimeout, 10000)
	if err != nil {
		return options, err
	}
	options.concurrency, err = intOption(req.Options, "concurrency", defaultConcurrency, 1, 64)
	return options, err
}

func subnetScan(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	return runSubnetScan(ctx, req, emit, subnetProbes{
		ping: platform.Ping, tcp: dialPort, system: platform.System, neighbors: platform.Neighbors,
		device: deviceinfo.Probe, hostname: systemReverseLookup,
	})
}

type subnetUpdate struct {
	started bool
	probes  int
	host    *SubnetHost
}

// Host admission stays bounded. Discovery overlaps independent operations
// through a separate bounded semaphore; other sweeps keep their original pool.
func scanSubnetHosts(ctx context.Context, options subnetOptions, probes subnetProbes, consume func(subnetUpdate)) {
	workers := min(options.concurrency, len(options.targets))
	slots := make(chan struct{}, min(4*options.concurrency, 64))
	jobs := make(chan netip.Addr)
	updates := make(chan subnetUpdate, 2*workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for address := range jobs {
				if ctx.Err() != nil {
					return
				}
				updates <- subnetUpdate{started: true}
				complete := func() { updates <- subnetUpdate{probes: 1} }
				var host SubnetHost
				if options.parallelDiscovery && options.concurrency > 1 {
					host = probeDiscoveryHost(ctx, address.String(), options, probes, slots, complete, func(early SubnetHost) { updates <- subnetUpdate{host: &early} })
				} else {
					host = probeSubnetHost(ctx, address.String(), options, probes, complete)
				}
				// Retain this send after cancellation. The coordinator drains all
				// workers so that observations from incomplete hosts survive.
				updates <- subnetUpdate{host: &host}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, address := range options.targets {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- address:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(updates)
	}()
	for update := range updates {
		consume(update)
	}
}

func runSubnetScan(ctx context.Context, req model.Request, emit model.Emit, probes subnetProbes) (model.Result, error) {
	options, err := parseSubnetOptions(req)
	if err != nil {
		return model.Result{}, err
	}
	start := time.Now()
	perHost := len(options.ports)
	if options.icmp {
		perHost++ // Subnet Ping and Discovery each perform one ICMP attempt.
	}
	if options.identify {
		perHost++ // One separately bounded UPnP identification attempt per host.
	}
	if options.resolveHostnames {
		perHost++ // One bounded configured reverse lookup, not response evidence.
	}
	data := SubnetResult{
		Source: sourceMetadata(ctx),
		Scope:  "subnet", Tool: req.Tool, Range: strings.TrimSpace(req.Target), Network: options.network, Scopes: options.scopes,
		Hosts: []SubnetHost{}, PortsRequested: options.ports, Identify: options.identify,
		ResolveHostnames: options.resolveHostnames,
		SubnetProgress:   SubnetProgress{Total: len(options.targets), NotStarted: len(options.targets), RequestedProbes: len(options.targets) * perHost},
		Note:             subnetNote(req.Tool),
	}
	if options.identify {
		data.Note += " Device identification adds one unicast UPnP request and at most one same-device description retrieval per address, within a 2.5-second total device budget. Reported models are optional and are never guessed from a MAC or port."
	}
	if options.resolveHostnames {
		data.Note += " Hostnames use one reverse lookup through configured DNS servers and the hosts file per address, with a 1.5-second lookup limit. Independent discovery checks overlap within bounded host and operation limits. Resolved or cached names do not establish that a device responded."
	}
	hostKind, progressKind := "subnet-host", "subnet-progress"
	if req.Tool == "discover" {
		hostKind, progressKind = "host", "progress"
	}
	publishProgress := func(message string) {
		var progress any = data.SubnetProgress
		if req.Tool == "discover" {
			progress = discoveryProgress{Scope: "subnet", SubnetProgress: data.SubnetProgress}
		}
		event(emit, progressKind, "info", message, progress)
	}
	result := func() model.Result {
		data.ElapsedMs = ms(time.Since(start))
		data.Cancelled = ctx.Err() != nil
		data.NotStarted = data.Total - data.Started
		return model.Result{
			Summary: fmt.Sprintf("%d addresses gave response evidence; %d of %d completed", data.Responded, data.Completed, data.Total),
			Data:    data,
		}
	}
	if err := ctx.Err(); err != nil {
		return result(), err
	}
	publishProgress(fmt.Sprintf("0 of %d addresses completed", data.Total))
	known, neighborErr := readNeighborIdentities(ctx, probes.neighbors)
	if neighborErr != nil && ctx.Err() == nil {
		data.NeighborNote = "Initial neighbor cache: " + neighborErr.Error()
	}
	if req.Tool == "discover" && ctx.Err() == nil {
		data.LocalContext, err = probes.system(ctx)
		if err != nil {
			data.LocalContextError = err.Error()
			model.Log(emit, "warning", "Local context could not be collected: "+err.Error(), nil)
		}
	}
	if err := ctx.Err(); err != nil {
		return result(), err
	}
	model.Log(emit, "info", fmt.Sprintf("Testing %d unique IPv4 addresses from %s with at most %d concurrent host probes", data.Total, data.Network, options.concurrency), nil)

	lastProgress := time.Now()
	hostIndex := map[string]int{}
	scanSubnetHosts(ctx, options, probes, func(update subnetUpdate) {
		if update.started {
			data.Started++
		}
		data.CompletedProbes += update.probes
		if update.host != nil {
			host := *update.host
			host.HostIdentity = known[host.Address]
			previous := SubnetHost{}
			if index, ok := hostIndex[host.Address]; ok {
				previous = data.Hosts[index]
				data.Hosts[index] = host
			} else {
				hostIndex[host.Address] = len(data.Hosts)
				data.Hosts = append(data.Hosts, host)
			}
			if host.Completed && !previous.Completed {
				data.Completed++
			}
			if host.Responded && !previous.Responded {
				data.Responded++
			}
			emitSubnetHost(emit, hostKind, host)
		}
		data.NotStarted = data.Total - data.Started
		// Per-probe counts remain exact while progress messages are bounded.
		// Immutable reachability snapshots may precede optional metadata updates.
		if time.Since(lastProgress) >= 200*time.Millisecond {
			publishProgress(fmt.Sprintf("%d of %d addresses completed; %d of %d probes finished", data.Completed, data.Total, data.CompletedProbes, data.RequestedProbes))
			lastProgress = time.Now()
		}
	})

	// A final read-only cache refresh can capture MAC addresses learned by the
	// probes. It does not send traffic, and a short independent deadline permits
	// enrichment of partial results even when the diagnostic was cancelled.
	refreshed, refreshErr := refreshNeighborIdentities(ctx, probes.neighbors)
	if refreshErr != nil {
		data.NeighborNote = joinIdentityNote(data.NeighborNote, "Final neighbor cache: "+refreshErr.Error())
	}
	for i := range data.Hosts {
		host := &data.Hosts[i]
		if identity, ok := refreshed[host.Address]; ok && identity != host.HostIdentity {
			host.HostIdentity = identity
			emitSubnetHost(emit, hostKind, *host)
		}
	}
	sort.Slice(data.Hosts, func(i, j int) bool {
		return netip.MustParseAddr(data.Hosts[i].Address).Less(netip.MustParseAddr(data.Hosts[j].Address))
	})
	publishProgress(fmt.Sprintf("%d of %d addresses completed; %d gave response evidence", data.Completed, data.Total, data.Responded))
	return result(), ctx.Err()
}

func probeSubnetHost(ctx context.Context, address string, options subnetOptions, probes subnetProbes, completedProbe func()) SubnetHost {
	host := SubnetHost{Address: address, Evidence: []string{}, OpenPorts: []int{}, Completed: true}
	host.HostnameIdentity = disabledHostname()
	if options.resolveHostnames {
		host.HostnameIdentity = HostnameIdentity{HostnameStatus: "cancelled", HostnameNote: "Hostname lookup did not run before cancellation."}
	}
	finish := func() SubnetHost {
		if options.icmp {
			stats := statistics(host.Samples)
			host.Statistics = &stats
		}
		host.Responded = len(host.Evidence) > 0
		switch {
		case !host.Completed:
			host.Status = "cancelled"
		case host.Responded:
			host.Status = "responded"
		case host.ICMPError != "":
			host.Status = "error"
		default:
			host.Status = "no_response"
			for _, port := range host.Ports {
				if port.State == "error" {
					host.Status = "error"
					break
				}
			}
		}
		return host
	}
	if options.icmp {
		if ctx.Err() != nil {
			host.Completed = false
			return finish()
		}
		reply, err := probes.ping(ctx, address, options.timeout, 128)
		reply.Sequence = 1
		if err != nil {
			reply.Success = false
			reply.Error = err.Error()
			reply.Status = "error"
			host.ICMPError = err.Error()
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				reply.Status = "cancelled"
				host.Completed = false
			}
		}
		host.Samples = append(host.Samples, reply)
		if reply.Success {
			host.Evidence = append(host.Evidence, "ICMP echo reply")
			latency := reply.RTTMs
			host.LatencyMs = &latency
		}
		completedProbe()
		if !host.Completed {
			return finish()
		}
	}
	for _, port := range options.ports {
		if ctx.Err() != nil {
			host.Completed = false
			return finish()
		}
		result := probes.tcp(ctx, address, port, options.timeout)
		host.Ports = append(host.Ports, result)
		if result.State == "open" || result.State == "closed" {
			state := "open"
			if result.State == "closed" {
				state = "refused"
			} else {
				host.OpenPorts = append(host.OpenPorts, port)
			}
			host.Evidence = append(host.Evidence, fmt.Sprintf("TCP %d %s", port, state))
			if host.LatencyMs == nil {
				latency := result.LatencyMs
				host.LatencyMs = &latency
			}
		}
		completedProbe()
		if result.State == "cancelled" || (ctx.Err() != nil && result.State == "timeout") {
			host.Completed = false
			return finish()
		}
	}
	if options.identify {
		if ctx.Err() != nil {
			host.Completed = false
			return finish()
		}
		info, err := probes.device(ctx, address, deviceProbeTimeout)
		host.DeviceIdentity = deviceIdentity(info, err)
		if info.Responded {
			host.Evidence = append(host.Evidence, "UPnP discovery reply")
		}
		completedProbe()
		if ctx.Err() != nil {
			host.Completed = false
		}
	}
	if options.resolveHostnames {
		if ctx.Err() != nil {
			host.Completed = false
			return finish()
		}
		host.HostnameIdentity = resolveHostname(ctx, address, probes.hostname)
		completedProbe()
		if ctx.Err() != nil {
			host.Completed = false
		}
	}
	return finish()
}

func emitSubnetHost(emit model.Emit, kind string, host SubnetHost) {
	level := "warning"
	if host.Responded {
		level = "success"
	} else if host.Status == "error" {
		level = "error"
	}
	message := host.Address + ": " + strings.ReplaceAll(host.Status, "_", " ")
	if host.Responded {
		message = host.Address + ": " + strings.Join(host.Evidence, "; ")
		if host.Enriching {
			message += "; device details pending"
		} else if !host.Completed {
			message += "; remaining probes cancelled"
		}
	}
	event(emit, kind, level, message, host)
}

func subnetNote(tool string) string {
	const common = " Every unique IPv4 address is tested directly, with overlapping scopes counted once. No response alone does not establish that a device is offline. Network and wildcard scopes exclude network and broadcast addresses through /30; both /31 addresses and a single /32 address are eligible. Explicit hosts and range endpoints are retained. MAC and vendor values use exact-IP neighbor cache entries only and may be stale. Routed targets may not expose their own MAC. A vendor identifies a MAC allocation, not a device model."
	switch tool {
	case "ping":
		return "One ICMP echo request is attempted per address. The single-host count and interval settings do not apply to a subnet sweep." + common
	case "discover":
		return "One ICMP echo and every selected TCP port are attempted per address, without requiring an ICMP reply first. Every started address is retained, including no-response, error, and partially cancelled hosts. Literal IPv4 targets require no forward DNS lookup; local context is collected once. An open or refused TCP connection is response evidence and can originate from a firewall. Service names are conventional port labels, not detected applications." + common
	default:
		return "Every selected TCP port is tested without ICMP pre-discovery. Open means a connection succeeded; closed means it was refused. Refusals are response evidence and can originate from a firewall. Timeouts are inconclusive. Service names are conventional port labels, not detected applications. The selected port checks use TCP only." + common
	}
}
