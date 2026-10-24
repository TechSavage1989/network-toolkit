package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type hostsDocument struct {
	exists   bool
	bytes    []byte
	text     string
	encoding string
	newline  string
}

func decodeHostsDocument(data []byte, exists bool, legacyDecode func([]byte) (string, string, error)) (hostsDocument, error) {
	d := hostsDocument{exists: exists, bytes: append([]byte(nil), data...), encoding: "UTF-8", newline: "CRLF"}
	switch {
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		d.encoding = "UTF-8 BOM"
		d.text = string(data[3:])
		if !utf8.ValidString(d.text) {
			return d, errors.New("The hosts file contains invalid UTF-8 text.")
		}
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		if len(data)%2 != 0 {
			return d, errors.New("The hosts file has an incomplete UTF-16 character.")
		}
		order := binary.LittleEndian
		d.encoding = "UTF-16 LE"
		units := make([]uint16, (len(data)-2)/2)
		for i := range units {
			if data[0] == 0xfe {
				d.encoding = "UTF-16 BE"
				units[i] = binary.BigEndian.Uint16(data[2+i*2:])
			} else {
				units[i] = order.Uint16(data[2+i*2:])
			}
		}
		for i := 0; i < len(units); i++ {
			if units[i] >= 0xd800 && units[i] <= 0xdbff {
				if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return d, errors.New("The hosts file contains an invalid UTF-16 surrogate.")
				}
				i++
			} else if units[i] >= 0xdc00 && units[i] <= 0xdfff {
				return d, errors.New("The hosts file contains an invalid UTF-16 surrogate.")
			}
		}
		d.text = string(utf16.Decode(units))
	case utf8.Valid(data):
		d.text = string(data)
	default:
		if legacyDecode == nil {
			return d, errors.New("The hosts file uses an unsupported text encoding.")
		}
		var err error
		d.text, d.encoding, err = legacyDecode(data)
		if err != nil {
			return d, err
		}
	}
	if strings.ContainsRune(d.text, 0) {
		return d, errors.New("The hosts file contains NUL characters and cannot be safely edited as text.")
	}
	crlf := strings.Count(d.text, "\r\n")
	rest := strings.ReplaceAll(d.text, "\r\n", "")
	lf := strings.Count(rest, "\n")
	cr := strings.Count(rest, "\r")
	switch {
	case crlf > 0 && lf == 0 && cr == 0:
		d.newline = "CRLF"
	case lf > 0 && crlf == 0 && cr == 0:
		d.newline = "LF"
	case cr > 0 && crlf == 0 && lf == 0:
		d.newline = "CR"
	case crlf+lf+cr > 0:
		d.newline = "Mixed"
	}
	return d, nil
}

// Preserve byte-for-byte output when text is unchanged. For edited text, line
// terminators on unchanged lines survive; new/edited lines use the file's most
// common original terminator. The submitted final-newline choice is retained.
func restoreHostsNewlines(original, edited string) string {
	edited = normalizeHostsNewlines(edited)
	if normalizeHostsNewlines(original) == edited {
		return original
	}
	type line struct{ text, end string }
	lines := []line{}
	start := 0
	for i := 0; i < len(original); i++ {
		if original[i] == '\r' || original[i] == '\n' {
			end := original[i : i+1]
			j := i + 1
			if original[i] == '\r' && j < len(original) && original[j] == '\n' {
				j++
				end = "\r\n"
			}
			lines = append(lines, line{original[start:i], end})
			start = j
			i = j - 1
		}
	}
	if start < len(original) {
		lines = append(lines, line{original[start:], ""})
	}
	endings := map[string][]string{}
	counts := map[string]int{}
	preferred := "\r\n"
	for _, l := range lines {
		endings[l.text] = append(endings[l.text], l.end)
		if l.end != "" {
			counts[l.end]++
			if counts[l.end] > counts[preferred] {
				preferred = l.end
			}
		}
	}
	var out strings.Builder
	parts := strings.Split(edited, "\n")
	for i, text := range parts {
		out.WriteString(text)
		if i == len(parts)-1 {
			break
		}
		end := preferred
		if e := endings[text]; len(e) > 0 {
			if e[0] != "" {
				end = e[0]
			}
			endings[text] = e[1:]
		}
		out.WriteString(end)
	}
	return out.String()
}
func encodeHostsDocument(d hostsDocument, content string, legacyEncode func(string, string) ([]byte, error)) ([]byte, error) {
	if _, err := validateHostsText(content); err != nil {
		return nil, err
	}
	content = restoreHostsNewlines(d.text, content)
	if content == d.text {
		return append([]byte(nil), d.bytes...), nil
	}
	switch d.encoding {
	case "UTF-8":
		return []byte(content), nil
	case "UTF-8 BOM":
		return append([]byte{0xef, 0xbb, 0xbf}, []byte(content)...), nil
	case "UTF-16 LE", "UTF-16 BE":
		units := utf16.Encode([]rune(content))
		out := make([]byte, 2+len(units)*2)
		out[0], out[1] = 0xff, 0xfe
		if d.encoding == "UTF-16 BE" {
			out[0], out[1] = 0xfe, 0xff
		}
		for i, u := range units {
			if d.encoding == "UTF-16 BE" {
				binary.BigEndian.PutUint16(out[2+i*2:], u)
			} else {
				binary.LittleEndian.PutUint16(out[2+i*2:], u)
			}
		}
		return out, nil
	default:
		if legacyEncode == nil {
			return nil, errors.New("This file's original encoding cannot be written safely.")
		}
		return legacyEncode(content, d.encoding)
	}
}

