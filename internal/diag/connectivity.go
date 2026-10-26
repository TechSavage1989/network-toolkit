package diag

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

type Statistics struct {
	Sent        int      `json:"sent"`
	Received    int      `json:"received"`
	Lost        int      `json:"lost"`
	LossPercent float64  `json:"lossPercent"`
	MinMs       *float64 `json:"minMs"`
	MaxMs       *float64 `json:"maxMs"`
	AvgMs       *float64 `json:"avgMs"`
	JitterMs    *float64 `json:"jitterMs"`
}

func statistics(samples []platform.PingReply) Statistics {
	s := Statistics{Sent: len(samples)}
	var sum, min, max, jitter float64
	var previous *float64
	jitterPairs := 0
	for _, p := range samples {
		if !p.Success {
			previous = nil
			continue
		}
		s.Received++
		sum += p.RTTMs
		if s.Received == 1 || p.RTTMs < min {
			min = p.RTTMs
		}
		if p.RTTMs > max {
			max = p.RTTMs
		}
		if previous != nil {
			jitter += math.Abs(p.RTTMs - *previous)
			jitterPairs++
		}
		v := p.RTTMs
		previous = &v
	}
	s.Lost = s.Sent - s.Received
	if s.Sent > 0 {
		s.LossPercent = 100 * float64(s.Lost) / float64(s.Sent)
	}
	if s.Received > 0 {
		avg := sum / float64(s.Received)
		s.MinMs = &min
		s.MaxMs = &max
		s.AvgMs = &avg
	}
	if jitterPairs > 0 {
		j := jitter / float64(jitterPairs)
		s.JitterMs = &j
	}
	return s
}

