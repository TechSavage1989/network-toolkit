package editorbuffer

import (
	"errors"
	"unicode/utf8"
)

// EditTransaction builds a single undoable replacement over multiple fragments.
// Callers serialize its methods. The document stays readable while fragments
// arrive and a later revision cannot be replaced by an outdated transaction.
type EditTransaction struct {
	buffer            *Buffer
	revision          uint64
	left, right, text *node
	from, to          int64
	start, end        int64
	contiguous        bool
	closed            bool
}

func (b *Buffer) BeginEdit(revision uint64, from, to int64) (*EditTransaction, error) {
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, ErrClosed
	}
	if b.binary {
		return nil, ErrBinary
	}
	if b.revision != revision {
		return nil, ErrRevision
	}
	if from < 0 || to < from || to > size(b.root).units {
		return nil, ErrBoundary
	}
	left, rest, err := b.split(b.root, from)
	if err != nil {
		return nil, err
	}
	_, right, err := b.split(rest, to-from)
	if err != nil {
		return nil, err
	}
	return &EditTransaction{buffer: b, revision: revision, left: left, right: right, from: from, to: to, start: b.end, end: b.end, contiguous: true}, nil
}

func (t *EditTransaction) Received() int64 { return size(t.text).units }

func validateEditFragment(text string) error {
	if len(text) > MaxEditBytes || !utf8.ValidString(text) {
		return errors.New("invalid or oversized edit fragment")
	}
	for _, r := range text {
		if r == 0 || r == '\r' {
			return errors.New("edit fragments require normalized Unicode text")
		}
	}
	return nil
}

func (t *EditTransaction) Append(text string) error {
	if err := validateEditFragment(text); err != nil {
		return err
	}
	b := t.buffer
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if t.closed || b.closed {
		return ErrClosed
	}
	if t.revision != b.revision {
		return ErrRevision
	}
	t.contiguous = t.contiguous && b.end == t.end
	tree, err := b.append([]byte(text))
	t.end = b.end
	if err != nil {
		return err
	}
	t.text = join(t.text, tree)
	return nil
}

func (t *EditTransaction) Commit() (Metadata, error) {
	b := t.buffer
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if t.closed || b.closed {
		return Metadata{}, ErrClosed
	}
	if t.revision != b.revision {
		return b.metadataLocked(), ErrRevision
	}
	t.closed = true
	if t.from != t.to || t.text != nil {
		b.undo = append(b.undo, b.root)
		b.redo = nil
		b.root = join(join(t.left, t.text), t.right)
		b.revision++
	}
	return b.metadataLocked(), nil
}

func (t *EditTransaction) Abort() {
	b := t.buffer
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	// No live root can point into a still-uncommitted contiguous file suffix.
	// Never truncate a later edit or another transaction's data.
	if !b.closed && t.contiguous && b.end == t.end && t.end > t.start && b.file.Truncate(t.start) == nil {
		b.end = t.start
	}
}
