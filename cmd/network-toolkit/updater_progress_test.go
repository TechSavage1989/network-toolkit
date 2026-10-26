package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUpdateDownloadPublishesMeasuredBytesWithoutBlockingStatus(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	app := &application{updater: appUpdater{progress: &updateProgress{Phase: "download", Version: "1.17.99"}, networkBusy: true}}
	client := &http.Client{Transport: updateRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: 6, Body: reader}, nil
	})}
	done := make(chan error, 1)
	go func() {
		app.updater.mu.Lock()
		data, err := app.updateDownloadUnlocked(context.Background(), client, updateAPIURL, 64, "1.17.99")
		app.updater.mu.Unlock()
		if err == nil && string(data) != "abcdef" {
			t.Errorf("downloaded bytes: %q", data)
		}
		done <- err
	}()
	if _, err := writer.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	observed := false
	for time.Now().Before(deadline) {
		app.updater.mu.Lock()
		p := *app.updater.progress
		app.updater.mu.Unlock()
		if p.Received == 3 && p.Total == 6 {
			observed = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !observed {
		t.Fatal("partial byte progress was unavailable while download was pending")
	}
	select {
	case <-done:
		t.Fatal("download finished before the remaining bytes")
	default:
	}
	_, _ = writer.Write([]byte("def"))
	writer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("download did not finish")
	}
}
func TestUpdateDownloadProgressHandlesUnknownAndOversizedBodies(t *testing.T) {
	for _, total := range []int64{-1, 6} {
		var last, totalSeen int64
		client := &http.Client{Transport: updateRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, ContentLength: total, Body: io.NopCloser(strings.NewReader("abcdef"))}, nil
		})}
		data, err := updateFetchProgress(context.Background(), client, updateAPIURL, 64, func(received, size int64) {
			if received < last {
				t.Error("progress moved backward")
			}
			last = received
			totalSeen = size
		})
		if err != nil || string(data) != "abcdef" || last != 6 || totalSeen != max(total, 0) {
			t.Fatalf("progress %d/%d: %q %v", last, totalSeen, data, err)
		}
		if _, err = updateFetchProgress(context.Background(), client, updateAPIURL, 3, func(int64, int64) {}); err == nil {
			t.Fatal("progress bypassed download size limit")
		}
	}
}
