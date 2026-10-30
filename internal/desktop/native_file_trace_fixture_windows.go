//go:build windows && toolkit_native_drag_trace122

package desktop

// This sink exists only in explicitly tagged synthetic native fixture binaries.
// It cannot be activated in the ordinary release executable. No web or IPC API
// supplies its destination, and no document/path/token fields are recorded.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var nativeFileFixtureSink func(NativeFileDragObservation)

func nativeFileTraceDiagnosticSink() func(NativeFileDragObservation) { return nativeFileFixtureSink }
func init() {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_SFTP122_TEST") != "1" {
		return
	}
	directory := os.Getenv("NETWORK_TOOLKIT_NATIVE_DRAG_TRACE122")
	if directory == "" {
		return
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return
	}
	temporary, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return
	}
	relative, err := filepath.Rel(temporary, directory)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return
	}
	file, err := os.OpenFile(filepath.Join(directory, fmt.Sprintf("native-drag122-%d.jsonl", os.Getpid())), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	events := make(chan NativeFileDragObservation, 4096)
	var count, dropped, terminal atomic.Uint32
	nativeFileFixtureSink = func(event NativeFileDragObservation) {
		if count.Add(1) > 4096 && (event.Kind != "ole-return" || terminal.Add(1) > 64) {
			dropped.Add(1)
			return
		}
		event.Dropped = dropped.Load()
		select {
		case events <- event:
		default:
			dropped.Add(1)
		}
	}
	go func() {
		encoder := json.NewEncoder(file)
		defer file.Close()
		for event := range events {
			if encoder.Encode(event) != nil {
				return
			}
		}
	}()
}
