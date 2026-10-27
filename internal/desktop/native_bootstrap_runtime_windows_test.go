//go:build windows

package desktop

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This is a real installed-WebView2/native-window test, not a COM mock or a
// cross-compile. CI must set the explicit switch; an unavailable runtime is a
// failure when requested, never a successful security acceptance.
func TestNativeBootstrapInstalledWebView2(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_SECURITY_TEST") != "1" {
		t.Skip("set NETWORK_TOOLKIT_NATIVE_SECURITY_TEST=1 on Windows with WebView2 for native security acceptance")
	}
	for _, document := range []string{"/", "/terminal-view/"+strings.Repeat("ab",16)} {
		t.Run(document, func(t *testing.T) {
			token := strings.Repeat("cd",32)
			var validProbe, ready atomic.Bool
			var unexpectedNavigation atomic.Int32
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				if r.URL.Path == "/favicon.ico" { w.WriteHeader(http.StatusNoContent); return }
				if r.URL.Path == "/native-probe" {
					if r.Header.Get("X-Native-Test") != token { http.Error(w,"unauthorized",http.StatusUnauthorized); return }
					validProbe.Store(true)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if r.URL.Path != document { unexpectedNavigation.Add(1); http.NotFound(w,r); return }
				w.Header().Set("Content-Type","text/html; charset=utf-8")
				_, _ = io.WriteString(w, `<!doctype html><title>Native security fixture</title><body>Native bootstrap security fixture<script>
window.__networkToolkitStartupState=0;
const capability=window.__networkToolkitNativeSession;
if(!/^[a-f0-9]{64}$/.test(capability||'')){window.__networkToolkitStartupState=-1;}else{
fetch('/native-probe',{headers:{'X-Native-Test':capability}}).then(response=>{
 if(!response.ok)throw Error('native capability unavailable');
 const frame=document.createElement('iframe');frame.src='/forbidden-frame';document.body.appendChild(frame);
 location.href='/forbidden-navigation';
 setTimeout(()=>{window.__networkToolkitStartupState=1;},300);
}).catch(()=>{window.__networkToolkitStartupState=-1;});}
</script></body>`)
			}))
			defer server.Close()
			// An ordinary local HTTP client can read the page but obtains neither
			// the secret bootstrap nor permission to the protected fixture API.
			response, err := http.Get(server.URL+document)
			if err != nil { t.Fatal(err) }
			body, _ := io.ReadAll(response.Body); response.Body.Close()
			if strings.Contains(string(body),token) { t.Fatal("HTTP page exposed native token") }
			response, err = http.Get(server.URL+"/native-probe")
			if err != nil { t.Fatal(err) }
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized { t.Fatal("ordinary client gained native capability") }
			dataDir, err := os.MkdirTemp("", "network-toolkit-native-security-")
			if err != nil { t.Fatal(err) }
			t.Cleanup(func(){
				deadline:=time.Now().Add(10*time.Second)
				for { if err:=os.RemoveAll(dataDir); err==nil { return } else if time.Now().After(deadline) { t.Errorf("synthetic WebView profile remained locked: %v",err); return }; time.Sleep(100*time.Millisecond) }
			})
			err=Run(ctx,Config{Title:"Network Toolkit native security test",URL:server.URL+document,SessionToken:token,DataDir:dataDir,Stop:cancel,OnReady:func(){ready.Store(true);cancel()}})
			if err!=nil { t.Fatal(err) }
			if !ready.Load()||!validProbe.Load(){ t.Fatal("installed WebView2 did not initialize through native-only capability") }
			if unexpectedNavigation.Load()!=0 { t.Fatal("native host permitted another document or frame navigation") }
		})
	}
}
