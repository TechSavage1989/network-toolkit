//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"networktoolkit/internal/capture"
	"networktoolkit/internal/platform"
)

// Native definitions follow Microsoft's Packet Monitor streaming API. The data
// source is opaque except for its documented leading Kind and UTF-16 Name.
// This avoids guessing the undocumented MAX_NAME/MAX_STRING array constants.
type packetNativeAPI struct {
	module    syscall.Handle
	functions map[string]uintptr
}

var packetNativeFunctions = []string{"PacketMonitorInitialize", "PacketMonitorUninitialize", "PacketMonitorCreateLiveSession", "PacketMonitorCloseSessionHandle", "PacketMonitorSetSessionActive", "PacketMonitorEnumDataSources", "PacketMonitorAddSingleDataSourceToSession", "PacketMonitorCreateRealtimeStream", "PacketMonitorCloseRealtimeStream", "PacketMonitorAttachOutputToSession"}

func openPacketNative() (*packetNativeAPI, error) {
	if localSystemDirectoryError != nil {
		return nil, localSystemDirectoryError
	}
	path, e := syscall.UTF16PtrFromString(filepath.Join(localSystemDirectory, "pktmonapi.dll"))
	if e != nil {
		return nil, e
	}
	loaded, _, loadError := localKernel.NewProc("LoadLibraryExW").Call(uintptr(unsafe.Pointer(path)), 0, 0x800)
	runtime.KeepAlive(path)
	if loaded == 0 {
		return nil, loadError
	}
	h := syscall.Handle(loaded)
	a := &packetNativeAPI{module: h, functions: map[string]uintptr{}}
	for _, name := range packetNativeFunctions {
		p, e := syscall.GetProcAddress(h, name)
		if e != nil {
			syscall.FreeLibrary(h)
			return nil, e
		}
		a.functions[name] = p
	}
	return a, nil
}
func (a *packetNativeAPI) close() { _ = syscall.FreeLibrary(a.module) }
func (a *packetNativeAPI) call(name string, args ...uintptr) error {
	result, _, _ := syscall.SyscallN(a.functions[name], args...)
	if int32(result) < 0 {
		return fmt.Errorf("%s failed (0x%08X)", name, uint32(result))
	}
	return nil
}
func (a *packetNativeAPI) release(name string, handle uintptr) {
	syscall.SyscallN(a.functions[name], handle)
}

type packetNativeSource struct {
	view    capture.Interface
	pointer uintptr
}

