//go:build windows

package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	networktoolkit "networktoolkit"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This exercises Explorer's real OLE drag into the installed native WebView2,
// not a constructed JavaScript DataTransfer. The production File-only helper is
// served by a synthetic authenticated fixture; app endpoint confinement has its
// own HTTP tests. Required CI never converts unavailable desktop support to pass.
func TestNativeExplorerDropInstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_EXPLORER_TEST") != "1" {
		t.Skip("explicit hosted Windows Explorer acceptance only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	temp := t.TempDir()
	source := filepath.Join(temp, "source")
	destination := filepath.Join(temp, "destination")
	for _, dir := range []string{source, destination, filepath.Join(source, "unsupported-folder")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	content := "SYNTHETIC NETWORK TOOLKIT107 EXPLORER FILE\r\n"
	name := "synthetic-drop107.txt"
	if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a7", 32)
	var uploads, rejections atomic.Int32
	var complete atomic.Bool
	asset, err := networktoolkit.Web.ReadFile("web/assets/transfer-drop.js")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(204)
			return
		}
		if r.URL.Path == "/assets/transfer-drop.js" {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(asset)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><title>Explorer107 native fixture</title><style>body{font:14px Segoe UI;background:#edf4ff;padding:12px}#target{border:3px dashed #185abb;height:230px;padding:14px}button{font:inherit;padding:12px}input,select{max-width:180px}progress{width:180px}.transfer-drop-row{padding:4px}#modal{background:white;padding:8px}</style></head><body><h1>Native Explorer drop fixture</h1><div id="target">Drop synthetic files here. Local destination only.</div><div id="modal"></div><p id="status">Ready for actual Explorer drop</p><script>const SESSION=window.__networkToolkitNativeSession;window.__networkToolkitStartupState=/^[a-f0-9]{64}$/.test(SESSION||'')?1:-1;window.toast=(message)=>{document.getElementById('status').textContent=message};window.openModal=(title,body,buttons)=>{document.getElementById('target').hidden=true;document.getElementById('modal').innerHTML='<h2>'+title+'</h2>'+body+buttons};window.closeModal=()=>document.getElementById('modal').replaceChildren();</script><script src="/assets/transfer-drop.js"></script><script>
const helper=NetworkToolkitTransferDrop;document.addEventListener('dragover',event=>{event.preventDefault();event.dataTransfer.dropEffect='copy'});document.addEventListener('drop',event=>{event.preventDefault();try{const files=helper.fromDrop(event.dataTransfer);helper.show({files,entries:[],label:'Synthetic isolated local destination',remote:false,check(){},send:async(row,onProgress,signal)=>{const form=new FormData();form.set('file',row.file,row.name);return helper.upload('/upload',form,{signal,onProgress})},done:async report=>{if(report.copied!==1||report.failed||report.cancelled)throw Error('Bad copy report');await fetch('/complete',{method:'POST',headers:{'X-Session-Token':SESSION},body:JSON.stringify(report)});document.getElementById('status').textContent='Native Explorer copy verified'}});}catch(error){fetch('/rejected',{method:'POST',headers:{'X-Session-Token':SESSION}}).then(()=>{document.getElementById('status').textContent='Folder drop rejected - no files imported'});}});
</script></body></html>`)
			return
		}
		if r.Header.Get("X-Session-Token") != token {
			http.Error(w, "unauthorized", 401)
			return
		}
		switch r.URL.Path {
		case "/rejected":
			if uploads.Load() != 0 {
				http.Error(w, "partial folder import", 400)
				return
			}
			rejections.Add(1)
			w.WriteHeader(204)
		case "/upload":
			if rejections.Load() != 1 {
				http.Error(w, "folder rejection missing", 400)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
			if err := r.ParseMultipartForm(1024); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, h, e := r.FormFile("file")
			if e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			defer f.Close()
			b, e := io.ReadAll(f)
			if e != nil || h.Filename != name || string(b) != content {
				http.Error(w, "wrong OS File bytes", 400)
				return
			}
			if e = os.WriteFile(filepath.Join(destination, name), b, 0600); e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			uploads.Add(1)
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true}`)
		case "/complete":
			complete.Store(uploads.Load() == 1 && rejections.Load() == 1)
			w.WriteHeader(204)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	result := make(chan error, 1)
	title := "Network Toolkit Explorer107 native acceptance"
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "native-explorer-drop.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	profile, profileErr := os.MkdirTemp("", "network-toolkit-explorer107-webview-")
	if profileErr != nil {
		t.Fatal(profileErr)
	}
	t.Cleanup(func() {
		deadline := time.Now().Add(10 * time.Second)
		for {
			if e := os.RemoveAll(profile); e == nil {
				return
			} else if time.Now().After(deadline) {
				t.Errorf("synthetic WebView profile remained locked: %v", e)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	err = Run(ctx, Config{Title: title, URL: server.URL + "/", SessionToken: token, DataDir: profile, Stop: cancel, OnReady: func() {
		// OnReady runs on the host UI thread after visibility/startup checks.
		// Bind the driver to this exact synthetic host, not a desktop title
		// search or PowerShell's null-string/PInvoke class conversion.
		var hwnd uintptr
		matches := 0
		wndWindows.Range(func(key, value any) bool {
			w, ok := value.(*hostWindow)
			if ok && w.cfg.DataDir == profile && w.cfg.URL == server.URL+"/" && w.cfg.Title == title && w.ready && w.mainShown && w.hwnd == key.(uintptr) {
				hwnd = w.hwnd
				matches++
			}
			return true
		})
		if matches != 1 || hwnd == 0 {
			result <- fmt.Errorf("expected one ready synthetic fixture HWND, found %d", matches)
			cancel()
			return
		}
		go func() {
			cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", script, "-SourceDirectory", source, "-WindowHandle", strconv.FormatUint(uint64(hwnd), 10), "-OwnerProcessID", strconv.Itoa(os.Getpid()))
			output, runErr := cmd.CombinedOutput()
			if runErr != nil {
				if out := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT"); out != "" {
					_ = os.MkdirAll(out, 0700)
					if _, _, captureErr := captureNativeVaultDialog(hwnd, filepath.Join(out, "native-explorer107-failure.png")); captureErr != nil {
						output = append(output, []byte(fmt.Sprintf("\nFailure capture: %v\n", captureErr))...)
					}
					_ = os.WriteFile(filepath.Join(out, "native-explorer107-failure.txt"), output, 0600)
				}
				result <- fmt.Errorf("actual Explorer drag: %w: %s", runErr, output)
				cancel()
				return
			}
			if out := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT"); out != "" {
				if _, _, e := captureNativeVaultDialog(hwnd, filepath.Join(out, "native-explorer107-copy.png")); e != nil {
					result <- e
					cancel()
					return
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": complete.Load(), "actualExplorer": true, "uploads": uploads.Load(), "wholeFolderDropRejected": rejections.Load() == 1, "transport": "installed WebView2 browser File/DataTransfer, native private bootstrap", "scope": "synthetic native helper fixture; not a live remote upload", "output": string(output)}, "", "  ")
				if e := os.WriteFile(filepath.Join(out, "native-explorer107-report.json"), report, 0600); e != nil {
					result <- e
					cancel()
					return
				}
			}
			result <- nil
			cancel()
		}()
	}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-result:
		if e != nil {
			t.Fatal(e)
		}
	default:
		t.Fatal("required actual Explorer driver did not finish")
	}
	if !complete.Load() {
		t.Fatalf("native copy missing: uploads=%d rejected=%d", uploads.Load(), rejections.Load())
	}
	for _, dir := range []string{source, destination} {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil || string(b) != content {
			t.Fatalf("source/destination bytes %s: %s %v", dir, b, e)
		}
	}
}
