package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func direct121(t *testing.T, a *application, paths []string) string {
	t.Helper()
	w := exportRequest(a, "", "POST", "/api/explorer-transfer", explorerTransferRequest{Action: "direct", Binding: editorBinding{Kind: "local"}, Paths: paths})
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if out["state"] != "ready" {
		t.Fatal(out)
	}
	return out["id"].(string)
}
func directAction121(a *application, id, action string) *httptest.ResponseRecorder {
	return exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]any{"action": action, "id": id, "gesture": 1})
}
func startDirect121(t *testing.T, a *application, id string) {
	t.Helper()
	if w := directAction121(a, id, "arm"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w := exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"direct":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "ntk-explorer-copy-") {
		t.Fatal("staged bytes before drop")
	}
	if w := directAction121(a, id, "start"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}
func TestExplorerDirect121LazyCopyAndCleanup(t *testing.T) {
	a := &application{}
	root := t.TempDir()
	source := filepath.Join(root, "folder")
	os.MkdirAll(filepath.Join(source, "empty"), 0700)
	os.WriteFile(filepath.Join(source, "file.txt"), []byte("SYNTHETIC_DIRECT121\r\n"), 0600)
	id := direct121(t, a, []string{source})
	defer a.finishExplorerOwner("")
	j := a.explorerTransfers.jobs[id]
	if j.stage != "" || j.entries != 0 || j.renderStarted {
		t.Fatal("registration read source")
	}
	if w := directAction121(a, id, "materialize"); w.Code != 409 {
		t.Fatal("unaccepted drop read source", w.Code)
	}
	startDirect121(t, a, id)
	if w := directAction121(a, id, "materialize"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var paths []string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w := exportRequest(a, "", "GET", "/api/explorer-transfer/native?id="+id, nil)
		if w.Code == 200 {
			var out struct {
				Paths []string `json:"paths"`
			}
			json.Unmarshal(w.Body.Bytes(), &out)
			paths = out.Paths
			break
		}
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
	if len(paths) != 1 {
		t.Fatal("deferred paths missing")
	}
	if data, e := os.ReadFile(filepath.Join(paths[0], "file.txt")); e != nil || string(data) != "SYNTHETIC_DIRECT121\r\n" {
		t.Fatal("bytes differ", e)
	}
	if info, e := os.Stat(filepath.Join(paths[0], "empty")); e != nil || !info.IsDir() {
		t.Fatal("empty folder lost")
	}
	if w := directAction121(a, id, "materialize"); w.Code != 200 {
		t.Fatal("duplicate receiver probe not idempotent")
	}
	stage := filepath.Dir(paths[0])
	w := exportRequest(a, "", "POST", "/api/explorer-transfer", map[string]any{"action": "finish", "id": id, "outcome": "copied"})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if _, e := os.Stat(stage); !os.IsNotExist(e) {
		t.Fatal("staging leaked")
	}
	if _, e := os.Stat(source); e != nil {
		t.Fatal("source deleted")
	}
}
func TestExplorerDirect121CancelAndLateGestureNeverRead(t *testing.T) {
	for _, mode := range []string{"cancel-before-arm", "released-before-start", "cancel-during-drop"} {
		t.Run(mode, func(t *testing.T) {
			a := &application{}
			id := direct121(t, a, []string{filepath.Join(t.TempDir(), "missing-valid-name.txt")})
			defer a.finishExplorerOwner("")
			j := a.explorerTransfers.jobs[id]
			switch mode {
			case "cancel-before-arm":
				if w := directAction121(a, id, "cancel"); w.Code != 200 {
					t.Fatal(w.Code)
				}
				if w := directAction121(a, id, "arm"); w.Code != 409 {
					t.Fatal("cancelled gesture resurrected")
				}
			case "released-before-start":
				directAction121(a, id, "arm")
				exportRequest(a, "", "GET", "/api/explorer-transfer/native", nil)
				directAction121(a, id, "disarm")
				if w := directAction121(a, id, "start"); w.Code != 409 {
					t.Fatal("released gesture started")
				}
			case "cancel-during-drop":
				startDirect121(t, a, id)
				if w := directAction121(a, id, "cancel"); w.Code != 200 {
					t.Fatal(w.Code)
				}
				if w := directAction121(a, id, "materialize"); w.Code != 409 {
					t.Fatal("cancelled drop read source")
				}
			}
			if j.renderStarted || j.stage != "" || j.entries != 0 {
				t.Fatal("cancelled operation touched source")
			}
		})
	}
}

func TestExplorerDirect121ExplicitCancelWinsInterruptedReadError(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		a := &application{}
		j := &explorerTransferJob{direct: true, cancelRequested: cancelled, state: "dragging", names: []string{"synthetic.txt"}, valid: func() bool { return true }}
		g := &editorDocumentGrant{run: func(context.Context, remoteBrowserRequest) (model.Result, error) { return model.Result{}, io.EOF }}
		a.stageExplorerExport(context.Background(), j, editorBinding{Kind: "sshConnection"}, []string{"/synthetic.txt"}, g)
		if cancelled {
			if j.state != "cancelled" || !strings.Contains(j.note, "cancelled") {
				t.Fatalf("explicit cancellation misreported: %s %s", j.state, j.note)
			}
		} else if j.state != "failed" || j.note != "EOF" {
			t.Fatalf("real read failure hidden: %s %s", j.state, j.note)
		}
		if j.stage != "" || len(j.paths) > 0 || j.rendered {
			t.Fatal("failed/cancelled read published staged paths")
		}
	}
}
