//go:build windows

package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	networktoolkit "networktoolkit"
)

func TestNativeExplorerFiles114(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_EXPLORER_TEST") != "1" {
		t.Skip("explicit hosted Windows Explorer acceptance only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	temp := t.TempDir()
	source := filepath.Join(temp, "source")
	dest := filepath.Join(temp, "destination")
	incoming := filepath.Join(source, "incoming-folder")
	os.MkdirAll(filepath.Join(incoming, "empty"), 0700)
	os.MkdirAll(dest, 0700)
	os.WriteFile(filepath.Join(incoming, "in.txt"), []byte("SYNTHETIC INCOMING114\r\n"), 0600)
	junctionTree := filepath.Join(source, "junction-tree")
	outside := filepath.Join(temp, "junction-outside")
	os.MkdirAll(junctionTree, 0700)
	os.MkdirAll(outside, 0700)
	os.WriteFile(filepath.Join(outside, "must-not-read.txt"), []byte("SYNTHETIC_LINK_TARGET114"), 0600)
	if output, err := exec.CommandContext(ctx, "cmd.exe", "/c", "mklink", "/J", filepath.Join(junctionTree, "linked"), outside).CombinedOutput(); err != nil {
		t.Fatalf("synthetic junction setup: %v %s", err, output)
	}

	stage, err := os.MkdirTemp("", "ntk-explorer-copy-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stage)
	os.MkdirAll(filepath.Join(stage, "outgoing-folder", "nested"), 0700)
	os.Mkdir(filepath.Join(stage, "outgoing-folder", "empty"), 0700)
	os.WriteFile(filepath.Join(stage, "outgoing-folder", "nested", "out.txt"), []byte("SYNTHETIC OUTGOING TREE114\r\n"), 0600)
	os.WriteFile(filepath.Join(stage, "outgoing-file.txt"), []byte("SYNTHETIC OUTGOING FILE114\r\n"), 0600)
	token := strings.Repeat("b7", 32)
	id := strings.Repeat("a1", 16)
	var mu sync.Mutex
	state := "idle"
	armed := false
	var gesture, revoked uint64
	starts := 0
	quickStarts, quickChecks := 0, 0
	imports := 0
	linkNegatives := 0
	finishes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			name := strings.TrimPrefix(r.URL.Path, "/assets/")
			if name != "remote-drag.js" && name != "transfer-drop.js" {
				http.NotFound(w, r)
				return
			}
			asset, e := networktoolkit.Web.ReadFile("web/assets/" + name)
			if e != nil {
				t.Error(e)
				http.Error(w, "asset", 500)
				return
			}
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(asset)
			return
		}
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(204)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>Files114 actual Explorer fixture</title><style>body{font:14px Segoe UI;background:#edf4ff;padding:12px}#target{border:3px dashed #185abb;height:180px;padding:16px}button{padding:12px;font:inherit}#modal{background:white;padding:12px}progress{max-width:100px}.transfer-drop-row{display:block}input,select{max-width:200px}</style><h1>Files114 native acceptance</h1><div id="target">Drop synthetic incoming folder here</div><button id="prepare">Prepare Explorer copy</button><div id="modal"></div><button id="quickBaseline">Record quick release</button><button id="quickCheck">Check quick release</button><p id="status">Ready</p><script>const SESSION=window.__networkToolkitNativeSession;window.__networkToolkitStartupState=/^[a-f0-9]{64}$/.test(SESSION||'')?1:-1;window.api=async(path,method='GET',body)=>{const r=await fetch('/api'+path,{method,headers:{'X-Session-Token':SESSION,'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});if(!r.ok)throw Error(await r.text());return r.status===204?{}:r.json()};window.toast=m=>document.getElementById('status').textContent=m;window.openModal=(title,body,footer)=>{document.getElementById('target').hidden=true;document.getElementById('modal').innerHTML='<h2>'+title+'</h2>'+body+footer;};window.closeModal=()=>{document.getElementById('modal').replaceChildren();document.getElementById('target').hidden=false;};</script><script src="/assets/transfer-drop.js"></script><script src="/assets/remote-drag.js"></script><script>
document.getElementById('quickBaseline').onclick=()=>api('/quick-baseline','POST',{}).then(()=>toast('Quick baseline recorded'));document.getElementById('quickCheck').onclick=()=>api('/quick-check','POST',{}).then(()=>toast('Quick release did not arm ordinary click')).catch(toast);const helper=NetworkToolkitTransferDrop;document.getElementById('prepare').onclick=()=>NetworkToolkitNativeDrag.prepareCopy({binding:{kind:'local'},paths:['synthetic-selection']}).catch(toast);document.addEventListener('dragover',e=>{e.preventDefault();e.dataTransfer.dropEffect='copy'});document.addEventListener('drop',async e=>{e.preventDefault();const linkTest=Array.from(e.dataTransfer.items).some(item=>item.webkitGetAsEntry?.()?.name==='junction-tree');try{const files=await helper.fromDropTree(e.dataTransfer);if(linkTest){if(files.some(row=>row.items?.some(item=>item.file)))throw Error('UNSAFE_LINK_TRAVERSAL');await api('/link-negative','POST',{});toast('Link target not traversed');return;}helper.show({files,entries:[],local:true,label:'Synthetic incoming destination',check(){},send:async row=>{if(!row.file.directory)throw Error('Expected a folder');for(const entry of row.file.items){if(entry.directory)continue;const form=new FormData();form.set('file',entry.file,entry.file.name);form.set('path',entry.path);await helper.upload('/incoming',form,{});}},done:async report=>{if(report.copied!==1||report.failed)throw Error('Import failed');document.getElementById('status').textContent='Incoming folder bytes verified';}});}catch(error){if(linkTest&&error.message!=='UNSAFE_LINK_TRAVERSAL'){await api('/link-negative','POST',{});toast('Link target not traversed');}else toast(error.message)}});</script>`)
			return
		}
		if r.Header.Get("X-Session-Token") != token {
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.URL.Path == "/api/quick-baseline" && r.Method == "POST" {
			mu.Lock()
			quickStarts = starts
			mu.Unlock()
			io.WriteString(w, `{"ok":true}`)
			return
		}
		if r.URL.Path == "/api/quick-check" && r.Method == "POST" {
			mu.Lock()
			defer mu.Unlock()
			if starts != quickStarts {
				http.Error(w, "ordinary click consumed stale drag", 409)
				return
			}
			quickChecks++
			io.WriteString(w, `{"ok":true}`)
			return
		}
		if r.URL.Path == "/api/link-negative" && r.Method == "POST" {
			mu.Lock()
			linkNegatives++
			mu.Unlock()
			io.WriteString(w, `{"ok":true}`)
			return
		}
		if r.URL.Path == "/incoming" {
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			if e := r.ParseMultipartForm(4096); e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, h, e := r.FormFile("file")
			if e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			defer f.Close()
			b, _ := io.ReadAll(f)
			if h.Filename != "in.txt" || r.FormValue("path") != "in.txt" || string(b) != "SYNTHETIC INCOMING114\r\n" {
				http.Error(w, "wrong browser File bytes", 400)
				return
			}
			mu.Lock()
			imports++
			mu.Unlock()
			io.WriteString(w, `{"ok":true}`)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/api/explorer-transfer/native" {
			if (state != "ready" && state != "claimed") || !armed {
				w.WriteHeader(204)
				return
			}
			state = "claimed"
			json.NewEncoder(w).Encode(nativeFileDragJob{ID: id, Gesture: gesture, Paths: []string{filepath.Join(stage, "outgoing-folder"), filepath.Join(stage, "outgoing-file.txt")}})
			return
		}
		if r.URL.Path != "/api/explorer-transfer" {
			http.NotFound(w, r)
			return
		}
		if r.Method == "POST" {
			var body struct {
				Action  string `json:"action"`
				Gesture uint64 `json:"gesture"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			switch body.Action {
			case "prepare":
				state = "ready"
				gesture = 0
				revoked = 0
			case "arm":
				if state != "ready" || body.Gesture <= revoked {
					http.Error(w, "not ready", 409)
					return
				}
				armed = true
				gesture = body.Gesture
			case "start":
				if body.Gesture != gesture || (state != "dragging" && body.Gesture <= revoked) {
					http.Error(w, "released", 409)
					return
				}
				if state != "dragging" {
					starts++
				}
				state = "dragging"
				armed = false
			case "disarm":
				if body.Gesture > revoked {
					revoked = body.Gesture
				}
				if state == "claimed" {
					state = "ready"
				}
				armed = false
			case "finish":
				finishes++
				state = "finished"
			case "cancel":
				state = "cancelled"
			default:
				http.Error(w, "bad action", 400)
				return
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"id": id, "state": state, "names": []string{"outgoing-folder", "outgoing-file.txt"}, "entries": 5, "bytes": 56, "message": state})
	}))
	defer server.Close()
	profile, profileErr := os.MkdirTemp("", "network-toolkit-files114-webview-")
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
	result := make(chan error, 1)
	title := "Network Toolkit Files114 native acceptance"
	script, _ := filepath.Abs(filepath.Join("..", "..", "scripts", "native-explorer-files114.ps1"))
	err = Run(ctx, Config{Title: title, URL: server.URL + "/", SessionToken: token, DataDir: profile, Stop: cancel, OnReady: func() {
		var hwnd uintptr
		wndWindows.Range(func(key, value any) bool {
			w, ok := value.(*hostWindow)
			if ok && w.cfg.DataDir == profile && w.cfg.URL == server.URL+"/" && w.cfg.Title == title && w.ready && w.mainShown && w.hwnd == key.(uintptr) {
				hwnd = key.(uintptr)
			}
			return true
		})
		if hwnd == 0 {
			result <- fmt.Errorf("native fixture HWND unavailable")
			cancel()
			return
		}
		go func() {
			cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", script, "-SourceDirectory", source, "-DestinationDirectory", dest, "-WindowHandle", strconv.FormatUint(uint64(hwnd), 10), "-OwnerProcessID", strconv.Itoa(os.Getpid()))
			output, e := cmd.CombinedOutput()
			out := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT")
			if out != "" {
				os.MkdirAll(out, 0700)
				_, _, ce := captureNativeVaultDialog(hwnd, filepath.Join(out, "native-files114.png"))
				if ce != nil && e == nil {
					e = ce
				}
				os.WriteFile(filepath.Join(out, "native-files114-driver.txt"), output, 0600)
			}
			if e != nil {
				result <- fmt.Errorf("actual Explorer gestures: %w: %s", e, output)
			} else {
				result <- nil
			}
			cancel()
		}()
	}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("native driver unfinished")
	}
	mu.Lock()
	defer mu.Unlock()
	if imports != 1 || finishes != starts || starts < 2 || starts > 3 || quickChecks != 1 || linkNegatives != 1 {
		t.Fatalf("actual imported=%d finished/cancelled drags=%d", imports, finishes)
	}
	for _, rel := range []string{"outgoing-file.txt", filepath.Join("outgoing-folder", "nested", "out.txt")} {
		want, e := os.ReadFile(filepath.Join(stage, rel))
		got, ge := os.ReadFile(filepath.Join(dest, rel))
		if e != nil || ge != nil || string(want) != string(got) {
			t.Fatalf("OS destination bytes differ: %s", rel)
		}
	}
	if info, e := os.Stat(filepath.Join(dest, "outgoing-folder", "empty")); e != nil || !info.IsDir() {
		t.Fatal("empty folder lost")
	}
	if out := os.Getenv("TOOLKIT_NATIVE_TEST_OUTPUT"); out != "" {
		b, _ := json.MarshalIndent(map[string]any{"passed": true, "actualExplorer": true, "importedFolders": imports, "junctionTargetNotTraversed": linkNegatives == 1, "nativeDragsIncludingCancel": finishes, "copyOnly": true, "releasedGestureDidNotArmLaterClick": quickChecks == 1, "scope": "synthetic hosted FileSystemEntry + native OLE bridge; remote staging separately tested"}, "", "  ")
		os.WriteFile(filepath.Join(out, "native-files114-report.json"), b, 0600)
	}
}
