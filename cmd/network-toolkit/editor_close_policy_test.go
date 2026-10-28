package main

import "testing"

func TestMainEditorCloseApprovalOptIn(t *testing.T) {
	for _, title := range []string{appName, appName + " Terminal"} {
		if !mainEditorCloseApproval(title) {
			t.Fatalf("primary editor-capable workspace %q must require close approval", title)
		}
	}
	for _, title := range []string{"", appName + " Terminal child", appName + " Editor"} {
		if mainEditorCloseApproval(title) {
			t.Fatalf("unrecognized/child host %q must retain its explicit policy", title)
		}
	}
}
