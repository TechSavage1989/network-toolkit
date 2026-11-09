package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"networktoolkit/internal/editorbuffer"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskEditorCompareMovedRunPreservesAlignment(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "compare"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	const moved = "same moved header\nunique moved detail\n"
	const middle = "unchanged 1\nunchanged 2\nunchanged 3\nunchanged 4\n"
	left := editorbuffer.InlineComparisonInput("first\n" + moved + middle + "last")
	right := editorbuffer.InlineComparisonInput("first\n" + middle + moved + "last")
	job := &diskComparisonJob{file: file}
	job.scan(context.Background(), left, right, editorbuffer.CompareOptions{})
	if !job.status.Done || job.status.Error != "" || job.status.Count != 2 || job.status.Rows != 10 {
		t.Fatalf("expected two differences and ten display rows, got count %d rows %d error %q", job.status.Count, job.status.Rows, job.status.Error)
	}
	removed, added := mustReadMove(t, job, 0), mustReadMove(t, job, 1)
	if removed.Left != 1 || removed.LeftCount != 2 || removed.RightCount != 0 || removed.MovedTo != 6 ||
		added.Right != 5 || added.RightCount != 2 || added.LeftCount != 0 || added.MovedTo != 2 {
		t.Fatalf("unexpected moves: %+v and %+v", removed, added)
	}
	for side := 0; side < 2; side++ {
		line, gapRow := int64(1), int64(7)
		if side == 1 {
			line, gapRow = 5, 1
		}
		marks, err := job.mapLines(line, line+2, side == 1)
		if err != nil || len(marks) != 2 || marks[0] != 4 || marks[1] != 4 {
			t.Fatalf("moved lines missing from map: %v %v", marks, err)
		}
		gap, err := job.atAlignmentRow(gapRow, side == 1)
		if err != nil || gap["gap"] != 1 {
			t.Fatalf("expected gap opposite moved text: %v %v", gap, err)
		}
	}
}

func mustReadMove(t *testing.T, job *diskComparisonJob, index int64) diskComparisonDifference {
	t.Helper()
	value, err := job.read(index)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestMoveIndexPreservesDuplicateAmbiguity(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "moves"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	index := &moveIndex{file: file}
	key := sha256.Sum256([]byte("repeated\nblock\n"))
	for i, side := range []int{0, 1, 0, 0} {
		if err := index.add(key, side, int64(i+1)); err != nil {
			t.Fatal(err)
		}
	}
	var record [moveRecordSize]byte
	if _, err := file.ReadAt(record[:], 0); err != nil {
		t.Fatal(err)
	}
	if index.size != moveRecordSize || int64(binary.LittleEndian.Uint64(record[sha256.Size:])) != -1 ||
		binary.LittleEndian.Uint64(record[sha256.Size+8:]) != 2 {
		t.Fatal("repeated source blocks were treated as unique")
	}
}
