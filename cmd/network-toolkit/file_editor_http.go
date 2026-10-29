package main

// Text editing is an explicitly authenticated file operation. File contents are
// never stored in application preferences, diagnostic results or transfer logs.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"networktoolkit/internal/model"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const textEditorLimit = 16 << 20 // 16 MiB encoded files; larger/binary files stay downloadable.
// JSON may escape one control byte as six ASCII bytes; metadata remains bounded.
const textEditorRequestLimit = textEditorLimit*6 + 16384

func editorDigest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

// decodeEditableText preserves UTF-8 and BOM-marked UTF-16 endianness. Invalid
// UTF-16 surrogate pairs are rejected to prevent silent file corruption.
func decodeEditableText(b []byte) (string, string, bool, error) {
	if len(b) >= 2 && ((b[0] == 0xff && b[1] == 0xfe) || (b[0] == 0xfe && b[1] == 0xff)) {
		little := b[0] == 0xff
		data := b[2:]
		if len(data)%2 != 0 {
			return "", "", false, errors.New("UTF-16 file has an incomplete code unit")
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			if little {
				units[i] = binary.LittleEndian.Uint16(data[2*i:])
			} else {
				units[i] = binary.BigEndian.Uint16(data[2*i:])
			}
		}
		runes := make([]rune, 0, len(units))
		for i := 0; i < len(units); i++ {
			c := units[i]
			if c == 0 {
				return "", "", false, errors.New("text file contains NUL bytes")
			}
			if c >= 0xd800 && c <= 0xdbff {
				if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return "", "", false, errors.New("invalid UTF-16 surrogate pair")
				}
				runes = append(runes, utf16.DecodeRune(rune(c), rune(units[i+1])))
				i++
				continue
			}
			if c >= 0xdc00 && c <= 0xdfff {
				return "", "", false, errors.New("invalid UTF-16 surrogate pair")
			}
			runes = append(runes, rune(c))
		}
		if little {
			return string(runes), "UTF-16LE", true, nil
		}
		return string(runes), "UTF-16BE", true, nil
	}
	hasBOM := bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf})
	content := b
	if hasBOM {
		content = b[3:]
	}
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return "", "", false, errors.New("file is binary or its text encoding is unsupported; UTF-8 and BOM-marked UTF-16 are supported")
	}
	return string(content), "UTF-8", hasBOM, nil
}
func encodeEditableText(content, encoding string, bom bool) ([]byte, error) {
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return nil, errors.New("text must contain valid Unicode without NUL bytes")
	}
	switch encoding {
	case "", "UTF-8":
		b := []byte(content)
		if bom {
			b = append([]byte{0xef, 0xbb, 0xbf}, b...)
		}
		if len(b) > textEditorLimit {
			return nil, errors.New("text exceeds the editor size limit")
		}
		return b, nil
	case "UTF-16LE", "UTF-16BE":
		units := utf16.Encode([]rune(content))
		n := len(units) * 2
		if bom {
			n += 2
		}
		if n > textEditorLimit {
			return nil, errors.New("text exceeds the editor size limit")
		}
		b := make([]byte, n)
		offset := 0
		if bom {
			if encoding == "UTF-16LE" {
				b[0] = 0xff
				b[1] = 0xfe
			} else {
				b[0] = 0xfe
				b[1] = 0xff
			}
			offset = 2
		}
		for i, c := range units {
			if encoding == "UTF-16LE" {
				binary.LittleEndian.PutUint16(b[offset+2*i:], c)
			} else {
				binary.BigEndian.PutUint16(b[offset+2*i:], c)
			}
		}
		return b, nil
	default:
		return nil, errors.New("Unsupported text encoding")
	}
}
func readTextBytes(b []byte) (map[string]any, error) {
	if len(b) > textEditorLimit {
		return nil, fmt.Errorf("text editor limit is %d MiB; download large files", textEditorLimit>>20)
	}
	content, encoding, bom, e := decodeEditableText(b)
	if e != nil {
		return nil, e
	}
	return map[string]any{"content": content, "hash": editorDigest(b), "bom": bom, "encoding": encoding, "bytes": len(b)}, nil
}

