package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

const localToolsProtocolVersion = 1
const localToolsFrameLimit = 8 << 20

type localHelperRequest struct {
	Version int                     `json:"version"`
	Request localToolsRequest       `json:"request"`
	WiFi    *localWiFiHelperRequest `json:"wifi,omitempty"`
}
type localHelperResponse struct {
	Version int               `json:"version"`
	Result  *localApplyResult `json:"result,omitempty"`
	WiFi    *localWiFiResult  `json:"wifi,omitempty"`
}

func validateLocalHelperRequest(r localHelperRequest) error {
	if r.Version != localToolsProtocolVersion {
		return errors.New("Unsupported local configuration helper protocol.")
	}
	if r.WiFi != nil {
		if r.Request.Kind != "" || r.Request.IP != nil || r.Request.Hosts != nil || r.Request.Network != nil {
			return errors.New("The helper accepts exactly one local operation.")
		}
		return validateLocalWiFiHelper(r.WiFi)
	}
	return validateLocalToolsRequest(r.Request)
}

func writeLocalFrame(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	defer clear(data)
	if len(data) == 0 || len(data) > localToolsFrameLimit {
		return errors.New("The local configuration message exceeds its size limit.")
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
		for len(part) > 0 {
			n, e := w.Write(part)
			if e != nil {
				return e
			}
			if n <= 0 || n > len(part) {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	return nil
}

func readLocalFrame(r io.Reader, target any) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size == 0 || size > localToolsFrameLimit {
		return errors.New("The local configuration message has an invalid size.")
	}
	data := make([]byte, size)
	defer clear(data)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("The local configuration message is empty.")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("The local configuration message contains extra data.")
	}
	return nil
}

func validLocalHelperID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