func readHostsDocument(path string, legacyDecode func([]byte) (string, string, error)) (hostsDocument, error) {
	if err := rejectLocalPathLinks(path); err != nil {
		return hostsDocument{}, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return decodeHostsDocument(nil, false, legacyDecode)
	}
	if err != nil {
		return hostsDocument{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return hostsDocument{}, err
	}
	if !info.Mode().IsRegular() {
		return hostsDocument{}, errors.New("The hosts path is not a regular file.")
	}
	if info.Size() > localHostsLimit {
		return hostsDocument{}, errors.New("The hosts file is larger than the 1 MiB editing limit.")
	}
	data, err := io.ReadAll(io.LimitReader(file, localHostsLimit+1))
	if err != nil {
		return hostsDocument{}, err
	}
	if len(data) > localHostsLimit {
		return hostsDocument{}, errors.New("The hosts file grew beyond the editing limit. Reload it.")
	}
	return decodeHostsDocument(data, true, legacyDecode)
}

func rejectLocalPathLinks(path string) error {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return errors.New("The system hosts path must be absolute.")
	}
	current := path
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || localPathIsReparse(info) {
				return errors.New("The hosts path contains a symbolic link or Windows reparse point; it will not be changed.")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return nil
}

type localHostsFileOps struct {
	decode  func([]byte) (string, string, error)
	encode  func(string, string) ([]byte, error)
	replace func(path, temporary, backup string) error
	create  func(path, temporary string) error
}

func saveHostsFile(ctx context.Context, path string, r localHostsRequest, ops localHostsFileOps) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := validateHostsText(r.Content); err != nil {
		return "", err
	}
	d, err := readHostsDocument(path, ops.decode)
	if err != nil {
		return "", err
	}
	if r.Revision != localHostsRevision(d.exists, d.bytes) {
		return "", errLocalConfigConflict
	}
	data, err := encodeHostsDocument(d, r.Content, ops.encode)
	if err != nil {
		return "", err
	}
	if len(data) > localHostsLimit {
		return "", errors.New("The encoded hosts file exceeds the 1 MiB limit.")
	}
	if bytes.Equal(data, d.bytes) && d.exists {
		return "", nil
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".NetworkToolkit-hosts-*.tmp")
	if err != nil {
		return "", err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	// Re-read immediately before replacement, including after any elevation wait.
	latest, err := readHostsDocument(path, ops.decode)
	if err != nil {
		return "", err
	}
	if localHostsRevision(latest.exists, latest.bytes) != r.Revision {
		return "", errLocalConfigConflict
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !d.exists {
		if ops.create == nil {
			return "", errors.New("Safe hosts-file creation is unavailable.")
		}
		// Publish only the complete flushed temporary file and never overwrite
		// a hosts file created by another process since our final revision read.
		if err = ops.create(path, temporary); errors.Is(err, os.ErrExist) {
			return "", errLocalConfigConflict
		}
		return "", err
	}
	backup := path + ".NetworkToolkit-" + randomHex(12) + ".bak"
	if _, err = os.Lstat(backup); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("Unable to reserve a unique hosts backup path.")
	}
	if ops.replace == nil {
		return "", errors.New("Safe hosts-file replacement is unavailable.")
	}
	if err = ops.replace(path, temporary, backup); err != nil {
		return backup, fmt.Errorf("Hosts replacement failed; inspect the original and backup before retrying: %w", err)
	}
	return backup, nil
}
