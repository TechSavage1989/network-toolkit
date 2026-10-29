//go:build windows

package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// This exercises the real native close timer against an outstanding renderer
// confirmation, not only the pure handshake model. All content is synthetic.
func TestNativeEditorCloseApprovalInstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_EDITOR_TEST") != "1" {
		t.Skip("explicit hosted Windows editor acceptance only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	started := time.Now()
	var traceMu sync.Mutex
	var trace []string
	traceClosed := false
	defer func() {
		traceMu.Lock()
		traceClosed = true
		traceMu.Unlock()
	}()
	outputDir := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT")
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	checkpoint := func(stage string) {
		traceMu.Lock()
		defer traceMu.Unlock()
		if traceClosed {
			return
		}
		line := fmt.Sprintf("%s elapsed=%s %s", time.Now().UTC().Format(time.RFC3339Nano), time.Since(started).Round(time.Millisecond), stage)
		trace = append(trace, line)
		t.Log(line)
		if outputDir != "" {
			if err := os.WriteFile(filepath.Join(outputDir, "native-editor-close115-stages.txt"), []byte(strings.Join(trace, "\n")+"\n"), 0600); err != nil {
				t.Errorf("write synthetic close fixture diagnostics: %v", err)
			}
		}
	}
	checkpoint("fixture start; total deadline=35s")
	token := strings.Repeat("ae", 32)
	var closing, continued atomic.Int32
	var ready atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(204)
			return
		}
		if r.URL.Path == "/continued" {
			if r.Header.Get("X-Session-Token") != token {
				http.Error(w, "unauthorized", 401)
				return
			}
			continued.Add(1)
			w.WriteHeader(204)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><meta charset="utf-8"><title>Editor close approval fixture</title><style>body{font:16px Segoe UI;padding:30px;background:#eff4ff;color:#18283f}button{padding:12px;margin:10px}textarea{width:90%;height:120px}</style><h1>Unsaved synthetic document</h1><textarea id="text">SYNTHETIC DIRTY EDITOR CONTENT</textarea><p id="status">Ready</p><section id="confirm" hidden><h2>Keep unsaved document?</h2><button id="cancel">Cancel close</button></section><button id="edit">Continue editing</button><script>
window.__networkToolkitStartupState=1;let pending=false,cancelled=false;
window.__networkToolkitClose=()=>{pending=true;document.getElementById('confirm').hidden=false;document.getElementById('status').textContent='Waiting for explicit close approval';return new Promise(resolve=>{document.getElementById('cancel').onclick=()=>{pending=false;cancelled=true;document.getElementById('confirm').hidden=true;document.getElementById('status').textContent='Close cancelled; original document retained';resolve(false);};});};
document.getElementById('edit').onclick=async()=>{if(pending||!cancelled)throw Error('Close was not cancelled');document.getElementById('text').value+='\nCONTINUED AFTER CANCEL';await fetch('/continued',{method:'POST',headers:{'X-Session-Token':window.__networkToolkitNativeSession}});document.getElementById('status').textContent='Native close cancelled; editing remains available';};
</script></html>`)
	}))
	defer server.Close()
	result := make(chan error, 1)
	profile, err := os.MkdirTemp("", "network-toolkit-editor-close-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for until := time.Now().Add(10 * time.Second); ; {
			if err := os.RemoveAll(profile); err == nil {
				return
			}
			if time.Now().After(until) {
				t.Error("synthetic editor profile remained locked")
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	err = Run(ctx, Config{Title: "Network Toolkit editor close acceptance", URL: server.URL + "/", SessionToken: token, DataDir: profile, RequireCloseApproval: true, Stop: cancel, OnClosing: func() { closing.Add(1) }, OnReady: func() {
		ready.Store(true)
		checkpoint("native renderer ready")
		var host *hostWindow
		wndWindows.Range(func(_, value any) bool {
			if h, ok := value.(*hostWindow); ok && h.cfg.DataDir == profile && h.ready {
				host = h
			}
			return true
		})
		if host == nil {
			result <- fmt.Errorf("exact fixture host not found")
			cancel()
			return
		}
		go func() {
			checkpoint("post exact native WM_CLOSE; begin 6.2s consent hold")
			wndPostMessage.Call(host.hwnd, wmClose, 0, 0)
			// Wait beyond the production five-second fallback. No automatic close or
			// irreversible primary-workspace notification may occur.
			time.Sleep(6200 * time.Millisecond)
			if closing.Load() != 0 {
				result <- fmt.Errorf("OnClosing ran without consent")
				cancel()
				return
			}
			visible, _, _ := wndIsWindowVisible.Call(host.hwnd)
			if visible == 0 {
				result <- fmt.Errorf("dirty native host closed on timeout")
				cancel()
				return
			}
			popup, _, _ := wndUser32.NewProc("GetLastActivePopup").Call(host.hwnd)
			owner, _, _ := wndUser32.NewProc("GetWindow").Call(popup, 4)
			var class [64]uint16
			wndUser32.NewProc("GetClassNameW").Call(popup, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
			if popup == 0 || owner != host.hwnd || syscall.UTF16ToString(class[:]) != "#32770" {
				result <- fmt.Errorf("bounded native close warning was not shown")
				cancel()
				return
			}
			checkpoint("6.2s consent hold passed; dirty host visible, no OnClosing, owned warning present")
			script, scriptErr := filepath.Abs(filepath.Join("..", "..", "scripts", "native-editor-close.ps1"))
			if scriptErr != nil {
				result <- scriptErr
				cancel()
				return
			}
			// Use the existing exact-owned, caption-validated OK control and verify
			// dismissal before UIA. An unchecked assumed control ID does not prove
			// the modal warning closed and can leave UIA blocked until our deadline.
			for _, action := range []string{"WaitWarning", "CancelEdit"} {
				checkpoint("PowerShell " + action + " begin")
				output, runErr := runNativeEditorCloseScript(ctx, script, host.hwnd, action)
				checkpoint(fmt.Sprintf("PowerShell %s end; error=%v context=%v output=%s", action, runErr, ctx.Err(), strings.TrimSpace(string(output))))
				if runErr != nil {
					result <- fmt.Errorf("native %s interaction (context=%v): %w: %s", action, ctx.Err(), runErr, output)
					cancel()
					return
				}
			}
			if continued.Load() != 1 || closing.Load() != 0 {
				result <- fmt.Errorf("cancelled host lost editability or notified closing")
				cancel()
				return
			}
			checkpoint("real Cancel/edit completed; continued=1 and OnClosing=0")
			if dir := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT"); dir != "" {
				_ = os.MkdirAll(dir, 0700)
				if _, _, e := captureNativeVaultDialog(host.hwnd, filepath.Join(dir, "native-editor-close108.png")); e != nil {
					result <- e
					cancel()
					return
				}
			}
			result <- nil
			checkpoint("all native consent/editability assertions passed; begin fixture cleanup")
			cancel()
		}()
	}})
	checkpoint(fmt.Sprintf("native Run returned; error=%v context=%v ready=%t continued=%d closing=%d", err, ctx.Err(), ready.Load(), continued.Load(), closing.Load()))
	if err != nil {
		t.Fatal(err)
	}
	if !ready.Load() {
		t.Fatal("native editor fixture did not initialize")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("native editor close proof did not finish")
	}
	if closing.Load() != 1 {
		t.Fatalf("approved final shutdown notified closing %d times", closing.Load())
	}
}