func packetSourceName(buffer []byte, address uintptr) (string, error) {
	if len(buffer) < 6 {
		return "", errors.New("The native adapter buffer is incomplete.")
	}
	base := uintptr(unsafe.Pointer(&buffer[0]))
	if address < base || address-base+6 > uintptr(len(buffer)) {
		return "", errors.New("The native adapter list contains an invalid pointer.")
	}
	off := int(address-base) + 4
	units := []uint16{}
	for n := 0; n < 256 && off+2 <= len(buffer); n++ {
		u := binary.LittleEndian.Uint16(buffer[off:])
		if u == 0 {
			return syscall.UTF16ToString(units), nil
		}
		units = append(units, u)
		off += 2
	}
	return "", errors.New("The native adapter name exceeds its bounds.")
}
func (a *packetNativeAPI) sources(handle uintptr, adapters []packetSystemAdapter) ([]packetNativeSource, []byte, error) {
	var needed uintptr
	_ = a.call("PacketMonitorEnumDataSources", handle, 1, 0, 0, uintptr(unsafe.Pointer(&needed)), 0)
	if needed < 8 || needed > 8<<20 {
		return nil, nil, errors.New("Windows did not return a valid packet adapter list.")
	}
	for tries := 0; tries < 3; tries++ {
		buffer := make([]byte, needed)
		e := a.call("PacketMonitorEnumDataSources", handle, 1, 0, uintptr(len(buffer)), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&buffer[0])))
		if e != nil {
			if needed > uintptr(len(buffer)) && needed <= 8<<20 {
				continue
			}
			return nil, nil, e
		}
		count := int(binary.LittleEndian.Uint32(buffer))
		if count < 1 || count > 1024 || 8+count*8 > len(buffer) {
			return nil, nil, errors.New("No valid Packet Monitor network adapters were returned.")
		}
		sources := []packetNativeSource{}
		for i := 0; i < count; i++ {
			address := uintptr(binary.LittleEndian.Uint64(buffer[8+i*8:]))
			name, e := packetSourceName(buffer, address)
			if e != nil {
				return nil, nil, e
			}
			adapter, matched := packetMatchAdapter(name, adapters)
			if !matched {
				continue
			}
			sum := sha256.Sum256([]byte(name + "\x00" + adapter.ID))
			view := packetAdapterView(capture.Interface{ID: fmt.Sprintf("live:%x", sum[:12]), Engine: "live"}, adapter)
			sources = append(sources, packetNativeSource{view: view, pointer: address})
		}
		return sources, buffer, nil
	}
	return nil, nil, errors.New("The packet adapter list kept changing. Refresh and try again.")
}
func packetCaptureSupport() packetCaptureCapabilities {
	support := packetCaptureCapabilities{Elevated: localElevated(), Interfaces: []capture.Interface{{ID: "nics", Name: "All network adapters", Engine: "auto"}}}
	if localSystemDirectoryError != nil {
		support.Note = localSystemDirectoryError.Error()
		return support
	}
	if a, e := openPacketNative(); e == nil {
		support.Live = true
		a.close()
	}
	if info, e := os.Stat(filepath.Join(localSystemDirectory, "pktmon.exe")); e == nil && !info.IsDir() {
		support.File = true
	}
	support.Available = support.Live || support.File
	support.Note = "Uses Windows Packet Monitor. Administrator approval is requested when starting capture or loading adapters. Live capture supports up to 9000 bytes per packet. File mode shows packet details after Stop."
	if !support.Available {
		support.Note = "Windows Packet Monitor is unavailable. Import PCAP or PCAPNG files to inspect existing captures."
	}
	return support
}
func packetCaptureCommand(ctx context.Context, args ...string) (string, error) {
	timeout := 12 * time.Second
	if len(args) > 0 && args[0] == "etl2pcap" {
		timeout = 45 * time.Second
	}
	child, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(child, filepath.Join(localSystemDirectory, "pktmon.exe"), args...)
	platform.ConfigureCommand(cmd)
	var out localOutputBuffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	e := cmd.Run()
	text := string(out.data)
	if decoded, _, err := localDecodeLegacy(out.data); err == nil {
		text = decoded
	}
	text = strings.TrimSpace(text)
	if e != nil {
		return text, fmt.Errorf("Packet Monitor %s: %s (%v)", args[0], text, e)
	}
	return text, nil
}

var packetComponentRow = regexp.MustCompile(`(?m)^\s*(\d+)\s+(.+)$`)

func packetFileInterfaces(ctx context.Context, adapters []packetSystemAdapter) ([]capture.Interface, error) {
	text, e := packetCaptureCommand(ctx, "list")
	if e != nil {
		return nil, e
	}
	out := []capture.Interface{}
	for _, match := range packetComponentRow.FindAllStringSubmatch(text, -1) {
		id, e := strconv.ParseUint(match[1], 10, 32)
		if e == nil && id > 0 {
			if adapter, matched := packetMatchAdapter(match[2], adapters); matched {
				out = append(out, packetAdapterView(capture.Interface{ID: "file:" + match[1], Engine: "file"}, adapter))
			}
		}
	}
	return out, nil
}
func packetCapturePerformList(ctx context.Context, emit func(packetCaptureMessage) error) error {
	adapters, e := packetReadSystemAdapters(ctx)
	if e != nil {
		return fmt.Errorf("Read configurable capture adapters: %w", e)
	}
	views := []capture.Interface{}
	var liveError error
	if api, e := openPacketNative(); e == nil {
		defer api.close()
		var handle uintptr
		if e = api.call("PacketMonitorInitialize", 0x10000, 0, uintptr(unsafe.Pointer(&handle))); e == nil {
			defer api.release("PacketMonitorUninitialize", handle)
			sources, buffer, sourceError := api.sources(handle, adapters)
			liveError = sourceError
			for _, s := range sources {
				views = append(views, s.view)
			}
			runtime.KeepAlive(buffer)
		} else {
			liveError = e
		}
	}
	if found, e := packetFileInterfaces(ctx, adapters); e == nil {
		views = append(views, found...)
	} else if len(views) == 0 {
		if liveError != nil {
			return liveError
		}
		return e
	}
	if len(views) > 0 {
		views = append([]capture.Interface{{ID: "nics", Name: "All configurable network adapters", Engine: "auto"}}, views...)
	}
	return emit(packetCaptureMessage{Version: 1, Type: "interfaces", Interfaces: views})
}