func ping(ctx context.Context, req model.Request, emit model.Emit, monitoring bool) (model.Result, error) {
	host, err := validateHost(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	defaultCount, minInterval := 10, 200
	if monitoring {
		defaultCount, minInterval = 60, 500
	}
	count, err := intOption(req.Options, "count", defaultCount, 0, 1000000000)
	if err != nil {
		return model.Result{}, err
	}
	interval, err := durationOption(req.Options, "intervalMs", 1000, minInterval, 10000)
	if err != nil {
		return model.Result{}, err
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 1500, 200, 10000)
	if err != nil {
		return model.Result{}, err
	}
	addresses, err := resolveHost(ctx, host, 5*time.Second)
	if err != nil {
		return model.Result{}, fmt.Errorf("resolve target: %w", err)
	}
	address := addresses[0]
	series := &sampleSeries{}
	result := func() model.Result {
		s := series.statistics()
		return model.Result{Summary: fmt.Sprintf("%d of %d ICMP replies, %.1f%% no response", s.Received, s.Sent, s.LossPercent), Data: map[string]any{"target": host, "address": address, "addresses": addresses, "samples": series.samples(), "statistics": s, "continuous": count == 0, "retainedSampleLimit": MaxRetainedSamples, "samplesOmitted": s.Sent - len(series.rows), "note": "ICMP can be filtered. No response alone does not establish that a device is offline. Jitter is the mean absolute RTT difference between consecutive successful samples. Lifetime statistics cover the complete run; at most the latest 10,000 individual samples are retained."}}
	}
	message := fmt.Sprintf("Sending %d ICMP echo requests to %s", count, address)
	if count == 0 {
		message = "Sending ICMP echo requests to " + address + " until stopped"
	}
	model.Log(emit, "info", message, nil)
	err = series.collect(ctx, count, interval, "icmp", 0, emit, func(ctx context.Context) (platform.PingReply, error) {
		return platform.Ping(ctx, address, timeout, 128)
	})
	return result(), err
}

func monitor(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	mode := stringOption(req.Options, "mode", "icmp")
	if mode == "icmp" {
		return ping(ctx, req, emit, true)
	}
	if mode != "tcp" {
		return model.Result{}, errors.New("monitor mode must be icmp or tcp")
	}
	host, err := validateHost(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	port, err := intOption(req.Options, "port", 443, 1, 65535)
	if err != nil {
		return model.Result{}, err
	}
	count, err := intOption(req.Options, "count", 60, 0, 1000000000)
	if err != nil {
		return model.Result{}, err
	}
	interval, err := durationOption(req.Options, "intervalMs", 1000, 500, 10000)
	if err != nil {
		return model.Result{}, err
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 1500, 200, 10000)
	if err != nil {
		return model.Result{}, err
	}
	probeHost := host
	if _, selected := platform.SourceBindingFromContext(ctx); selected {
		addresses, resolveErr := resolveHost(ctx, host, 5*time.Second)
		if resolveErr != nil {
			return model.Result{}, fmt.Errorf("resolve monitor target: %w", resolveErr)
		}
		probeHost = addresses[0]
	}
	series := &sampleSeries{}
	result := func() model.Result {
		s := series.statistics()
		return model.Result{Summary: fmt.Sprintf("%d of %d TCP connections to port %d succeeded", s.Received, s.Sent, port), Data: map[string]any{"target": host, "mode": "tcp", "port": port, "samples": series.samples(), "statistics": s, "continuous": count == 0, "retainedSampleLimit": MaxRetainedSamples, "samplesOmitted": s.Sent - len(series.rows), "note": "Samples measure TCP connection establishment, not ICMP latency or application response time. Failed connections do not by themselves prove a host outage. Lifetime statistics cover the complete run; at most the latest 10,000 individual samples are retained."}}
	}
	err = series.collect(ctx, count, interval, "tcp", port, emit, func(ctx context.Context) (platform.PingReply, error) {
		p := dialPort(ctx, probeHost, port, timeout)
		return platform.PingReply{Time: p.Time, Address: p.Address, Success: p.State == "open", RTTMs: p.LatencyMs, Status: p.State, Error: p.Error}, nil
	})
	return result(), err
}

type TraceHop struct {
	HostnameIdentity
	Time    string               `json:"time,omitempty"`
	Hop     int                  `json:"hop"`
	Address string               `json:"address"`
	Status  string               `json:"status"`
	Reached bool                 `json:"reached"`
	RTTMs   *float64             `json:"rttMs"`
	Samples []platform.PingReply `json:"samples"`
}

func trace(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	return runTrace(ctx, req, emit, platform.Ping, systemReverseLookup)
}

func runTrace(ctx context.Context, req model.Request, emit model.Emit, probe func(context.Context, string, time.Duration, int) (platform.PingReply, error), lookup reverseLookup) (model.Result, error) {
	host, err := validateHost(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	maxHops, err := intOption(req.Options, "maxHops", 24, 1, 64)
	if err != nil {
		return model.Result{}, err
	}
	probes, err := intOption(req.Options, "probes", 1, 1, 3)
	if err != nil {
		return model.Result{}, err
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 4000, 200, 10000)
	if err != nil {
		return model.Result{}, err
	}
	addresses, err := resolveHost(ctx, host, 5*time.Second)
	if err != nil {
		return model.Result{}, err
	}
	address := addresses[0]
	hops := make([]TraceHop, 0, maxHops)
	reached := false
	resolveNames := boolOption(req.Options, "resolveHostnames", true)
	hostnameCache := map[string]HostnameIdentity{}
	enrichHop := func(hop *TraceHop) {
		hop.HostnameIdentity = disabledHostname()
		if !resolveNames {
			return
		}
		hop.HostnameIdentity = HostnameIdentity{HostnameStatus: "not_resolved", HostnameNote: "No responding IP was available for reverse lookup."}
		if hop.Address == "" {
			return
		}
		key := hop.Address
		if ip, parseErr := netip.ParseAddr(key); parseErr == nil {
			key = ip.Unmap().WithZone("").String()
		}
		identity, exists := hostnameCache[key]
		if !exists {
			identity = resolveHostname(ctx, hop.Address, lookup)
			hostnameCache[key] = identity
		}
		hop.HostnameIdentity = identity
	}
	result := func() model.Result {
		summary := fmt.Sprintf("Target did not reply within %d tested hops", len(hops))
		if reached {
			summary = fmt.Sprintf("Target replied at hop %d", len(hops))
		}
		note := "This is an ICMP trace. Routers may suppress or rate-limit TTL-expired messages, so unanswered hops do not prove a route failure. Different probes can take different paths."
		if resolveNames {
			note += " Each distinct responding IP has one reverse lookup with a 1.5-second limit. Missing names leave the IP and probe timings intact; resolver names may be cached and are not additional response evidence."
		}
		return model.Result{Summary: summary, Data: map[string]any{"target": host, "address": address, "hops": hops, "reached": reached, "resolveHostnames": resolveNames, "note": note}}
	}
	for ttl := 1; ttl <= maxHops; ttl++ {
		hop := TraceHop{Hop: ttl, Status: "timeout", Samples: make([]platform.PingReply, 0, probes)}
		for p := 0; p < probes; p++ {
			if err = ctx.Err(); err != nil {
				if len(hop.Samples) != 0 {
					enrichHop(&hop)
					hops = append(hops, hop)
					event(emit, "hop", "warning", fmt.Sprintf("Hop %d: stopped; collected replies retained", ttl), hop)
				}
				return result(), err
			}
			r, probeErr := probe(ctx, address, timeout, ttl)
			stampReply(&r)
			hop.Time = laterMeasurementTime(hop.Time, r.Time)
			r.Sequence = p + 1
			if probeErr != nil {
				r.Status = "error"
				r.Error = probeErr.Error()
			}
			hop.Samples = append(hop.Samples, r)
			if r.Address != "" {
				hop.Address = r.Address
				hop.Status = r.Status
				v := r.RTTMs
				hop.RTTMs = &v
			}
			if r.Success {
				hop.Reached = true
				reached = true
				hop.Status = "reply"
			}
			if probeErr != nil {
				enrichHop(&hop)
				hops = append(hops, hop)
				event(emit, "hop", "error", fmt.Sprintf("Hop %d: %s", ttl, probeErr), hop)
				return result(), probeErr
			}
		}
		enrichHop(&hop)
		hops = append(hops, hop)
		level := "info"
		if reached {
			level = "success"
		}
		if hop.Address == "" {
			level = "warning"
		}
		label := hop.Address
		if hop.Hostname != "" {
			label = hop.Hostname + " [" + hop.Address + "]"
		}
		event(emit, "hop", level, fmt.Sprintf("Hop %d: %s %s", ttl, label, hop.Status), hop)
		if err := ctx.Err(); err != nil {
			return result(), err
		}
		if reached {
			break
		}
	}
	return result(), nil
}

type PortResult struct {
	Time      string  `json:"time,omitempty"`
	Port      int     `json:"port"`
	Service   string  `json:"service"`
	State     string  `json:"state"`
	LatencyMs float64 `json:"latencyMs"`
	Address   string  `json:"address"`
	Error     string  `json:"error,omitempty"`
}

var services = map[int]string{20: "FTP data", 21: "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP", 53: "DNS", 67: "DHCP", 68: "DHCP", 80: "HTTP", 88: "Kerberos", 110: "POP3", 111: "RPC", 123: "NTP", 135: "RPC endpoint", 139: "NetBIOS", 143: "IMAP", 161: "SNMP", 389: "LDAP", 443: "HTTPS", 445: "SMB", 465: "SMTPS", 514: "Syslog", 587: "SMTP submission", 636: "LDAPS", 993: "IMAPS", 995: "POP3S", 1433: "SQL Server", 1521: "Oracle", 1723: "PPTP", 2049: "NFS", 2375: "Docker API", 3128: "HTTP proxy", 3306: "MySQL", 3389: "RDP", 5432: "PostgreSQL", 5900: "VNC", 5985: "WinRM HTTP", 5986: "WinRM HTTPS", 6379: "Redis", 8000: "HTTP alternate", 8080: "HTTP alternate", 8443: "HTTPS alternate", 9100: "Printer", 9200: "Elasticsearch", 27017: "MongoDB"}

func dialPort(ctx context.Context, host string, port int, timeout time.Duration) PortResult {
	r := PortResult{Port: port, Service: services[port], Address: net.JoinHostPort(host, fmt.Sprint(port))}
	start := time.Now()
	conn, err := diagnosticDialContext(ctx, "tcp", r.Address, timeout)
	r.LatencyMs = ms(time.Since(start))
	r.Time = measurementTime()
	if err == nil {
		r.State = "open"
		r.Address = conn.RemoteAddr().String()
		conn.Close()
		return r
	}
	r.Error = err.Error()
	var ne net.Error
	switch {
	case errors.Is(err, context.Canceled):
		r.State = "cancelled"
	case errors.As(err, &ne) && ne.Timeout():
		r.State = "timeout"
	case errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "refused"):
		r.State = "closed"
	case strings.Contains(strings.ToLower(err.Error()), "unreachable") || strings.Contains(strings.ToLower(err.Error()), "no route") || strings.Contains(strings.ToLower(err.Error()), "network is down"):
		r.State = "unreachable"
	default:
		r.State = "error"
	}
	return r
}

func checkPorts(ctx context.Context, host string, ports []int, timeout time.Duration, concurrency int, emit model.Emit) []PortResult {
	results := make([]PortResult, 0, len(ports))
	var mu sync.Mutex
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < concurrency && i < len(ports); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				if ctx.Err() != nil {
					return
				}
				r := dialPort(ctx, host, p, timeout)
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				level := "info"
				if r.State == "open" {
					level = "success"
				} else if r.State == "timeout" || r.State == "unreachable" {
					level = "warning"
				}
				event(emit, "port", level, fmt.Sprintf("TCP %d: %s (%.2f ms)", p, r.State, r.LatencyMs), r)
			}
		}()
	}
