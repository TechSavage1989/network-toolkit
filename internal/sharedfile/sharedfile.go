// Package sharedfile opens and replaces files that several running apps of
// one data folder read and rewrite at the same time (the settings file, the
// credential vault, SSH host trust, the macro library).
//
// Every such file is written as a new file that then takes the place of the
// old one in a single step, so a reader sees the old file or the new one,
// never half of either. Two things can still go wrong for a moment, and both
// are waited out here instead of being reported:
//
//   - On Windows a file cannot be replaced while another program has it open,
//     unless that program opened it allowing deletion. Open does so for
//     reading, and Replace tries again when a reader that did not (a virus
//     scanner, a backup program, an older version of this app) is in the way.
//   - A reader that looks at a file and then opens it can find a different
//     file, because the replace happened in between. Callers that compare the
//     two (to refuse links) ask again with Retry.
package sharedfile

import (
	"os"
	"time"
)

// How long a read or a replace waits for another program that has the file
// for a moment. Reads of a file take well under a millisecond.
const patience = 1500 * time.Millisecond

// Retry calls attempt until it succeeds, fails with an error that again does
// not want repeated, or patience runs out. It returns attempt's last error.
func Retry(attempt func() error, again func(error) bool) error {
	deadline := time.Now().Add(patience)
	pause := time.Millisecond
	for {
		err := attempt()
		if err == nil || !again(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(pause)
		if pause < 32*time.Millisecond {
			pause *= 2
		}
	}
}

// Open opens a file for reading without keeping other programs from
// replacing or removing it meanwhile.
func Open(path string) (*os.File, error) {
	var file *os.File
	err := Retry(func() error {
		var err error
		file, err = open(path)
		return err
	}, Transient)
	return file, err
}

// Replace puts source in the place of target in one step. When another
// program holds target in a way that forbids this, it waits for a moment.
func Replace(source, target string) error {
	return Retry(func() error { return replace(source, target) }, Transient)
}
