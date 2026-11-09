package main

import (
	"context"
	"errors"
	"io"
	"os"
	"unicode/utf8"

	"networktoolkit/internal/editorbuffer"
)

func (d *editorDiskDocument) replaceDiskMatches(ctx context.Context, b *editorbuffer.Buffer, req editorDiskRequest) (any, error) {
	s, err := b.Snapshot()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if s.Revision != req.Revision {
		return nil, editorbuffer.ErrRevision
	}
	f, err := os.CreateTemp("", "nt-editor-replace-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	count, err := s.WriteReplaced(ctx, f, req.Query, req.Replace, req.Regex, req.MatchCase, req.WholeWord)
	s.Close()
	if err != nil {
		return nil, err
	}
	meta := b.Metadata()
	if meta.Revision != req.Revision {
		return nil, editorbuffer.ErrRevision
	}
	if count == 0 {
		return map[string]any{"metadata": meta, "count": count}, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	stage, err := b.BeginEdit(req.Revision, 0, meta.Units)
	if err != nil {
		return nil, err
	}
	defer stage.Abort()
	data := make([]byte, editorbuffer.ChunkBytes)
	pending := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, readErr := io.ReadFull(f, data[pending:])
		n += pending
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return nil, readErr
		}
		limit := n
		if readErr == nil {
			start := n - 1
			for !utf8.RuneStart(data[start]) {
				start--
			}
			if !utf8.FullRune(data[start:n]) {
				limit = start
			}
		}
		if err := stage.Append(string(data[:limit])); err != nil {
			return nil, err
		}
		if readErr != nil {
			break
		}
		pending = copy(data, data[limit:n])
	}
	d.editMu.Lock()
	defer d.editMu.Unlock()
	if d.bulk != nil {
		return nil, errors.New("finish or cancel the pending bulk edit")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	meta, err = stage.Commit()
	return map[string]any{"metadata": meta, "count": count}, err
}
