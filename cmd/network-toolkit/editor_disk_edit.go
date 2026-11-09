package main

import (
	"crypto/sha256"
	"errors"
	"time"

	"networktoolkit/internal/editorbuffer"
)

type editorDiskEdit struct {
	id       string
	revision uint64
	from, to int64
	stage    *editorbuffer.EditTransaction
	offset   int64
	digest   [32]byte
	timer    *time.Timer
}

func (d *editorDiskDocument) clearBulkLocked() {
	if d.bulk != nil {
		if d.bulk.timer != nil {
			d.bulk.timer.Stop()
		}
		d.bulk.stage.Abort()
		d.bulk = nil
	}
}

func (d *editorDiskDocument) extendBulkLocked(edit *editorDiskEdit) {
	if edit.timer != nil {
		edit.timer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(5*time.Minute, func() {
		d.editMu.Lock()
		defer d.editMu.Unlock()
		if d.bulk == edit && edit.timer == timer {
			d.clearBulkLocked()
		}
	})
	edit.timer = timer
}

func (d *editorDiskDocument) bulkEdit(b *editorbuffer.Buffer, req editorDiskRequest) (any, error) {
	d.editMu.Lock()
	defer d.editMu.Unlock()
	if req.RequestID == "" || len(req.RequestID) > 128 {
		return nil, errors.New("invalid bulk edit identity")
	}
	digest := sha256.Sum256([]byte("bulk-commit:" + req.RequestID))
	for _, receipt := range d.receipts {
		if receipt.id != req.RequestID {
			continue
		}
		if req.Action == "edit-commit" && receipt.digest == digest {
			return receipt.metadata, nil
		}
		return nil, errors.New("bulk edit identity was already used")
	}
	edit := d.bulk
	if req.Action == "edit-begin" {
		if edit == nil {
			stage, err := b.BeginEdit(req.Revision, req.From, req.To)
			if err != nil {
				return nil, err
			}
			edit = &editorDiskEdit{id: req.RequestID, revision: req.Revision, from: req.From, to: req.To, stage: stage, offset: -1}
			d.bulk = edit
		} else if edit.id != req.RequestID || edit.revision != req.Revision || edit.from != req.From || edit.to != req.To {
			return nil, errors.New("finish or cancel the pending bulk edit")
		}
		d.extendBulkLocked(edit)
		return map[string]any{"received": edit.stage.Received()}, nil
	}
	if req.Action == "edit-abort" {
		if edit != nil && edit.id == req.RequestID {
			d.clearBulkLocked()
		}
		return map[string]any{"ok": true}, nil
	}
	if edit == nil || edit.id != req.RequestID {
		return nil, errors.New("the bulk edit is unavailable")
	}
	if req.Action == "edit-chunk" {
		chunk := sha256.Sum256([]byte(req.Text))
		switch {
		case req.Offset == edit.stage.Received() && req.Text != "":
			if err := edit.stage.Append(req.Text); err != nil {
				return nil, err
			}
			edit.offset, edit.digest = req.Offset, chunk
		case req.Offset >= 0 && req.Offset == edit.offset && chunk == edit.digest:
			// The previous bounded request was applied before its response was lost.
		default:
			return nil, errors.New("bulk edit chunk does not match the pending insertion")
		}
		d.extendBulkLocked(edit)
		return map[string]any{"received": edit.stage.Received()}, nil
	}
	metadata, err := edit.stage.Commit()
	d.clearBulkLocked()
	if err == nil {
		d.rememberMutationLocked(editorDiskReceipt{req.RequestID, digest, metadata})
	}
	return metadata, err
}