send:
	for _, p := range ports {
		select {
		case <-ctx.Done():
			break send
		case jobs <- p:
		}
	}
	close(jobs)
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Port < results[j].Port })
	return results
}

func tcp(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	host, err := validateHost(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	ports, err := parsePorts(stringOption(req.Options, "ports", "22,53,80,443,445,3389"), 1024)
	if err != nil {
		return model.Result{}, err
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 1000, 100, 10000)
	if err != nil {
		return model.Result{}, err
	}
	concurrency, err := intOption(req.Options, "concurrency", 32, 1, 64)
	if err != nil {
		return model.Result{}, err
	}
	addresses, err := resolveHost(ctx, host, 5*time.Second)
	if err != nil {
		return model.Result{}, err
	}
	start := time.Now()
	probeHost := host
	if _, selected := platform.SourceBindingFromContext(ctx); selected {
		probeHost = addresses[0]
	}
	rows := checkPorts(ctx, probeHost, ports, timeout, concurrency, emit)
	open, closed, timedOut := 0, 0, 0
	for _, r := range rows {
		switch r.State {
		case "open":
			open++
		case "closed":
			closed++
		case "timeout":
			timedOut++
		}
	}
	return model.Result{Summary: fmt.Sprintf("%d open, %d refused, %d timed out across %d tested TCP ports", open, closed, timedOut, len(rows)), Data: map[string]any{"target": host, "addresses": addresses, "ports": rows, "openCount": open, "closedCount": closed, "timeoutCount": timedOut, "testedCount": len(rows), "requestedCount": len(ports), "elapsedMs": ms(time.Since(start)), "note": "These are TCP connection tests. Service names are conventional port labels, not detected applications. Closed means the connection was refused; timeout is inconclusive. UDP services are not tested."}}, ctx.Err()
}

type QuickCheck struct {
	Time   string `json:"time,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Data   any    `json:"data,omitempty"`
}

func quick(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	host, err := validateHost(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	ports, err := parsePorts(stringOption(req.Options, "ports", "53,80,443"), 64)
	if err != nil {
		return model.Result{}, err
	}
	checks := make([]QuickCheck, 0, 5)
	add := func(c QuickCheck) {
		if c.Time == "" {
			c.Time = measurementTime()
		}
		checks = append(checks, c)
		event(emit, "check", c.Status, c.Name+": "+c.Detail, c)
	}
	result := func() model.Result {
		pass, warn, fail := 0, 0, 0
		for _, c := range checks {
			switch c.Status {
			case "success":
				pass++
			case "warning":
				warn++
			case "error":
				fail++
			}
		}
		return model.Result{Summary: fmt.Sprintf("%d passed, %d need review, %d failed", pass, warn, fail), Data: map[string]any{"target": host, "checks": checks, "note": "Checks apply to the target and selected services only. ICMP filtering, intentionally closed ports, and local policy can explain negative results. This is not an internet-wide connectivity verdict."}}
	}
	local, systemErr := platform.System(ctx)
	if systemErr != nil {
		add(QuickCheck{Name: "Local adapters", Status: "warning", Detail: systemErr.Error()})
	} else {
		add(QuickCheck{Name: "Local adapters", Status: "info", Detail: "Local interface and resolver information collected", Data: local})
	}
	addresses, resolveErr := resolveHost(ctx, host, 5*time.Second)
	if resolveErr != nil {
		add(QuickCheck{Name: "Name resolution", Status: "error", Detail: resolveErr.Error()})
		return result(), ctx.Err()
	}
	status := "success"
	detail := strings.Join(addresses, ", ")
	if _, e := netip.ParseAddr(host); e == nil {
		status = "info"
		detail = "Literal IP address supplied; DNS lookup not required"
	}
	add(QuickCheck{Name: "Name resolution", Status: status, Detail: detail, Data: addresses})
	if len(addresses) > 0 && ctx.Err() == nil {
		add(makeHostnameCheck(ctx, addresses[0], systemReverseLookup))
	}
	samples := make([]platform.PingReply, 0, 3)
	var pingErr error
	for i := 0; i < 3; i++ {
		if err = ctx.Err(); err != nil {
			return result(), err
		}
		p, e := platform.Ping(ctx, addresses[0], 1500*time.Millisecond, 128)
		stampReply(&p)
		p.Sequence = i + 1
		samples = append(samples, p)
		if e != nil {
			pingErr = e
			break
		}
	}
	s := statistics(samples)
	if pingErr != nil {
		add(QuickCheck{Name: "ICMP reachability", Status: "warning", Detail: pingErr.Error(), Data: samples})
	} else if s.Received == 0 {
		add(QuickCheck{Name: "ICMP reachability", Status: "warning", Detail: "No ICMP reply; filtering or an unreachable target are both possible", Data: samples})
	} else {
		add(QuickCheck{Name: "ICMP reachability", Status: "success", Detail: fmt.Sprintf("%d of %d replies, %.2f ms average", s.Received, s.Sent, *s.AvgMs), Data: samples})
	}
	probeHost := host
	if _, selected := platform.SourceBindingFromContext(ctx); selected {
		probeHost = addresses[0]
	}
	rows := checkPorts(ctx, probeHost, ports, 1200*time.Millisecond, 16, nil)
	for _, p := range rows {
		state := "warning"
		if p.State == "open" {
			state = "success"
		}
		add(QuickCheck{Time: p.Time, Name: fmt.Sprintf("TCP %d %s", p.Port, p.Service), Status: state, Detail: p.State, Data: p})
	}
	return result(), ctx.Err()
}
