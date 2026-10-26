package diag

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"networktoolkit/internal/deviceinfo"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

const deviceProbeTimeout = 2500 * time.Millisecond

// DeviceIdentity holds only strings actually reported by a device description.
// Neither a MAC assignment nor a port number provides a model or manufacturer.
type DeviceIdentity struct {
	Manufacturer      string `json:"manufacturer,omitempty"`
	Model             string `json:"model,omitempty"`
	ModelName         string `json:"modelName,omitempty"`
	ModelNumber       string `json:"modelNumber,omitempty"`
	FriendlyName      string `json:"friendlyName,omitempty"`
	ModelSource       string `json:"modelSource,omitempty"`
	DescriptionURL    string `json:"descriptionUrl,omitempty"`
	IdentityResponded bool   `json:"identityResponded,omitempty"`
	IdentityError     string `json:"identityError,omitempty"`
}

type deviceProbe func(context.Context, string, time.Duration) (deviceinfo.Info, error)

func deviceIdentity(info deviceinfo.Info, err error) DeviceIdentity {
	identity := DeviceIdentity{
		Manufacturer: info.Manufacturer, Model: info.Model, ModelName: info.ModelName,
		ModelNumber: info.ModelNumber, FriendlyName: info.FriendlyName, ModelSource: info.ModelSource,
		DescriptionURL: info.DescriptionURL, IdentityResponded: info.Responded,
	}
	if err != nil {
		identity.IdentityError = err.Error()
	}
	return identity
}

type IdentifyResult struct {
	Time   string          `json:"time,omitempty"`
	Source *SourceMetadata `json:"source,omitempty"`
	HostIdentity
	DeviceIdentity
	HostnameIdentity
	Address      string   `json:"address"`
	Status       string   `json:"status"`
	Responded    bool     `json:"responded"`
	Evidence     []string `json:"evidence"`
	ElapsedMs    float64  `json:"elapsedMs"`
	NeighborNote string   `json:"neighborNote,omitempty"`
	Note         string   `json:"note"`
}

func identify(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	return runIdentify(ctx, req, emit, platform.Neighbors, deviceinfo.Probe, systemReverseLookup)
}

func runIdentify(ctx context.Context, req model.Request, emit model.Emit, neighbors neighborReader, probe deviceProbe, lookup reverseLookup) (model.Result, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(req.Target))
	if err != nil || !address.Is4() || address.IsUnspecified() || address.IsMulticast() || address.String() == "255.255.255.255" {
		return model.Result{}, errors.New("enter one IPv4 device address without a hostname, subnet, URL, or port")
	}
	timeout, err := durationOption(req.Options, "timeoutMs", int(deviceProbeTimeout/time.Millisecond), 1200, 5000)
	if err != nil {
		return model.Result{}, err
	}
	start := time.Now()
	data := IdentifyResult{
		Source:  sourceMetadata(ctx),
		Address: address.String(), Status: "no_response", Evidence: []string{},
		HostnameIdentity: disabledHostname(),
		Note:             "MAC and vendor use only the exact-IP local neighbor cache, may be stale, and may be unavailable for routed devices. Vendor identifies the IEEE MAC assignment, not a model. Optional manufacturer and model fields come from one device-reported UPnP description. This is a bounded unicast request to the selected IPv4 device; devices without UPnP support may not report a model.",
	}
	resolveName := boolOption(req.Options, "resolveHostnames", true)
	if resolveName {
		data.HostnameIdentity = HostnameIdentity{HostnameStatus: "cancelled", HostnameNote: "Hostname lookup did not run before cancellation."}
		data.Note += " Hostname uses a separately bounded reverse lookup through configured DNS servers and the hosts file and is independent of UPnP support or response evidence."
	}
	result := func() model.Result {
		data.ElapsedMs = ms(time.Since(start))
		if ctx.Err() != nil {
			data.Status = "cancelled"
		}
		summary := "No UPnP response received; available neighbor identity is retained"
		if data.Responded {
			summary = "Device replied to the UPnP request"
		}
		if data.Model != "" {
			summary = fmt.Sprintf("Device reported model %s", data.Model)
		}
		if data.Status == "error" {
			summary = "Device details could not be retrieved; available neighbor identity is retained"
		}
		if data.Status == "cancelled" {
			summary = "Device details cancelled; partial identity information is retained"
		}
		return model.Result{Summary: summary, Data: data}
	}
	if err := ctx.Err(); err != nil {
		return result(), err
	}
	known, neighborErr := readNeighborIdentities(ctx, neighbors)
	data.Time = measurementTime()
	data.HostIdentity = known[data.Address]
	if neighborErr != nil {
		data.NeighborNote = "Initial neighbor cache: " + neighborErr.Error()
	}
	if err := ctx.Err(); err != nil {
		return result(), err
	}
	model.Log(emit, "info", "Requesting device-reported details from "+data.Address, nil)
	info, probeErr := probe(ctx, data.Address, timeout)
	data.Time = measurementTime()
	data.DeviceIdentity = deviceIdentity(info, probeErr)
	data.Responded = info.Responded
	if data.Responded {
		data.Evidence = append(data.Evidence, "UPnP discovery reply")
		data.Status = "responded"
	} else if probeErr != nil && !errors.Is(probeErr, deviceinfo.ErrNoResponse) {
		data.Status = "error"
	}
	if resolveName && ctx.Err() == nil {
		data.HostnameIdentity = resolveHostname(ctx, data.Address, lookup)
		data.Time = measurementTime()
	}
	refreshed, refreshErr := refreshNeighborIdentities(ctx, neighbors)
	data.Time = measurementTime()
	if identity, ok := refreshed[data.Address]; ok {
		data.HostIdentity = identity
	}
	if refreshErr != nil {
		data.NeighborNote = joinIdentityNote(data.NeighborNote, "Final neighbor cache: "+refreshErr.Error())
	}
	out := result()
	level := "warning"
	if data.Responded {
		level = "success"
	} else if data.Status == "error" {
		level = "error"
	}
	event(emit, "identity", level, out.Summary, data)
	return out, ctx.Err()
}
