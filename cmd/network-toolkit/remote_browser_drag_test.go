package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteDragTicketLocalSingleUse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "network-settings.txt")
	if err := os.WriteFile(path, []byte("correct bytes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	app := &application{}
	requestBody, _ := json.Marshal(remoteBrowserRequest{Protocol: "local", Action: "stat", Path: path})
	rec := httptest.NewRecorder()
	app.handleRemoteDragTicket(rec, httptest.NewRequest(http.MethodPost, "/api/remote-browser/drag-ticket", strings.NewReader(string(requestBody))))
	if rec.Code != http.StatusOK {
		t.Fatalf("ticket response: %d %s", rec.Code, rec.Body.String())
	}
	var ticket struct {
		Ticket   string `json:"ticket"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	if len(ticket.Ticket) != 48 || ticket.Filename != "network-settings.txt" {
		t.Fatalf("invalid ticket: %+v", ticket)
	}
	rec2 := httptest.NewRecorder()
	app.handleRemoteDragDownload(rec2, httptest.NewRequest(http.MethodGet, "/drag-download/"+ticket.Ticket, nil))
	if rec2.Code != http.StatusOK || rec2.Body.String() != "correct bytes\n" {
		t.Fatalf("download: %d %s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Header().Get("Content-Disposition"), "network-settings.txt") {
		t.Fatal("missing attachment filename")
	}
	again := httptest.NewRecorder()
	app.handleRemoteDragDownload(again, httptest.NewRequest(http.MethodGet, "/drag-download/"+ticket.Ticket, nil))
	if again.Code != http.StatusNotFound {
		t.Fatalf("ticket was reused: %d", again.Code)
	}
}

func TestRemoteDragTicketRejectsCredentialsAndFolders(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "item.txt")
	if err := os.WriteFile(file, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	app := &application{}
	cases := []remoteBrowserRequest{
		{Protocol: "local", Action: "stat", Path: dir},
		{Protocol: "local", Action: "stat", Path: file, Password: "bad"},
		{Protocol: "local", Action: "stat", Path: file, SessionID: "other"},
		{Protocol: "local", Action: "remove", Path: file},
	}
	for _, test := range cases {
		value, _ := json.Marshal(test)
		rec := httptest.NewRecorder()
		app.handleRemoteDragTicket(rec, httptest.NewRequest(http.MethodPost, "/api/remote-browser/drag-ticket", strings.NewReader(string(value))))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("accepted invalid payload: %s, got %d", value, rec.Code)
		}
	}
}
