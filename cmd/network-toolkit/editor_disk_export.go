package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"

	"networktoolkit/internal/editorbuffer"
)

type editorDiskRange struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type editorDiskExport struct {
	id       string
	hash     string
	done     chan struct{}
	format   editorbuffer.Format
	snapshot *editorbuffer.Snapshot
}

func (d *editorDiskDocument) export(ctx context.Context, w http.ResponseWriter, b *editorbuffer.Buffer, req editorDiskRequest) {
	snapshot, err := b.Snapshot()
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	defer snapshot.Close()
	if snapshot.Revision != req.Revision {
		jsonError(w, 409, editorbuffer.ErrRevision.Error())
		return
	}
	var selection *[2]int64
	if req.Range != nil {
		selection = &[2]int64{req.Range.From, req.Range.To}
	}
	count, encode, err := snapshot.PrepareEncoding(req.Format, selection)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	var receipt *editorDiskExport
	d.mu.Lock()
	if d.mode == "upload" && selection == nil {
		receipt = &editorDiskExport{id: randomHex(24), format: req.Format, snapshot: snapshot, done: make(chan struct{})}
		d.exported = receipt
		w.Header().Set("X-Editor-Export", receipt.id)
	}
	d.mu.Unlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(count, 10))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if receipt == nil {
		if err := encode(ctx, w); err != nil {
			panic(http.ErrAbortHandler)
		}
		return
	}
	hash := sha256.New()
	err = encode(ctx, io.MultiWriter(w, hash))
	if err == nil {
		receipt.hash = hex.EncodeToString(hash.Sum(nil))
	}
	close(receipt.done)
	if err != nil {
		// Terminate a partial body so the browser aborts its staged writer.
		panic(http.ErrAbortHandler)
	}
}

func (d *editorDiskDocument) markExportSaved(ctx context.Context, imported *editorbuffer.Imported, id string) (any, error) {
	d.mu.Lock()
	receipt := d.exported
	d.mu.Unlock()
	if receipt == nil || receipt.id != id {
		return nil, errors.New("the exported revision is unavailable")
	}
	select {
	case <-receipt.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.exported != receipt || receipt.hash == "" || d.saving {
		return nil, errors.New("the exported revision did not finish successfully")
	}
	meta := receipt.snapshot.MarkSaved()
	imported.Format, imported.Hash = receipt.format, receipt.hash
	return map[string]any{"metadata": meta, "hash": receipt.hash, "format": receipt.format}, nil
}
