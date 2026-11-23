//go:build windows

package desktop

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

// This is a real installed-WebView2/native-window test, not a COM mock or a
// cross-compile. CI must set the explicit switch; an unavailable runtime is a
// failure when requested, never a successful security acceptance.
func TestNativeBootstrapInstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_SECURITY_TEST") != "1" {
		t.Skip("set NETWORK_TOOLKIT_NATIVE_SECURITY_TEST=1 on Windows with WebView2 for native security acceptance")
	}
	// Bounded diagnostic A/B: fresh hosts, two repeats per document/scenario,
	// and reversed ordering on the second repeat. All security assertions stay.
	for _, document := range []string{"/", "/terminal-view/" + strings.Repeat("ab", 16), "/editor-view/" + strings.Repeat("cd", 16)} {
		for repeat := 1; repeat <= 2; repeat++ {
			popups := []bool{false, true}
			if repeat == 2 {
				popups = []bool{true, false}
			}
			for _, popup := range popups {
				scenario := "without-popup"
				if popup {
					scenario = "with-popup"
				}
				t.Run(fmt.Sprintf("%s/%s/repeat-%d", document, scenario, repeat), func(t *testing.T) {
					runNativeBootstrapDocument(t, document, popup)
				})
			}
		}
	}
}

func runNativeBootstrapDocument(t *testing.T, document string, popup bool) {
	t.Helper()
	popupScript := "const popup=null;"
	if popup {
		popupScript = "const popup=window.open('/forbidden-popup','_blank');"
	}
	token := strings.Repeat("cd", 32)
	var validProbe, ready, forbiddenExecuted atomic.Bool
	var settingsError error // Written on the Run UI thread; read after Run returns.
	var reportError error
	var report *nativeSecurityBlobReport
	var reportVerified atomic.Bool
	var reportSaves atomic.Int32
	reportStarted := false
	reportDone := make(chan struct{})
	reportPath := filepath.Join(t.TempDir(), "native-security-report.json")
	const reportBytes = "{\"synthetic\":\"NATIVE_REPORT\"}\n"
	var unexpectedNavigation, nativeDocumentRequests atomic.Int32
	var requestMu sync.Mutex
	var unexpectedRequests []string
	// 90 s per document: hosted Windows runners have needed up to 40 s to bring
	// up an installed WebView2 (runtime 156) for the third document; the old
	// 35 s budget turned that into an intermittent failure. Readiness and the
	// valid probe are still required; only the wait is longer.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/native-probe" {
			if r.Header.Get("X-Native-Test") != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			validProbe.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		repeatDocument := r.URL.Path == document && r.Header.Get("Sec-Fetch-Dest") == "document" && nativeDocumentRequests.Add(1) != 1
		if r.URL.Path != document || r.Header.Get("Sec-Fetch-Dest") == "iframe" || repeatDocument {
			unexpectedNavigation.Add(1)
			requestMu.Lock()
			unexpectedRequests = append(unexpectedRequests, fmt.Sprintf("%s %s destination=%q mode=%q", r.Method, r.URL.RequestURI(), r.Header.Get("Sec-Fetch-Dest"), r.Header.Get("Sec-Fetch-Mode")))
			requestMu.Unlock()
			if r.URL.Path == "/forbidden-executed" {
				forbiddenExecuted.Store(true)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.URL.Path == "/forbidden-frame" || r.URL.Path == "/forbidden-navigation" || r.URL.Path == "/forbidden-cross-site-frame" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				// Only record whether a capability exists, never its value.
				_, _ = io.WriteString(w, `<!doctype html><script>fetch('/forbidden-executed?capability='+(typeof window.__networkToolkitNativeSession!=='undefined')).finally(()=>{window.__networkToolkitStartupState=1;});</script>`)
				return
			}
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html><title>Native security fixture</title><body>Native bootstrap security fixture<script>
window.__networkToolkitStartupState=0;
const capability=window.__networkToolkitNativeSession;
if(!/^[a-f0-9]{64}$/.test(capability||'')){window.__networkToolkitStartupState=-1;}else{
fetch('/native-probe',{headers:{'X-Native-Test':capability}}).then(response=>{
 if(!response.ok)throw Error('native capability unavailable');
 alert('Synthetic browser alert must not appear');
 if(confirm('Synthetic browser confirmation must not appear')!==false)throw Error('browser confirmation accepted');
 const textAnswer=prompt('Synthetic browser prompt must not appear','fixture');
 if(textAnswer!==null&&textAnswer!==false)throw Error('browser prompt accepted');
 const appDialog=document.createElement('dialog');appDialog.textContent='Synthetic application dialog';document.body.appendChild(appDialog);
 appDialog.showModal();if(!appDialog.open)throw Error('application dialog was blocked');appDialog.close();appDialog.remove();
 ` + popupScript + `
 const frame=document.createElement('iframe');frame.src='/forbidden-frame';document.body.appendChild(frame);
 const exactFrame=document.createElement('iframe');exactFrame.src=location.href;document.body.appendChild(exactFrame);
 const crossSiteFrame=document.createElement('iframe');const crossSiteURL=new URL('/forbidden-cross-site-frame',location.href);crossSiteURL.hostname='localhost';crossSiteFrame.src=crossSiteURL.href;document.body.appendChild(crossSiteFrame);
 location.href='/forbidden-navigation';
 setTimeout(()=>{location.hash='same-document';},75);
 setTimeout(()=>{location.reload();},150);
 setTimeout(()=>{window.__networkToolkitStartupState=location.hash==='#same-document'&&(!popup||popup.closed)?1:-1;},500);
}).catch(()=>{window.__networkToolkitStartupState=-1;});}
</script></body>`)
	}))
	defer server.Close()
	// An ordinary local HTTP client can read the page but obtains neither
	// the secret bootstrap nor permission to the protected fixture API.
	response, err := http.Get(server.URL + document)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if strings.Contains(string(body), token) {
		t.Fatal("HTTP page exposed native token")
	}
	response, err = http.Get(server.URL + "/native-probe")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatal("ordinary client gained native capability")
	}
	dataDir, err := os.MkdirTemp("", "network-toolkit-native-security-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		deadline := time.Now().Add(10 * time.Second)
		for {
			if err := os.RemoveAll(dataDir); err == nil {
				return
			} else if time.Now().After(deadline) {
				t.Errorf("synthetic WebView profile remained locked: %v", err)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	err = Run(ctx, Config{Title: "Network Toolkit native security test", URL: server.URL + document, SessionToken: token, DataDir: dataDir, Stop: cancel, OnReady: func() {
		ready.Store(true)
		settingsError = checkNativeBrowserSettings(dataDir)
		if settingsError == nil && document == "/" && !popup {
			report, reportError = startNativeSecurityBlobReport(dataDir, reportPath, &reportSaves)
			if reportError == nil {
				reportStarted = true
				post := report.view.cfg.Post
				go func() {
					defer close(reportDone)
					deadline := time.NewTimer(15 * time.Second)
					defer deadline.Stop()
					ticker := time.NewTicker(25 * time.Millisecond)
					defer ticker.Stop()
					for {
						status := make(chan struct {
							state int32
							err   error
						}, 1)
						if !post(func() {
							state, err := report.state()
							status <- struct {
								state int32
								err   error
							}{state, err}
						}) {
							reportError = fmt.Errorf("native blob report state dispatch was refused")
							cancel()
							return
						}
						select {
						case current := <-status:
							if current.err != nil || current.state == 1 {
								reportError = fmt.Errorf("native blob report did not complete: state=%d error=%v", current.state, current.err)
								cancel()
								return
							}
							if current.state == 2 {
								if data, err := os.ReadFile(reportPath); err == nil && string(data) == reportBytes {
									reportVerified.Store(true)
									cancel()
									return
								}
							}
						case <-ctx.Done():
							return
						case <-deadline.C:
							reportError = fmt.Errorf("native blob report completion timed out")
							cancel()
							return
						}
						select {
						case <-ctx.Done():
							return
						case <-deadline.C:
							reportError = fmt.Errorf("native blob report completion timed out")
							cancel()
							return
						case <-ticker.C:
						}
					}
				}()
				return
			}
		}
		cancel()
	}, OnClosing: func() {
		if report != nil {
			report.close()
		}
	}, navigationObserver: func(observation nativeNavigationObservation) {
		t.Logf("native navigation: kind=%s URI=%q blocked=%t cancelHRESULT=%#x readHRESULT=%#x cancelled=%t initialPermit=%t resourceContext=%d contextHRESULT=%#x responseHRESULT=%#x protocolMethod=%q protocolSession=%q runtime=%q", observation.Kind, observation.URI, observation.Blocked, observation.CancelResult, observation.CancelReadResult, observation.Cancelled, observation.InitialDocumentPermit, observation.ResourceContext, observation.ResourceContextResult, observation.ResponseResult, observation.ProtocolMethod, observation.ProtocolSession, observation.RuntimeVersion)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if reportStarted {
		<-reportDone
	}
	if !ready.Load() || !validProbe.Load() {
		t.Fatal("installed WebView2 did not initialize through native-only capability")
	}
	if settingsError != nil {
		t.Fatal(settingsError)
	}
	if document == "/" && !popup {
		if reportError != nil || !reportVerified.Load() || reportSaves.Load() != 1 {
			t.Fatalf("native blob report did not complete through DownloadStarting: saves=%d exactBytes=%t error=%v", reportSaves.Load(), reportVerified.Load(), reportError)
		}
		if data, err := os.ReadFile(reportPath); err != nil || string(data) != reportBytes {
			t.Fatalf("native blob report did not retain exact bytes after host shutdown: %v", err)
		}
		t.Log("native blob report reached DownloadState Completed and retained exact synthetic bytes after host shutdown")
	}
	if nativeDocumentRequests.Load() != 1 {
		t.Fatalf("expected exactly one native document request, got %d", nativeDocumentRequests.Load())
	}
	if unexpectedNavigation.Load() != 0 {
		requestMu.Lock()
		requests := append([]string(nil), unexpectedRequests...)
		requestMu.Unlock()
		t.Fatalf("native host permitted another document or frame navigation (forbidden script executed=%t): %v", forbiddenExecuted.Load(), requests)
	}
}

// Use the actual native DownloadStarting/deferral path, with only the test
// host's destination chooser replaced by an already-approved temporary path.
// This keeps the unattended fixture from opening a Windows Save dialog.
func startNativeSecurityBlobReport(dataDir, destination string, saves *atomic.Int32) (*nativeSecurityBlobReport, error) {
	var view *WebView
	wndWindows.Range(func(_, value any) bool {
		if host, ok := value.(*hostWindow); ok && host.cfg.DataDir == dataDir && host.ready {
			view = host.view
		}
		return true
	})
	if view == nil || view.closed || view.core == 0 {
		return nil, fmt.Errorf("native blob report fixture host was not found")
	}
	report := &nativeSecurityBlobReport{view: view}
	view.cfg.SaveDownload = func(suggested string) (string, bool) {
		if report.closed || suggested != "native-security-report.json" || report.operation != 0 || len(view.pendingDownloads) != 1 {
			return "", false
		}
		// The chooser runs on the UI thread while DownloadStarting still owns
		// this sole synthetic operation. Keep a separate reference because the
		// production StateChanged callback releases its own on completion.
		for pending := range view.pendingDownloads {
			report.operation = pending.operation
			comAddRef(report.operation)
		}
		saves.Add(1)
		return destination, true
	}
	script := utf16String(`(()=>{const blob=new Blob(['{"synthetic":"NATIVE_REPORT"}\n'],{type:'application/json'});const link=document.createElement('a');link.href=URL.createObjectURL(blob);link.download='native-security-report.json';document.body.appendChild(link);link.click();link.remove();})();`)
	handler := newCOMHandler(iidScriptCompleted, func(result, value uintptr) uintptr { return hresultOK })
	hr := comCall(view.core, 29, uintptr(unsafe.Pointer(script)), handler.pointer())
	handler.release()
	runtime.KeepAlive(script)
	return report, hresultError("Start native synthetic blob report", hr)
}

// All fields and COM access remain on the native UI thread. The file polling
// goroutine posts state reads, and OnClosing releases the retained operation.
type nativeSecurityBlobReport struct {
	view      *WebView
	operation uintptr
	closed    bool
}

func (r *nativeSecurityBlobReport) state() (int32, error) {
	if r.closed || r.view.closed {
		return -1, fmt.Errorf("native blob report host closed before completion")
	}
	if r.operation == 0 {
		return 0, nil
	}
	state := int32(-1)
	err := hresultError("Read native synthetic download state", comCall(r.operation, 16, uintptr(unsafe.Pointer(&state))))
	return state, err
}

func (r *nativeSecurityBlobReport) close() {
	if !r.closed {
		r.closed = true
		comRelease(r.operation)
		r.operation = 0
	}
}