func editableFile(path string) ([]byte, os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, errors.New("text editor requires a regular file")
	}
	if info.Size() > textEditorLimit {
		return nil, 0, errors.New("text file exceeds the 16 MiB editor limit")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, textEditorLimit+1))
	if err != nil {
		return nil, 0, err
	}
	if len(b) > textEditorLimit {
		return nil, 0, errors.New("text file exceeds the editor limit")
	}
	return b, info.Mode().Perm(), nil
}
func readEditableFile(path string) (map[string]any, error) {
	b, _, e := editableFile(path)
	if e != nil {
		return nil, e
	}
	return readTextBytes(b)
}
func saveEditableLocal(path string, content []byte, expectedHash string) (map[string]any, error) {
	if len(content) > textEditorLimit {
		return nil, errors.New("text exceeds the 16 MiB editor limit")
	}
	if _, e := readTextBytes(content); e != nil {
		return nil, e
	}
	old, mode, err := editableFile(path)
	if err == nil {
		if expectedHash == "" || editorDigest(old) != expectedHash {
			return nil, errors.New("file has changed since opening, or the destination exists; reload instead of overwriting")
		}
	} else if os.IsNotExist(err) {
		if expectedHash != "" {
			return nil, errors.New("file was removed after opening; save to a new path")
		}
		mode = 0600
	} else {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".network-toolkit-edit-*")
	if err != nil {
		return nil, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(content)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err = os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "hash": editorDigest(content), "bytes": len(content)}, nil
}
func (a *application) handleLocalTextEditor(w http.ResponseWriter, r *http.Request) {
	var release func()
	var admitted bool
	r, release, admitted = admitEditorMemory(w, r)
	if !admitted {
		return
	}
	defer release()
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var req struct {
		Action       string `json:"action"`
		Path         string `json:"path"`
		Content      string `json:"content"`
		ExpectedHash string `json:"expectedHash"`
		BOM          bool   `json:"bom"`
		Encoding     string `json:"encoding"`
	}
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, textEditorRequestLimit)).Decode(&req); e != nil {
		jsonError(w, 400, "Invalid text editor request")
		return
	}
	filepathAbs, e := absoluteExplorerPath(req.Path)
	if e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	if len(req.ExpectedHash) != 0 && len(req.ExpectedHash) != 64 {
		jsonError(w, 400, "Invalid file revision")
		return
	}
	switch req.Action {
	case "read-text":
		result, e := readEditableFile(filepathAbs)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		jsonResponse(w, 200, result)
	case "write-text":
		contents, encodingError := encodeEditableText(req.Content, req.Encoding, req.BOM)
		if encodingError != nil {
			jsonError(w, 400, encodingError.Error())
			return
		}
		result, e := saveEditableLocal(filepathAbs, contents, req.ExpectedHash)
		if e != nil {
			jsonError(w, 409, e.Error())
			return
		}
		jsonResponse(w, 200, result)
	default:
		jsonError(w, 400, "Unsupported editor action")
	}
}
func (a *application) handleRemoteTextEditor(w http.ResponseWriter, r *http.Request, req remoteBrowserRequest) {
	a.handleRemoteTextEditorWithOperation(w, r, req, a.runRemoteBrowserOperation)
}

// The detached-editor capability supplies one already-authorized, pinned transport.
// The existing hash, encoding, size and staging checks are identical for both paths.
func (a *application) handleRemoteTextEditorWithOperation(w http.ResponseWriter, r *http.Request, req remoteBrowserRequest, run func(context.Context, remoteBrowserRequest) (model.Result, error)) {
	if req.Action == "read-text" && req.Protocol != "ftp" {
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()
		result, err := run(ctx, req)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		data, _ := result.Data.(map[string]any)
		contents, ok := data["contentBytes"].([]byte)
		if !ok {
			jsonError(w, 500, "Unable to read remote file contents")
			return
		}
		text, err := readTextBytes(contents)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		jsonResponse(w, 200, text)
		return
	}
	tmpDir, e := os.MkdirTemp("", "nt-edit-")
	if e != nil {
		jsonError(w, 500, "Unable to prepare the text editor")
		return
	}
	defer os.RemoveAll(tmpDir)
	tempPath := filepath.Join(tmpDir, "content")
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	if req.Action == "read-text" || req.ExpectedHash != "" {
		probe := req
		probe.Action = "stat"
		result, e := run(ctx, probe)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		if info, ok := result.Data.(map[string]any); ok {
			switch n := info["sizeBytes"].(type) {
			case uint64:
				if n > textEditorLimit {
					jsonError(w, 400, "Text file exceeds the 16 MiB editor limit")
					return
				}
			case int64:
				if n > textEditorLimit {
					jsonError(w, 400, "Text file exceeds the 16 MiB editor limit")
					return
				}
			case float64:
				if n > textEditorLimit {
					jsonError(w, 400, "Text file exceeds the 16 MiB editor limit")
					return
				}
			}
		}
		probe.Action = "download"
		probe.LocalPath = tempPath
		probe.Overwrite = false
		if _, e = run(ctx, probe); e != nil {
			jsonError(w, 400, e.Error())
			return
		}
	}
	if req.Action == "read-text" {
		result, e := readEditableFile(tempPath)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		jsonResponse(w, 200, result)
		return
	}
	if req.ExpectedHash != "" {
		old, e := os.ReadFile(tempPath)
		if e != nil {
			jsonError(w, 409, "Remote file changed or was removed")
			return
		}
		if editorDigest(old) != req.ExpectedHash {
			jsonError(w, 409, "Remote file changed after opening. Reload before saving.")
			return
		}
	}
	contents, encodeErr := encodeEditableText(req.Content, req.Encoding, req.BOM)
	if encodeErr != nil {
		jsonError(w, 400, encodeErr.Error())
		return
	}
	if _, e := readTextBytes(contents); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	if e := os.WriteFile(tempPath, contents, 0600); e != nil {
		jsonError(w, 500, "Cannot stage text for upload")
		return
	}
	upload := req
	upload.Action = "upload"
	upload.LocalPath = tempPath
	upload.Overwrite = req.ExpectedHash != ""
	if _, e := run(ctx, upload); e != nil {
		jsonError(w, 409, "Save failed: "+e.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "hash": editorDigest(contents), "bytes": len(contents)})
}

