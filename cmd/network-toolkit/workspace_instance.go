package main

import (
	"bytes"
	"errors"
	"runtime"
)

// workspaceInstanceProtection decides how a workspace's instance record
// (Network_Toolkit.instance.json: loopback endpoint and activation token) is
// stored. On Windows every record is protected with DPAPI, so only the same
// Windows user can recover the endpoint and its restricted activation token,
// even when the selected app-data folder is shared. Elsewhere the record is a
// private 0600 file inside the workspace.
//
// Terminal builds before peer send wrote their record as plain JSON. A plain
// record is still read for window activation, so an older Terminal that is
// already running keeps receiving launches, but this build never writes one
// and peer send never trusts one (readPeerInstance).
type workspaceInstanceProtection struct {
	required  bool
	protect   func([]byte) ([]byte, error)
	unprotect func([]byte) ([]byte, error)
}

var workspaceInstanceGuard = workspaceInstanceProtection{required: runtime.GOOS == "windows", protect: protectCredentialBlob, unprotect: unprotectCredentialBlob}

var errWorkspaceInstanceUnprotected = errors.New("The workspace instance record is not protected.")

// seal returns the bytes to store and clears the plain record when it was
// protected. There is no plain fallback: a record that cannot be protected is
// not written.
func (p workspaceInstanceProtection) seal(plain []byte) ([]byte, error) {
	if !p.required {
		return plain, nil
	}
	if p.protect == nil {
		return nil, errors.New("Workspace instance protection is unavailable.")
	}
	protected, err := p.protect(plain)
	clear(plain)
	if err != nil || len(protected) == 0 || plainWorkspaceInstance(protected) {
		return nil, errors.New("The workspace instance record could not be protected.")
	}
	return protected, nil
}

// open returns the plain record and whether it must be cleared by the caller.
// allowLegacyPlain accepts an unprotected JSON record where protection is
// required; only window activation of the Terminal passes it.
func (p workspaceInstanceProtection) open(stored []byte, allowLegacyPlain bool) (plain []byte, protected bool, err error) {
	if !p.required {
		return stored, false, nil
	}
	if plainWorkspaceInstance(stored) {
		if !allowLegacyPlain {
			return nil, false, errWorkspaceInstanceUnprotected
		}
		return stored, false, nil
	}
	if p.unprotect == nil {
		return nil, false, errors.New("Workspace instance protection is unavailable.")
	}
	plain, err = p.unprotect(stored)
	if err != nil {
		return nil, false, err
	}
	return plain, true, nil
}

// A JSON instance record starts with an object. A DPAPI blob starts with its
// binary provider header and can never be mistaken for one.
func plainWorkspaceInstance(stored []byte) bool {
	trimmed := bytes.TrimLeft(stored, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}
