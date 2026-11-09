package editorbuffer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"hash"
	"io"
)

// ComparisonLineDigester fingerprints non-overlapping source ranges in order
// with one reusable read buffer. Exact line endings are normalized at EOF.
type ComparisonLineDigester struct {
	ctx    context.Context
	reader *bufio.Reader
	hash   hash.Hash
	line   int64
	total  int64
}

func (in ComparisonInput) LineDigester(ctx context.Context) *ComparisonLineDigester {
	return &ComparisonLineDigester{
		ctx: ctx, reader: bufio.NewReaderSize(in.reader(ctx, false), ChunkBytes),
		hash: sha256.New(), total: in.Lines(),
	}
}

func (d *ComparisonLineDigester) Digest(start, count int64) ([sha256.Size]byte, bool, error) {
	var result [sha256.Size]byte
	if start < d.line || start > d.total || count <= 0 || count > d.total-start {
		return result, false, ErrBoundary
	}
	d.hash.Reset()
	hasText := false
	for end := start + count; d.line < end; {
		if err := d.ctx.Err(); err != nil {
			return result, false, err
		}
		part, err := d.reader.ReadSlice('\n')
		if d.line >= start {
			d.hash.Write(part)
			hasText = hasText || len(bytes.TrimSpace(part)) != 0
		}
		switch err {
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			if d.line >= start {
				d.hash.Write([]byte{'\n'})
			}
		case nil:
		default:
			return result, false, err
		}
		d.line++
	}
	d.hash.Sum(result[:0])
	return result, hasText, nil
}
