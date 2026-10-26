package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpdateCommitRequiresHelperSuccess(t *testing.T) {
	for _, kind := range []string{"legacy-success", "success", "failure", "missing", "cancelled", "wrong-version"} {
		t.Run(kind, func(t *testing.T) {
			dir, _ := os.MkdirTemp(t.TempDir(), ".network-toolkit-update-")
			p := updatePlan{Target: "app.exe", WorkingDir: t.TempDir(), OriginalSHA256: strings.Repeat("a", 64), CandidateSHA256: strings.Repeat("b", 64), Version: "1.17.60", ParentPID: 1, Nonce: strings.Repeat("c", 64)}
			b, _ := json.Marshal(p)
			os.WriteFile(filepath.Join(dir, "plan.json"), b, 0600)
			result := ""
			switch kind {
			case "legacy-success":
				result = "Installed 1.17.60 successfully. Previous executable retained as previous.exe.\n"
			case "success":
				result = "Installed 1.17.60 and confirmed the application interface started. Previous executable retained as previous.exe.\n"
			case "failure":
				result = "New app exited. The previous executable was restored and restarted"
			case "wrong-version":
				result = "Installed 1.17.61 successfully. Previous executable retained as previous.exe."
			}
			if result != "" {
				os.WriteFile(filepath.Join(dir, "update-result.txt"), []byte(result), 0600)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancelled" {
				cancel()
			}
			err := awaitUpdateCommit(ctx, dir, "1.17.60", 20*time.Millisecond)
			success := kind == "legacy-success" || kind == "success"
			if (err == nil) != success {
				t.Fatalf("confirmation result %v, expected success %v", err, success)
			}
		})
	}
}
