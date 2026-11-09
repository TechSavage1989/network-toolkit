package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"

	"networktoolkit/internal/editorbuffer"
)

type editorDiskVerify struct {
	hash       hash.Hash
	received   int64
	lastOffset int64
	lastDigest [32]byte
	finished   bool
}

// A writable browser handle is re-read before replacement. Only the running
// digest and one retry receipt are retained, even for a multi-gigabyte file.
func (d *editorDiskDocument) verifyBrowserSource(req editorDiskRequest) (any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.mode != "upload" {
		return nil, errors.New("only imported browser files can be verified this way")
	}
	if req.Action == "verify-start" {
		d.verify = &editorDiskVerify{hash: sha256.New(), lastOffset: -1}
		return map[string]any{"received": 0}, nil
	}
	check := d.verify
	if check == nil {
		return nil, errors.New("start verification before reading the browser file")
	}
	if req.Action == "verify-finish" {
		if check.received != req.Size {
			return nil, errors.New("the browser file has not been fully verified")
		}
		check.finished = true
		return map[string]any{"hash": hex.EncodeToString(check.hash.Sum(nil))}, nil
	}
	if check.finished || len(req.Data) == 0 || len(req.Data) > editorbuffer.MaxEditBytes {
		return nil, errors.New("invalid browser file verification chunk")
	}
	digest := sha256.Sum256(req.Data)
	if req.Offset == check.received {
		check.hash.Write(req.Data)
		check.received += int64(len(req.Data))
		check.lastOffset, check.lastDigest = req.Offset, digest
	} else if req.Offset < 0 || req.Offset != check.lastOffset || check.lastDigest != digest {
		return nil, errors.New("browser file verification chunk does not match the previous bytes")
	}
	return map[string]any{"received": check.received}, nil
}
