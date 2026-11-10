//go:build !windows

package sharedfile

import "os"

// A file that is open elsewhere can be replaced and removed on these systems.
func open(path string) (*os.File, error)  { return os.Open(path) }
func replace(source, target string) error { return os.Rename(source, target) }

// Transient reports whether err only says that another program has the file
// at this moment. On these systems an open file is never in the way.
func Transient(error) bool { return false }

// HeldByAnother reports whether err says that another program has the file
// open in a way that excludes this one. That does not happen here.
func HeldByAnother(error) bool { return false }