// Browser File objects are streamed via multipart, not serialized into JSON.
// The actual SFTP upload passes through the standard verified connection and
// terminal ownership checks by dispatching to handleRemoteBrowser.
func (a *application) handleRemoteDropUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	if e := r.ParseMultipartForm(4 << 20); e != nil {
		jsonError(w, 413, "Upload failed or exceeds 512 MiB")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, head, e := r.FormFile("file")
	if e != nil {
		jsonError(w, 400, "Choose a file to upload")
		return
	}
	defer file.Close()
	name := head.Filename
	if validateExplorerDropName(name) != nil {
		jsonError(w, 400, "Unsafe uploaded filename")
		return
	}
	var req remoteBrowserRequest
	if e = json.Unmarshal([]byte(r.FormValue("connection")), &req); e != nil {
		jsonError(w, 400, "Invalid connection selection")
		return
	}
	if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
		jsonError(w, 400, "Choose one regular file, not a folder batch")
		return
	}
	if chosen := r.FormValue("name"); chosen != "" {
		if err := validateExplorerDropName(chosen); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		name = chosen
	}
	if req.Protocol == "ftp" {
		jsonError(w, 400, "Windows FTP uploads are unavailable: FTP cannot guarantee atomic no-overwrite publication. Use SFTP or the existing explicit FTP transfer controls.")
		return
	}
	targetDir := req.Path
	if strings.ContainsAny(targetDir, "\\\x00\r\n") {
		jsonError(w, 400, "Unsafe remote folder")
		return
	}
	for _, segment := range strings.Split(targetDir, "/") {
		if segment == ".." {
			jsonError(w, 400, "Parent traversal is not allowed in a drop destination")
			return
		}
	}
	if len(targetDir) > 4096-len(name)-1 {
		jsonError(w, 400, "Remote path is too long")
		return
	}
	req.Path = path.Join(targetDir, name)
	req.Action = "upload"
	req.StagedUpload = true
	tmp, e := os.CreateTemp("", "nt-dropped-*")
	if e != nil {
		jsonError(w, 500, "Cannot stage the uploaded file")
		return
	}
	tempPath := tmp.Name()
	defer os.Remove(tempPath)
	_, e = io.Copy(tmp, io.LimitReader(&explorerContextReader{ctx: r.Context(), reader: file}, (512<<20)+1))
	closeErr := tmp.Close()
	if e != nil || closeErr != nil {
		jsonError(w, 500, "Cannot stage file")
		return
	}
	req.LocalPath = tempPath
	body, e := json.Marshal(req)
	if e != nil {
		jsonError(w, 400, "Invalid upload details")
		return
	}
	forwarded := r.Clone(r.Context())
	forwarded.Method = http.MethodPost
	forwarded.Body = io.NopCloser(bytes.NewReader(body))
	forwarded.ContentLength = int64(len(body))
	if editorRequestView(r) != "" {
		forwarded.URL = r.URL.JoinPath("..")
		forwarded.URL.Path = "/api/remote-browser"
		if !a.serveApprovedEditorExplorer(w, forwarded) {
			jsonError(w, 403, "File explorer access is no longer approved.")
		}
		return
	}
	a.handleRemoteBrowser(w, forwarded)
}

// Copy a remote file through a short-lived private staging directory. Neither
// an arbitrary browser-provided local path nor a terminal credential is used.
func (a *application) handleRemoteCopy(w http.ResponseWriter, r *http.Request, req remoteBrowserRequest) {
	if req.Path == "" || req.NewPath == "" || req.Path == req.NewPath {
		jsonError(w, 400, "Choose distinct source and destination paths")
		return
	}
	if strings.HasSuffix(req.Path, "/") || strings.HasSuffix(req.NewPath, "/") {
		jsonError(w, 400, "Use folder copy for directories")
		return
	}
	tmp, e := os.MkdirTemp("", "nt-copy-")
	if e != nil {
		jsonError(w, 500, "Cannot stage remote file")
		return
	}
	defer os.RemoveAll(tmp)
	stage := filepath.Join(tmp, "copy")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	probe := req
	probe.Action = "stat"
	res, e := a.runRemoteBrowserOperation(ctx, probe)
	if e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	if v, ok := res.Data.(map[string]any); ok {
		if mode, ok := v["mode"].(string); ok && strings.HasPrefix(mode, "04") {
			jsonError(w, 400, "Select a regular file, not a directory")
			return
		}
	}
	download := req
	download.Action = "download"
	download.LocalPath = stage
	download.Overwrite = false
	if _, e = a.runRemoteBrowserOperation(ctx, download); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	upload := req
	upload.Action = "upload"
	upload.LocalPath = stage
	upload.Path = req.NewPath
	upload.Overwrite = false
	if _, e = a.runRemoteBrowserOperation(ctx, upload); e != nil {
		jsonError(w, 400, e.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "message": "Remote file copied"})
}
