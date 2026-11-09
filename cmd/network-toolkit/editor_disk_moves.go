package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"networktoolkit/internal/editorbuffer"
	"os"
)

const (
	moveBuckets    = 1 << 16
	moveRecordSize = sha256.Size + 24
)

// moveIndex keeps bucket heads in memory and candidates in a temporary file.
// Only an unambiguous pair of exact, multi-line changed runs is a move.
type moveIndex struct {
	file  *os.File
	heads [moveBuckets]int64
	size  int64
}

func (m *moveIndex) add(key [sha256.Size]byte, side int, index int64) error {
	bucket := binary.LittleEndian.Uint16(key[:2])
	var record [moveRecordSize]byte
	for next := m.heads[bucket]; next != 0; {
		offset := next - 1
		if _, err := m.file.ReadAt(record[:], offset); err != nil {
			return err
		}
		if [sha256.Size]byte(record[:sha256.Size]) == key {
			cell := sha256.Size + 8*side
			if binary.LittleEndian.Uint64(record[cell:]) != 0 {
				index = -1
			}
			binary.LittleEndian.PutUint64(record[cell:], uint64(index))
			_, err := m.file.WriteAt(record[cell:cell+8], offset+int64(cell))
			return err
		}
		next = int64(binary.LittleEndian.Uint64(record[sha256.Size+16:]))
	}
	clear(record[:])
	copy(record[:], key[:])
	binary.LittleEndian.PutUint64(record[sha256.Size+8*side:], uint64(index))
	binary.LittleEndian.PutUint64(record[sha256.Size+16:], uint64(m.heads[bucket]))
	if _, err := m.file.WriteAt(record[:], m.size); err != nil {
		return err
	}
	m.heads[bucket] = m.size + 1
	m.size += moveRecordSize
	return nil
}

func (j *diskComparisonJob) findMoves(ctx context.Context, left, right editorbuffer.ComparisonInput, status *diskComparisonStatus) error {
	file, err := os.CreateTemp("", "nt-editor-moves-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	index := moveIndex{file: file}
	digests := [2]*editorbuffer.ComparisonLineDigester{left.LineDigester(ctx), right.LineDigester(ctx)}
	for i := int64(0); i < status.Count; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		diff, err := j.read(i)
		if err != nil {
			return err
		}
		paired := min(diff.LeftCount, diff.RightCount)
		for side, run := range [2][2]int64{{diff.Left, diff.LeftCount}, {diff.Right, diff.RightCount}} {
			if run[1]-paired < 2 {
				continue
			}
			key, hasText, err := digests[side].Digest(run[0]+paired, run[1]-paired)
			if err != nil {
				return err
			}
			if !hasText {
				continue
			}
			if err := index.add(key, side, i+1); err != nil {
				return err
			}
		}
	}
	var record [moveRecordSize]byte
	for offset := int64(0); offset < index.size; offset += moveRecordSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := file.ReadAt(record[:], offset); err != nil {
			return err
		}
		a := int64(binary.LittleEndian.Uint64(record[sha256.Size:]))
		b := int64(binary.LittleEndian.Uint64(record[sha256.Size+8:]))
		if a <= 0 || b <= 0 {
			continue
		}
		removed, err := j.read(a - 1)
		if err != nil {
			return err
		}
		added, err := j.read(b - 1)
		if err != nil {
			return err
		}
		from, to := removed.Left+removed.RightCount, added.Right+added.LeftCount
		count := removed.LeftCount - removed.RightCount
		for _, pair := range [2][2]int64{{a - 1, to + 1}, {b - 1, from + 1}} {
			var move [8]byte
			binary.LittleEndian.PutUint64(move[:], uint64(pair[1]))
			if _, err := j.file.WriteAt(move[:], pair[0]*diskCompareRecordSize+48); err != nil {
				return err
			}
		}
		total := max(status.LeftTotal, status.RightTotal)
		addOverview(&status.LeftOverview, from, count, total, 4)
		addOverview(&status.RightOverview, to, count, total, 4)
	}
	return nil
}
