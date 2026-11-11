package editorbuffer

import (
	"context"
	"errors"
)

// Replacement is a prepared Replace All: the document of one snapshot with
// every match replaced. Commit makes it the current text as one undo step.
type Replacement struct {
	b        *Buffer
	revision uint64
	root     *node
	eol      *eolMap
	Count    int64
}

// A stretch of the document's own text between two matches that is shorter
// than this is copied next to the replacements; a longer one keeps its pieces.
const replaceInlineBytes = 4096

type leafWalk struct{ stack []*node }

func (w *leafWalk) push(n *node) {
	for ; n != nil; n = n.left {
		w.stack = append(w.stack, n)
	}
}

func (w *leafWalk) next() (piece, bool) {
	if len(w.stack) == 0 {
		return piece{}, false
	}
	n := w.stack[len(w.stack)-1]
	w.stack = w.stack[:len(w.stack)-1]
	w.push(n.right)
	return n.part, true
}

// ReplaceAll replaces each match on its own instead of replacing the whole
// document with the result. In a document whose lines keep their own endings
// (eol.go) that is what keeps a line without a match the way it was read:
//   - a long stretch between two matches keeps its pieces, so it still is the
//     text that was read;
//   - a short stretch is copied together with the replacements around it, and
//     every line break in the copy is recorded with the ending it had. That
//     bounds the number of pieces when a word on every line is replaced.
//
// A line break inside a replacement is new text. The work is one pass over
// the document; nothing changes until Commit.
func (s *Snapshot) ReplaceAll(ctx context.Context, query, replacement string, regex, matchCase, wholeWord bool) (*Replacement, error) {
	b := s.b
	if b.binary {
		return nil, ErrBinary
	}
	if err := validateEditFragment(replacement); err != nil {
		return nil, err
	}
	scan, err := s.matchScan(ctx, query, regex, matchCase, wholeWord)
	if err != nil {
		return nil, err
	}
	result := &Replacement{b: b, revision: s.Revision, eol: s.eol}
	var endings *eolCursor
	if s.eol != nil {
		endings = &eolCursor{m: s.eol, file: b.file}
	}
	// run is new text waiting to be stored: replacements and the short
	// stretches between them. runKinds holds the ending of each of its line
	// breaks. Runs stored one after the other share one record.
	var run, runKinds []byte
	var segmentKinds *eolKinds
	var segmentStarts, segmentRanks []int64
	segmentStart, segmentEnd := int64(-1), int64(-1)
	closeSegment := func() {
		if segmentKinds != nil && segmentKinds.count > 0 {
			result.eol = result.eol.with(newSegment(segmentStart, segmentEnd, segmentKinds, segmentStarts, segmentRanks))
		}
		segmentKinds, segmentStarts, segmentRanks = nil, nil, nil
	}
	flush := func() error {
		if len(run) == 0 {
			return nil
		}
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return ErrClosed
		}
		start := b.end
		tree, err := b.append(run)
		end := b.end
		b.mu.Unlock()
		if err != nil {
			return err
		}
		result.root = join(result.root, tree)
		if endings != nil {
			if segmentKinds == nil || segmentEnd != start {
				closeSegment()
				segmentKinds, segmentStart = &eolKinds{}, start
			}
			segmentStarts, segmentRanks = append(segmentStarts, start), append(segmentRanks, segmentKinds.count)
			for _, kind := range runKinds {
				segmentKinds.add(kind)
			}
			segmentEnd = end
		}
		run, runKinds = run[:0], runKinds[:0]
		return nil
	}
	walk := &leafWalk{}
	walk.push(s.root)
	var leaf piece
	var data []byte
	leafStart, pos, started := int64(0), int64(0), false
	// keep carries the document's own text from pos up to a byte position.
	keep := func(target int64) error {
		inline := target-pos < replaceInlineBytes
		if !inline {
			if err := flush(); err != nil {
				return err
			}
		}
		for pos < target {
			for !started || leafStart+leaf.bytes <= pos {
				if started {
					leafStart += leaf.bytes
				}
				next, more := walk.next()
				if !more {
					return errors.New("search produced a replacement outside the document")
				}
				leaf, data, started = next, nil, true
			}
			from, to := pos-leafStart, min(leaf.bytes, target-leafStart)
			whole := from == 0 && to == leaf.bytes
			if (inline || !whole) && data == nil {
				if data, err = b.bytes(leaf); err != nil {
					return err
				}
			}
			switch {
			case inline:
				run = append(run, data[from:to]...)
				if endings != nil && leaf.lines > 0 {
					recorded, line, err := endings.locate(leaf.offset + from)
					if err != nil {
						return err
					}
					for _, c := range data[from:to] {
						if c != '\n' {
							continue
						}
						kind := s.eol.base
						if recorded != nil {
							kind = recorded.kind(line)
							line++
						}
						runKinds = append(runKinds, kind)
					}
				}
			case whole:
				result.root = join(result.root, branch(nil, leaf, nil))
			default:
				result.root = join(result.root, branch(nil, piece{leaf.offset + from, measureText(data[from:to])}, nil))
			}
			pos = leafStart + to
		}
		return nil
	}
	total := size(s.root).bytes
	for at := int64(0); at <= total; {
		from, to, found, err := scan.next(at)
		if err != nil {
			return nil, err
		}
		if !found {
			break
		}
		if from < pos {
			return nil, errors.New("search produced an invalid overlapping replacement")
		}
		if err = keep(from); err != nil {
			return nil, err
		}
		run = append(run, replacement...)
		if endings != nil {
			for i := 0; i < len(replacement); i++ {
				if replacement[i] == '\n' {
					runKinds = append(runKinds, s.eol.base)
				}
			}
		}
		if len(run) >= ChunkBytes {
			if err = flush(); err != nil {
				return nil, err
			}
		}
		pos = to
		result.Count++
		if at, err = scan.after(from, to); err != nil {
			return nil, err
		}
	}
	if result.Count == 0 {
		return result, nil
	}
	if err = keep(total); err != nil {
		return nil, err
	}
	if err = flush(); err != nil {
		return nil, err
	}
	closeSegment()
	return result, ctx.Err()
}

// Commit applies the replacement if the document is still at the revision it
// was prepared from.
func (r *Replacement) Commit() (Metadata, error) {
	b := r.b
	b.lifetime.RLock()
	defer b.lifetime.RUnlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return Metadata{}, ErrClosed
	}
	if b.binary {
		return b.metadataLocked(), ErrBinary
	}
	if r.revision != b.revision {
		return b.metadataLocked(), ErrRevision
	}
	if r.Count > 0 {
		b.undo = append(b.undo, b.root)
		b.redo = nil
		b.root = r.root
		b.eol = r.eol
		b.revision++
	}
	return b.metadataLocked(), nil
}
