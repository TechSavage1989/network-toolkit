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
	s, err := b.Snapshot()
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if s.Revision != req.Revision {
		return nil, editorbuffer.ErrRevision
	}
	staged, err := s.ReplaceAll(ctx, req.Query, req.Replace, req.Regex, req.MatchCase, req.WholeWord)
	if err != nil {
		return nil, err
	}
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
