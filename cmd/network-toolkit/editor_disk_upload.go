package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"networktoolkit/internal/editorbuffer"
)

// Browser File objects have no authorized absolute path. Stage their bounded
// chunks privately, then run the same streaming decoder used for local files.
type editorDiskUpload struct {
	file *os.File
	size int64
}

func (u *editorDiskUpload) close() {
	_ = u.file.Close()
	_ = os.Remove(u.file.Name())
}

func (u *editorDiskUpload) append(offset int64, data []byte) (int64, error) {
	if offset < 0 || len(data) == 0 || len(data) > editorbuffer.MaxEditBytes || offset > u.size {
		return u.size, errors.New("invalid browser file chunk")
	}
	if offset < u.size {
		// Verify retries against the exact bytes already received.
		if int64(len(data)) > u.size-offset {
			return u.size, errors.New("browser file chunk overlaps a different upload")
		}
		previous := make([]byte, len(data))
		if _, err := u.file.ReadAt(previous, offset); err != nil || !bytes.Equal(previous, data) {
			return u.size, errors.New("browser file chunk does not match the earlier upload")
		}
		return u.size, nil
	}
	n, err := u.file.WriteAt(data, offset)
	if err != nil {
		return u.size, err
	}
	if n != len(data) {
		return u.size, io.ErrShortWrite
	}
	u.size += int64(n)
	return u.size, nil
}

func (m *editorViewRegistry) diskUpload(w http.ResponseWriter, g *editorDocumentGrant, disk *editorDiskDocument, req editorDiskRequest) {
	m.mu.Lock()
	disk.mu.Lock()
	if req.Action == "reload-start" {
		err := disk.beginUploadReload(g, req.Revision)
		disk.mu.Unlock()
		m.mu.Unlock()
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"received": 0})
		return
	}
	if disk.mode != "upload" || disk.closed || disk.err != "" {
		disk.mu.Unlock()
		m.mu.Unlock()
		jsonError(w, 409, "This browser file import is unavailable.")
		return
	}
	upload := disk.upload
	if req.Action == "import-chunk" {
		m.mu.Unlock()
		if upload == nil {
			disk.mu.Unlock()
			jsonError(w, 409, "This browser file import has already received its contents.")
			return
		}
		received, err := upload.append(req.Offset, req.Data)
		disk.mu.Unlock()
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"received": received})
		return
	}
	if upload == nil {
		m.mu.Unlock()
		// A lost finish response can be retried while decoding or after it finishes.
		if disk.uploadSize != req.Size {
			disk.mu.Unlock()
			jsonError(w, 409, "The completed upload has a different size.")
			return
		}
		loading := disk.loading
		disk.mu.Unlock()
		jsonResponse(w, 202, map[string]any{"loading": loading})
		return
	}
	if req.Size != upload.size {
		disk.mu.Unlock()
		m.mu.Unlock()
		jsonError(w, 409, "The selected file has not been fully received.")
		return
	}
	reload := disk.reloadUpload
	if reload && (g.busy != 1 || disk.imported.Buffer.Metadata().Revision != disk.reloadRevision) {
		disk.mu.Unlock()
		m.mu.Unlock()
		jsonError(w, 409, "The document changed during the upload. Cancel and reload again.")
		return
	}
	disk.upload = nil
	disk.uploadSize = upload.size
	disk.loading = true
	ctx, cancel := context.WithCancel(disk.ctx)
	disk.uploadCancel = cancel
	disk.mu.Unlock()
	m.mu.Unlock()
	go func() {
		defer cancel()
		defer upload.close()
		source := io.NewSectionReader(upload.file, 0, upload.size)
		result, err := editorbuffer.Import(ctx, source, "", func(n int64) {
			disk.mu.Lock()
			disk.progress = n
			disk.mu.Unlock()
		})
		if reload {
			m.finishUploadReload(g, disk, result, err)
		} else {
			m.finishDiskOpen(g, disk, result, err)
		}
	}()
	jsonResponse(w, 202, map[string]any{"loading": true})
}

// Called with the registry and document locks held. The previous buffer stays
// readable until a complete, valid replacement is available.
func (d *editorDiskDocument) beginUploadReload(g *editorDocumentGrant, revision uint64) error {
	if d.mode != "upload" || d.closed || d.loading || d.imported == nil || filepath.IsAbs(g.snapshot.Path) {
		return errors.New("this file cannot be reloaded from a browser source")
	}
	if d.reloadUpload && d.upload != nil && d.upload.size == 0 && d.reloadRevision == revision {
		return nil
	}
	if d.upload != nil || g.busy != 1 || d.saving || d.imported.Buffer.Metadata().Revision != revision {
		return editorbuffer.ErrRevision
	}
	f, err := os.CreateTemp("", "nt-editor-reload-*")
	if err != nil {
		return err
	}
	d.upload = &editorDiskUpload{file: f}
	d.reloadUpload, d.reloadRevision = true, revision
	d.reloadError, d.progress = "", 0
	return nil
}

func (m *editorViewRegistry) finishUploadReload(g *editorDocumentGrant, d *editorDiskDocument, result *editorbuffer.Imported, err error) {
	m.mu.Lock()
	d.mu.Lock()
	d.reloadUpload, d.uploadCancel = false, nil
	if d.closed || err != nil {
		d.loading = false
		if err != nil {
			d.reloadError = err.Error()
		}
		d.mu.Unlock()
		m.mu.Unlock()
		if result != nil {
			result.Buffer.Close()
		}
		return
	}
	old, uid := d.imported, g.snapshot.UID
	d.imported, d.exported, d.verify = result, nil, nil
	d.editMu.Lock()
	d.clearBulkLocked()
	d.receipts = nil
	d.editMu.Unlock()
	g.snapshot.Hash = result.Hash
	g.snapshot.Encoding, g.snapshot.SavedEncoding = result.Format.Encoding, result.Format.Encoding
	g.snapshot.BOM, g.snapshot.SavedBOM = result.Format.BOM, result.Format.BOM
	g.snapshot.EOL, g.snapshot.SavedEOL = result.Format.EOL, result.Format.EOL
	g.snapshot.EditRevision = result.Buffer.Metadata().Revision
	g.snapshot.Dirty = false
	d.mu.Unlock()
	m.mu.Unlock()
	m.cancelComparisonsUsing(uid)
	d.stopReplaceAll()
	old.Buffer.Close()
	d.mu.Lock()
	d.loading = false
	d.mu.Unlock()
}