type packetStreamConfiguration struct {
	Context          uintptr
	Event            uintptr
	Data             uintptr
	BufferMultiplier uint16
	Truncation       uint16
}
type packetStreamDescriptor struct {
	Data           uintptr
	Size           uint32
	MetadataOffset uint32
	PacketOffset   uint32
	PacketLength   uint32
	MissedWrites   uint32
	MissedReads    uint32
}
type packetStreamMetadata struct {
	Group        uint64
	Count        uint16
	Appearance   uint16
	Direction    uint16
	PacketType   uint16
	Component    uint16
	Edge         uint16
	Reserved     uint16
	DropReason   uint32
	DropLocation uint32
	Processor    uint16
	Timestamp    int64
}

func nativePacketLength(data []byte, link uint16) uint32 {
	offset := 0
	ether := uint16(0)
	if link == 1 && len(data) >= 14 {
		offset = 14
		ether = binary.BigEndian.Uint16(data[12:])
		for (ether == 0x8100 || ether == 0x88a8) && len(data) >= offset+4 {
			ether = binary.BigEndian.Uint16(data[offset+2:])
			offset += 4
		}
	} else if link == 101 && len(data) > 0 {
		if data[0]>>4 == 4 {
			ether = 0x800
		} else if data[0]>>4 == 6 {
			ether = 0x86dd
		}
	}
	length := len(data)
	if ether == 0x800 && len(data) >= offset+20 {
		length = max(length, offset+int(binary.BigEndian.Uint16(data[offset+2:])))
	} else if ether == 0x86dd && len(data) >= offset+40 {
		length = max(length, offset+40+int(binary.BigEndian.Uint16(data[offset+4:])))
	}
	return uint32(length)
}
func packetNativeRecord(descriptor *packetStreamDescriptor) (capture.Record, uint64, error) {
	if descriptor == nil || descriptor.Data == 0 || descriptor.Size > 1<<20 || descriptor.PacketLength == 0 || descriptor.PacketLength > 9000 || uint64(descriptor.PacketOffset)+uint64(descriptor.PacketLength) > uint64(descriptor.Size) || uint64(descriptor.MetadataOffset)+uint64(unsafe.Sizeof(packetStreamMetadata{})) > uint64(descriptor.Size) {
		return capture.Record{}, 0, errors.New("Invalid native packet descriptor")
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(descriptor.Data)), int(descriptor.Size))
	meta := (*packetStreamMetadata)(unsafe.Pointer(&raw[descriptor.MetadataOffset]))
	link := uint16(147)
	switch meta.PacketType {
	case 1:
		link = 1
	case 2:
		link = 105
	case 3:
		link = 101
	}
	stamp := time.Now().UTC()
	const epoch = int64(116444736000000000)
	if meta.Timestamp >= epoch && meta.Timestamp < epoch+72581184000000000 {
		stamp = time.Unix((meta.Timestamp-epoch)/10000000, ((meta.Timestamp-epoch)%10000000)*100).UTC()
	}
	direction := ""
	switch meta.Direction {
	case 1, 3, 5:
		direction = "in"
	case 2, 4, 6:
		direction = "out"
	}
	data := append([]byte{}, raw[descriptor.PacketOffset:descriptor.PacketOffset+descriptor.PacketLength]...)
	return capture.Record{Time: stamp, Data: data, OriginalLength: nativePacketLength(data, link), LinkType: link, Interface: fmt.Sprintf("Component %d", meta.Component), Direction: direction, DropReason: meta.DropReason}, uint64(descriptor.MissedWrites) + uint64(descriptor.MissedReads), nil
}

type packetNativeSink struct {
	queue  chan capture.Record
	events chan string
	lost   atomic.Uint64
}

var packetNativeSinks sync.Map
var packetNativeNext atomic.Uint64

// Windows retains callback thunks for the process lifetime. Reuse two fixed
// callbacks so repeatedly starting capture cannot exhaust the thunk table or
// retain a per-capture closure. The opaque context is an integer registry ID.
var packetNativeDataCallback = syscall.NewCallback(func(context, ptr uintptr) uintptr {
	value, ok := packetNativeSinks.Load(context)
	if !ok {
		return 0
	}
	sink := value.(*packetNativeSink)
	record, missed, e := packetNativeRecord((*packetStreamDescriptor)(unsafe.Pointer(ptr)))
	sink.lost.Add(missed)
	if e != nil {
		sink.lost.Add(1)
		return 0
	}
	select {
	case sink.queue <- record:
	default:
		sink.lost.Add(1)
	}
	return 0
})
var packetNativeEventCallback = syscall.NewCallback(func(context, _ uintptr, kind uintptr) uintptr {
	value, ok := packetNativeSinks.Load(context)
	if !ok {
		return 0
	}
	select {
	case value.(*packetNativeSink).events <- fmt.Sprintf("Native stream event %d", kind):
	default:
	}
	return 0
})

