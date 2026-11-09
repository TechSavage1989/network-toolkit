package editorbuffer

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

// FromHistory migrates a bounded renderer checkpoint without changing the
// order of undo/redo operations. Identical text shares its immutable root.
func FromHistory(ctx context.Context, dir string, revision uint64, content, saved string, undo, redo []string) (*Buffer, error) {
	b, err := empty(dir)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			b.Close()
		}
	}()
	roots := map[string]*node{"": nil}
	load := func(text string) (*node, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if root, present := roots[text]; present {
			return root, nil
		}
		if !utf8.ValidString(text) || strings.ContainsAny(text, "\x00\r") {
			return nil, errors.New("editor history contains invalid normalized text")
		}
		root, err := b.append([]byte(text))
		if err != nil {
			return nil, err
		}
		roots[text] = root
		return root, nil
	}
	b.root, err = load(content)
	if err != nil {
		return nil, err
	}
	b.saved, err = load(saved)
	if err != nil {
		return nil, err
	}
	for _, item := range undo {
		root, err := load(item)
		if err != nil {
			return nil, err
		}
		b.undo = append(b.undo, root)
	}
	for _, item := range redo {
		root, err := load(item)
		if err != nil {
			return nil, err
		}
		b.redo = append(b.redo, root)
	}
	b.revision = max(1, revision)
	ok = true
	return b, nil
}
