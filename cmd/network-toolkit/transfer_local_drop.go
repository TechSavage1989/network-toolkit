package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// Local Explorer imports carry browser-granted bytes. The destination is an
// authenticated user's explicit selection, not an untrusted source pathname.
func (a *application) handleLocalExplorerDrop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		a.handleExplorerImportAction(w, r)
		return
	}
	owner := editorRequestView(r)
	var approved *editorExplorerAccess
	if owner != "" && a.editorViews != nil {
		a.editorViews.mu.Lock()
		if v := a.editorViews.views[owner]; v != nil {
			approved = v.explorer
		}
		a.editorViews.mu.Unlock()
	}
	valid := func() bool {
		if !a.explorerCallerValid(owner) {
			return false
		}
		if owner != "" {
			a.editorViews.mu.Lock()
			defer a.editorViews.mu.Unlock()
			v := a.editorViews.views[owner]
			return approved != nil && v != nil && v.explorer == approved
		}
		return true
	}
	if !valid() {
		jsonError(w, 403, "File explorer approval unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExplorerDropBytes+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		jsonError(w, 400, "Expected a selected file and destination")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "destination" || part.FileName() != "" {
		jsonError(w, 400, "Missing destination")
		return
	}
	var destination struct {
		Path      string `json:"path"`
		ID        string `json:"id,omitempty"`
		Name      string `json:"name"`
		Overwrite bool   `json:"overwrite"`
	}
	decoder := json.NewDecoder(io.LimitReader(part, 16<<10))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&destination)
	part.Close()
	if err != nil || (destination.ID == "" && !filepath.IsAbs(destination.Path)) || len(destination.Path) > 4096 {
		jsonError(w, 400, "Choose an absolute destination folder")
		return
	}
	part, err = reader.NextPart()
	if err != nil || part.FormName() != "file" || validateExplorerDropName(part.FileName()) != nil {
		jsonError(w, 400, "Choose one regular file")
		return
	}
	defer part.Close()

	if destination.ID != "" {
		job, e := a.importSession(editorRequestView(r), destination.ID)
		if e != nil {
			jsonError(w, 404, e.Error())
			return
		}
		job.mu.Lock()
		defer job.mu.Unlock()
		relative, e := importRelative(destination.Path)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		if e = job.verify(relative); e != nil {
			jsonError(w, 409, e.Error())
			return
		}
		release, e := a.reserveExplorerMutation(editorExplorerTarget{Binding: editorBinding{Kind: "local"}, Path: filepath.Join(job.path, relative, destination.Name)})
		if e != nil {
			jsonError(w, 409, e.Error())
			return
		}
		defer release()
		if destination.Overwrite {
			jsonError(w, 400, "Recursive folder copies do not overwrite or merge existing folders")
			return
		}
		if len(job.dirs)+job.files >= 1500 || job.bytes >= explorerExportMaxBytes {
			jsonError(w, 400, "Folder copy reached its resource limit")
			return
		}
		remaining := explorerExportMaxBytes - job.bytes
		bounded := &io.LimitedReader{R: &explorerSinglePartReader{part: part, reader: reader}, N: remaining + 1}
		total, e := writeExplorerDropRoot(r.Context(), job.root, job.path, job.info, relative, destination.Name, false, bounded, func() (func(), error) {
			if bounded.N == 0 {
				return nil, fmt.Errorf("Folder copy exceeds2GiB")
			}
			if err := job.verify(relative); err != nil {
				return nil, err
			}
			return func() {}, nil
		})
		if e != nil {
			jsonError(w, 400, "Copy did not complete: "+e.Error())
			return
		}
		job.bytes += total
		job.files++
		jsonResponse(w, 200, map[string]any{"ok": true, "bytes": total})
		return
	}
	release, err := a.reserveExplorerMutation(editorExplorerTarget{Binding: editorBinding{Kind: "local"}, Path: filepath.Join(destination.Path, destination.Name)})
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	defer release()
	total, err := writeExplorerDrop(r.Context(), destination.Path, ".", destination.Name, destination.Overwrite, &explorerSinglePartReader{part: part, reader: reader}, func() (func(), error) {
		if !valid() {
			return nil, fmt.Errorf("file explorer approval changed before publication")
		}
		return func() {}, nil
	})
	if err != nil {
		jsonError(w, 400, "Copy did not complete: "+err.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "bytes": total, "name": destination.Name, "message": "File copied; original unchanged"})
}
