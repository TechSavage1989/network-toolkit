package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"networktoolkit/internal/terminal"
	"strings"
)

const terminalShellFrameLimit = 64 << 10

// This private protocol is a single explicitly approved interactive shell, not
// a privileged HTTP endpoint or a generic executable/argument launcher.
type terminalShellFrame struct {
	Version int                         `json:"version"`
	Kind    string                      `json:"kind"`
	Options *terminal.LocalShellOptions `json:"options,omitempty"`
	State   *terminal.LocalShellState   `json:"state,omitempty"`
	Data    []byte                      `json:"data,omitempty"`
	Cols    int                         `json:"cols,omitempty"`
	Rows    int                         `json:"rows,omitempty"`
	Error   string                      `json:"error,omitempty"`
}

func validateTerminalShellStart(f terminalShellFrame) error {
	if f.Version != 1 || f.Kind != "start" || f.Options == nil || f.State != nil || len(f.Data) > 0 || f.Cols != 0 || f.Rows != 0 || f.Error != "" {
		return errors.New("invalid private terminal startup request")
	}
	o := f.Options
	if o.Type == "cmd" && strings.HasPrefix(o.WorkingDirectory, `\\`) {
		return errors.New("Command Prompt requires a local drive starting directory, not UNC")
	}
	if o.Type != "cmd" && o.Type != "powershell" {
		return errors.New("unsupported administrator shell")
	}
	if o.Cols < 2 || o.Cols > 500 || o.Rows < 2 || o.Rows > 300 || len(o.WorkingDirectory) > 32760 {
		return errors.New("invalid administrator shell dimensions or directory")
	}
	return nil
}
func validateTerminalShellControl(f terminalShellFrame) error {
	if f.Version != 1 || f.Options != nil || f.State != nil || f.Error != "" {
		return errors.New("invalid private terminal control")
	}
	switch f.Kind {
	case "input":
		if len(f.Data) == 0 || len(f.Data) > 16<<10 || f.Cols != 0 || f.Rows != 0 {
			return errors.New("invalid private terminal input")
		}
	case "resize":
		if len(f.Data) != 0 || f.Cols < 2 || f.Cols > 500 || f.Rows < 2 || f.Rows > 300 {
			return errors.New("invalid private terminal size")
		}
	case "close":
		if len(f.Data) != 0 || f.Cols != 0 || f.Rows != 0 {
			return errors.New("invalid private terminal close")
		}
	default:
		return errors.New("unsupported private terminal control")
	}
	return nil
}
func writeTerminalShellFrame(w io.Writer, f terminalShellFrame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	defer clear(data)
	if len(data) == 0 || len(data) > terminalShellFrameLimit {
		return errors.New("private terminal frame too large")
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
		for len(part) > 0 {
			n, e := w.Write(part)
			if e != nil {
				return e
			}
			if n < 1 || n > len(part) {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}
func readTerminalShellFrame(r io.Reader) (f terminalShellFrame, err error) {
	var header [4]byte
	if _, err = io.ReadFull(r, header[:]); err != nil {
		return
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size == 0 || size > terminalShellFrameLimit {
		return f, errors.New("invalid private terminal frame size")
	}
	data := make([]byte, size)
	defer clear(data)
	if _, err = io.ReadFull(r, data); err != nil {
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&f); err != nil {
		return
	}
	var extra any
	if e := decoder.Decode(&extra); !errors.Is(e, io.EOF) {
		return f, errors.New("extra private terminal frame data")
	}
	return
}

func terminalShellPeerMatches(expected, actual uint32) bool {
	return expected != 0 && actual == expected
}
func validateTerminalShellReady(f terminalShellFrame, directory string) (terminal.LocalShellState, error) {
	if f.Version != 1 || f.Kind != "ready" || f.State == nil || !f.State.Elevated || f.State.WorkingDirectory != directory || f.Options != nil || len(f.Data) != 0 || f.Error != "" || f.Cols != 0 || f.Rows != 0 {
		return terminal.LocalShellState{}, errors.New("administrator terminal did not confirm its process state")
	}
	return *f.State, nil
}
