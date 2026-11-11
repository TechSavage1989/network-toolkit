package main

import (
	"context"
	"errors"

	"networktoolkit/internal/editorbuffer"
)

// Replace All replaces each match on its own, as one undo step. Everything
// between the matches stays the text that was read, so a file whose lines end
// in different ways keeps the ending of every line without a match.
func (d *editorDiskDocument) replaceDiskMatches(ctx context.Context, b *editorbuffer.Buffer, req editorDiskRequest) (any, error) {
	// A reload or a close of the document stops the search (stopReplaceAll).
	search := &replaceAllContext{Context: ctx}
	ctx = search
	d.mu.Lock()
	if d.replacing == nil {
		d.replacing = map[*replaceAllContext]struct{}{}
	}
	d.replacing[search] = struct{}{}
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.replacing, search)
		d.mu.Unlock()
	}()
	s, err := b.Snapshot()
	if err != nil {
		return nil, err
	}
	if s.Revision != req.Revision {
		s.Close()
		return nil, editorbuffer.ErrRevision
	}
	staged, err := s.ReplaceAll(ctx, req.Query, req.Replace, req.Regex, req.MatchCase, req.WholeWord)
	// The snapshot holds the buffer open. It is released here, before this
	// request waits for anything else: a Close of the buffer (a reload, the
	// tab closing) may be waiting for it.
	s.Close()
	if err != nil {
		return nil, err
	}
	// Given up unless the commit below succeeds: its text leaves the scratch file.
	defer staged.Abort()
	d.editMu.Lock()
	defer d.editMu.Unlock()
	if d.bulk != nil {
		return nil, errors.New("finish or cancel the pending bulk edit")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	meta, err := staged.Commit()
	return map[string]any{"metadata": meta, "count": staged.Count}, err
}