func packetCaptureLive(ctx context.Context, o capture.Options, emit func(packetCaptureMessage) error) error {
	api, e := openPacketNative()
	if e != nil {
		return e
	}
	defer api.close()
	var handle, session, stream uintptr
	if e = api.call("PacketMonitorInitialize", 0x10000, 0, uintptr(unsafe.Pointer(&handle))); e != nil {
		return e
	}
	defer api.release("PacketMonitorUninitialize", handle)
	name, _ := syscall.UTF16PtrFromString("NetworkToolkit." + randomHex(12))
	if e = api.call("PacketMonitorCreateLiveSession", handle, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&session))); e != nil {
		return e
	}
	defer api.release("PacketMonitorCloseSessionHandle", session)
	adapters, e := packetReadSystemAdapters(ctx)
	if e != nil {
		return e
	}
	sources, buffer, e := api.sources(handle, adapters)
	if e != nil {
		return e
	}
	found := false
	for _, source := range sources {
		if o.Interface == "nics" || source.view.ID == o.Interface {
			if e = api.call("PacketMonitorAddSingleDataSourceToSession", session, source.pointer); e != nil {
				return e
			}
			found = true
		}
	}
	runtime.KeepAlive(buffer)
	if !found {
		return errors.New("No matching configurable adapter is available for live capture. Refresh adapters or choose compatibility file capture.")
	}
	sink := &packetNativeSink{queue: make(chan capture.Record, 512), events: make(chan string, 8)}
	queue, events := sink.queue, sink.events
	contextID := uintptr(packetNativeNext.Add(1))
	packetNativeSinks.Store(contextID, sink)
	defer packetNativeSinks.Delete(contextID)
	snap := o.SnapLen
	if snap == 0 || snap > 9000 {
		snap = 9000
	}
	config := packetStreamConfiguration{Context: contextID, Event: packetNativeEventCallback, Data: packetNativeDataCallback, BufferMultiplier: 10, Truncation: uint16(snap)}
	if e = api.call("PacketMonitorCreateRealtimeStream", handle, uintptr(unsafe.Pointer(&config)), uintptr(unsafe.Pointer(&stream))); e != nil {
		return e
	}
	defer func() {
		if stream != 0 {
			api.release("PacketMonitorCloseRealtimeStream", stream)
		}
	}()
	if e = api.call("PacketMonitorAttachOutputToSession", session, stream); e != nil {
		return e
	}
	if e = api.call("PacketMonitorSetSessionActive", session, 1); e != nil {
		return e
	}
	active := true
	defer func() {
		if active {
			_ = api.call("PacketMonitorSetSessionActive", session, 0)
		}
	}()
	if e = emit(packetCaptureMessage{Version: 1, Type: "ready", Engine: "live", Note: "Live Windows Packet Monitor capture"}); e != nil {
		return e
	}
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]capture.Record, 0, 128)
	send := func() error {
		if len(batch) == 0 && sink.lost.Load() == 0 {
			return nil
		}
		e := emit(packetCaptureMessage{Version: 1, Type: "packets", Packets: batch, Lost: sink.lost.Swap(0)})
		batch = nil
		return e
	}
	for {
		select {
		case <-ctx.Done():
			if e = api.call("PacketMonitorSetSessionActive", session, 0); e != nil {
				return e
			}
			active = false
			api.release("PacketMonitorCloseRealtimeStream", stream)
			stream = 0
			for {
				select {
				case record := <-queue:
					batch = append(batch, record)
					if len(batch) >= 128 {
						if e = send(); e != nil {
							return e
						}
					}
				default:
					return send()
				}
			}
		case record := <-queue:
			batch = append(batch, record)
			if len(batch) >= 128 {
				if e = send(); e != nil {
					return e
				}
			}
		case <-ticker.C:
			if e = send(); e != nil {
				return e
			}
		case message := <-events:
			if e = emit(packetCaptureMessage{Version: 1, Type: "status", Note: message}); e != nil {
				return e
			}
		}
	}
}
func packetCaptureFile(ctx context.Context, o capture.Options, emit func(packetCaptureMessage) error) error {
	if o.Filter.Direction != "" && o.Filter.Direction != "any" {
		return errors.New("This Windows version requires file capture. Choose both traffic directions.")
	}
	filters, e := packetCaptureCommand(ctx, "filter", "list")
	if e != nil {
		return e
	}
	if len(packetComponentRow.FindAllStringSubmatch(filters, -1)) > 0 {
		return errors.New("Packet Monitor has filters configured by another tool. Clear those filters in that tool before starting this capture.")
	}
	dir, e := os.MkdirTemp("", "NetworkToolkit-Capture-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	etl, pcap := filepath.Join(dir, "capture.etl"), filepath.Join(dir, "capture.pcapng")
	adapters, e := packetReadSystemAdapters(ctx)
	if e != nil {
		return e
	}
	interfaces, e := packetFileInterfaces(ctx, adapters)
	if e != nil {
		return e
	}
	components := []string{}
	for _, adapter := range interfaces {
		if o.Interface == "nics" || adapter.ID == o.Interface {
			components = append(components, strings.TrimPrefix(adapter.ID, "file:"))
		}
	}
	if len(components) == 0 {
		return errors.New("That configurable capture adapter is no longer available. Refresh adapters and try again.")
	}
	component := strings.Join(components, ",")
	args := []string{"start", "--capture", "--comp", component, "--pkt-size", strconv.Itoa(o.SnapLen), "--file-size", strconv.Itoa(o.MaxMB), "--log-mode", "circular", "--file-name", etl}
	if _, e = packetCaptureCommand(ctx, args...); e != nil {
		return e
	}
	owned := true
	stop := func() error {
		if !owned {
			return nil
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		status, e := packetCaptureCommand(cleanup, "status")
		if e != nil {
			return e
		}
		if !strings.Contains(strings.ToLower(status), strings.ToLower(etl)) {
			owned = false
			return errors.New("The Packet Monitor capture was changed outside this app. The other capture was left running.")
		}
		_, e = packetCaptureCommand(cleanup, "stop")
		if e == nil {
			owned = false
		}
		return e
	}
	defer func() {
		if owned {
			_ = stop()
		}
	}()
	if e = emit(packetCaptureMessage{Version: 1, Type: "ready", Engine: "file", Note: "Recording a bounded circular ETL file. Stop to inspect packets. Keep filters are applied when the file is read."}); e != nil {
		return e
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for waiting := true; waiting; {
		select {
		case <-ctx.Done():
			waiting = false
		case <-ticker.C:
			if info, e := os.Stat(etl); e == nil {
				if e = emit(packetCaptureMessage{Version: 1, Type: "status", Note: fmt.Sprintf("Recording file: %.1f MiB. Packet details appear after Stop.", float64(info.Size())/(1<<20))}); e != nil {
					return e
				}
			}
		}
	}
	if e = stop(); e != nil {
		return e
	}
	convert, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if _, e = packetCaptureCommand(convert, "etl2pcap", etl, "--out", pcap); e != nil {
		return e
	}
	file, e := os.Open(pcap)
	if e != nil {
		return e
	}
	defer file.Close()
	batch := []capture.Record{}
	batchBytes := 0
	send := func() error {
		if len(batch) == 0 {
			return nil
		}
		e := emit(packetCaptureMessage{Version: 1, Type: "packets", Packets: batch})
		batch = nil
		batchBytes = 0
		return e
	}
	if e = capture.Read(file, func(r capture.Record) error {
		if batchBytes+len(r.Data) > 2<<20 {
			if e := send(); e != nil {
				return e
			}
		}
		batch = append(batch, r)
		batchBytes += len(r.Data)
		if len(batch) >= 64 {
			return send()
		}
		return nil
	}); e != nil {
		return e
	}
	return send()
}
func packetCapturePerform(ctx context.Context, o capture.Options, emit func(packetCaptureMessage) error) error {
	captureCtx, cancel := context.WithTimeout(ctx, time.Duration(o.Duration)*time.Second)
	defer cancel()
	support := packetCaptureSupport()
	if o.Engine == "auto" && strings.HasPrefix(o.Interface, "file:") {
		o.Engine = "file"
	}
	if o.Engine == "auto" && strings.HasPrefix(o.Interface, "live:") {
		o.Engine = "live"
	}
	if o.Engine == "live" || o.Engine == "auto" && support.Live {
		return packetCaptureLive(captureCtx, o, emit)
	}
	return packetCaptureFile(captureCtx, o, emit)
}
