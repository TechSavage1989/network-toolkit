package main

// SFTP v3 file subsystem over authenticated SSH. No shell or command execution.
// Each SSH/SFTP session is confined to one explicitly selected server root.
import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	sftpInit        byte   = 1
	sftpVersion     byte   = 2
	sftpOpen        byte   = 3
	sftpClose       byte   = 4
	sftpRead        byte   = 5
	sftpWrite       byte   = 6
	sftpLstat       byte   = 7
	sftpFstat       byte   = 8
	sftpSetstat     byte   = 9
	sftpFsetstat    byte   = 10
	sftpOpendir     byte   = 11
	sftpReaddir     byte   = 12
	sftpRemove      byte   = 13
	sftpMkdir       byte   = 14
	sftpRmdir       byte   = 15
	sftpRealpath    byte   = 16
	sftpStat        byte   = 17
	sftpRename      byte   = 18
	sftpReadlink    byte   = 19
	sftpSymlink     byte   = 20
	sftpStatus      byte   = 101
	sftpHandle      byte   = 102
	sftpData        byte   = 103
	sftpName        byte   = 104
	sftpAttrs       byte   = 105
	sftpOK          uint32 = 0
	sftpEOF         uint32 = 1
	sftpNoFile      uint32 = 2
	sftpDenied      uint32 = 3
	sftpFailure     uint32 = 4
	sftpBadMessage  uint32 = 5
	sftpUnsupported uint32 = 8
	sftpFlagSize    uint32 = 1
	sftpFlagUID     uint32 = 2
	sftpFlagPerm    uint32 = 4
	sftpFlagTimes   uint32 = 8
	sftpPRead       uint32 = 1
	sftpPWrite      uint32 = 2
	sftpPAppend     uint32 = 4
	sftpPCreate     uint32 = 8
	sftpPTrunc      uint32 = 16
	sftpPExcl       uint32 = 32
)

type sftpHandleState struct {
	file      *os.File
	entries   []os.FileInfo
	at        int
	path      string
	operation string
	bytes     uint64
}
type sftpSession struct {
	channel   ssh.Channel
	root      string
	access    string
	overwrite bool
	peer      string
	server    *transferHost
	handles   map[string]*sftpHandleState
}

func sftp32(n uint32) []byte   { var v [4]byte; binary.BigEndian.PutUint32(v[:], n); return v[:] }
func sftp64(n uint64) []byte   { var v [8]byte; binary.BigEndian.PutUint64(v[:], n); return v[:] }
func sftpBlob(b []byte) []byte { return append(sftp32(uint32(len(b))), b...) }
func sftpStr(s string) []byte  { return sftpBlob([]byte(s)) }
func sftpRead32(b *[]byte) (uint32, error) {
	if len(*b) < 4 {
		return 0, io.ErrUnexpectedEOF
	}
	n := binary.BigEndian.Uint32((*b)[:4])
	*b = (*b)[4:]
	return n, nil
}
func sftpRead64(b *[]byte) (uint64, error) {
	if len(*b) < 8 {
		return 0, io.ErrUnexpectedEOF
	}
	n := binary.BigEndian.Uint64((*b)[:8])
	*b = (*b)[8:]
	return n, nil
}
func sftpReadBlob(b *[]byte) ([]byte, error) {
	n, e := sftpRead32(b)
	if e != nil {
		return nil, e
	}
	if int(n) > len(*b) {
		return nil, io.ErrUnexpectedEOF
	}
	v := (*b)[:n]
	*b = (*b)[n:]
	return v, nil
}
func sftpReadStr(b *[]byte) (string, error) { x, e := sftpReadBlob(b); return string(x), e }
func sftpWritePacket(w io.Writer, t byte, body []byte) error {
	if len(body) > 1<<20 {
		return errors.New("SFTP response too large")
	}
	data := append([]byte{t}, body...)
	frame := append(sftp32(uint32(len(data))), data...)
	for len(frame) > 0 {
		n, e := w.Write(frame)
		if e != nil {
			return e
		}
		if n < 1 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}
func sftpReadPacket(r io.Reader) (byte, []byte, error) {
	var head [4]byte
	if _, e := io.ReadFull(r, head[:]); e != nil {
		return 0, nil, e
	}
	n := binary.BigEndian.Uint32(head[:])
	if n < 1 || n > 1<<20 {
		return 0, nil, errors.New("unsupported SFTP frame length")
	}
	payload := make([]byte, n)
	if _, e := io.ReadFull(r, payload); e != nil {
		return 0, nil, e
	}
	return payload[0], payload[1:], nil
}
func sftpErrorCode(e error) uint32 {
	if e == nil {
		return sftpOK
	}
	if os.IsNotExist(e) {
		return sftpNoFile
	}
	if os.IsPermission(e) {
		return sftpDenied
	}
	return sftpFailure
}
func (s *sftpSession) status(id, code uint32, msg string) error {
	if len(msg) > 160 {
		msg = msg[:160]
	}
	return sftpWritePacket(s.channel, sftpStatus, append(append(append(sftp32(id), sftp32(code)...), sftpStr(msg)...), sftpStr("en")...))
}
func (s *sftpSession) response(id uint32, t byte, parts ...[]byte) error {
	payload := sftp32(id)
	for _, p := range parts {
		payload = append(payload, p...)
	}
	return sftpWritePacket(s.channel, t, payload)
}
func (s *sftpSession) normalize(request string) (string, string, error) {
	if strings.ContainsRune(request, 0) || len(request) > 4096 {
		return "", "", errors.New("invalid path")
	}
	// SSH clients use POSIX paths even on Windows; strip the virtual root.
	request = strings.ReplaceAll(request, "\\", "/")
	if strings.Contains(request, ":") {
		return "", "", errors.New("absolute drive paths are not allowed")
	}
	virtual := path.Clean("/" + strings.TrimPrefix(request, "/"))
	actual, e := transferPath(s.root, strings.TrimPrefix(virtual, "/"), true)
	return virtual, actual, e
}
func sftpInfoAttrs(info os.FileInfo) []byte {
	mode := uint32(info.Mode().Perm())
	if info.IsDir() {
		mode |= 0040000
	} else {
		mode |= 0100000
	}
	sec := uint32(info.ModTime().Unix())
	return bytes.Join([][]byte{sftp32(sftpFlagSize | sftpFlagPerm | sftpFlagTimes), sftp64(uint64(info.Size())), sftp32(mode), sftp32(sec), sftp32(sec)}, nil)
}
func (s *sftpSession) meta(id uint32, name string, attrs os.FileInfo) error {
	return s.response(id, sftpAttrs, sftpInfoAttrs(attrs))
}
func (s *sftpSession) names(id uint32, names []struct {
	name string
	info os.FileInfo
}) error {
	out := sftp32(uint32(len(names)))
	for _, entry := range names {
		out = append(out, sftpStr(entry.name)...)
		out = append(out, sftpStr(entry.name)...)
		out = append(out, sftpInfoAttrs(entry.info)...)
	}
	return s.response(id, sftpName, out)
}
func (s *sftpSession) handle(id uint32) (*sftpHandleState, error) { _ = id; return nil, nil }
func (s *sftpSession) newHandle(h *sftpHandleState) (string, error) {
	if len(s.handles) >= 64 {
		return "", errors.New("too many open files")
	}
	key := randomTransferID()
	s.handles[key] = h
	return key, nil
}
func (s *sftpSession) handleRequest(t byte, id uint32, b []byte) error {
	fail := func(e error) error { return s.status(id, sftpErrorCode(e), e.Error()) }
	allowedRead := s.access != "write"
	allowedWrite := s.access != "read"
	switch t {
	case sftpRealpath:
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		virtual, full, e := s.normalize(name)
		if e != nil {
			return fail(e)
		}
		info, e := os.Stat(full)
		if e != nil {
			return fail(e)
		}
		return s.names(id, []struct {
			name string
			info os.FileInfo
		}{{virtual, info}})
	case sftpStat, sftpLstat:
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		_, full, e := s.normalize(name)
		if e != nil {
			return fail(e)
		}
		info, e := os.Lstat(full)
		if e != nil {
			return fail(e)
		}
		return s.meta(id, name, info)
	case sftpFstat:
		h, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		item := s.handles[h]
		if item == nil || item.file == nil {
			return s.status(id, sftpFailure, "Invalid file handle")
		}
		info, e := item.file.Stat()
		if e != nil {
			return fail(e)
		}
		return s.meta(id, item.path, info)
	case sftpOpendir:
		if !allowedRead {
			return s.status(id, sftpDenied, "Read access is disabled")
		}
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		_, full, e := s.normalize(name)
		if e != nil {
			return fail(e)
		}
		entries, e := os.ReadDir(full)
		if e != nil {
			return fail(e)
		}
		if len(entries) > 10000 {
			return s.status(id, sftpFailure, "Directory exceeds 10,000 entries")
		}
		infos := make([]os.FileInfo, 0, len(entries))
		for _, entry := range entries {
			info, ee := entry.Info()
			if ee == nil {
				infos = append(infos, info)
			}
		}
		key, e := s.newHandle(&sftpHandleState{entries: infos, path: name, operation: "browse"})
		if e != nil {
			return fail(e)
		}
		return s.response(id, sftpHandle, sftpStr(key))
	case sftpReaddir:
		h, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		item := s.handles[h]
		if item == nil || item.entries == nil {
			return s.status(id, sftpFailure, "Invalid directory handle")
		}
		if item.at >= len(item.entries) {
			return s.status(id, sftpEOF, "End of directory")
		}
		count := 32
		if count > len(item.entries)-item.at {
			count = len(item.entries) - item.at
		}
		items := make([]struct {
			name string
			info os.FileInfo
		}, 0, count)
		for _, info := range item.entries[item.at : item.at+count] {
			items = append(items, struct {
				name string
				info os.FileInfo
			}{info.Name(), info})
		}
		item.at += count
		return s.names(id, items)
	case sftpOpen:
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		flags, e := sftpRead32(&b)
		if e != nil {
			return fail(e)
		}
		if flags&sftpPRead != 0 && !allowedRead {
			return s.status(id, sftpDenied, "Read access is disabled")
		}
		if flags&(sftpPWrite|sftpPAppend|sftpPTrunc|sftpPCreate) != 0 && !allowedWrite {
			return s.status(id, sftpDenied, "Write access is disabled")
		}
		_, full, e := s.normalize(name)
		if e != nil {
			return fail(e)
		}
		oflag := 0
		if flags&sftpPRead != 0 && flags&sftpPWrite != 0 {
			oflag = os.O_RDWR
		} else if flags&sftpPWrite != 0 || flags&sftpPAppend != 0 {
			oflag = os.O_WRONLY
		} else {
			oflag = os.O_RDONLY
		}
		if flags&sftpPCreate != 0 {
			oflag |= os.O_CREATE
		}
		if flags&sftpPExcl != 0 {
			oflag |= os.O_EXCL
		}
		if flags&sftpPAppend != 0 {
			oflag |= os.O_APPEND
		}
		if flags&sftpPTrunc != 0 {
			oflag |= os.O_TRUNC
		}
		if !s.overwrite && oflag&os.O_WRONLY != 0 || !s.overwrite && oflag&os.O_RDWR != 0 {
			// New uploads must use O_EXCL. Existing files may be read, but not modified.
			if _, err := os.Stat(full); err == nil {
				return s.status(id, sftpDenied, "Existing-file overwrite is disabled")
			}
			if oflag&os.O_CREATE != 0 {
				oflag |= os.O_EXCL
			}
		}
		f, e := os.OpenFile(full, oflag, 0600)
		if e != nil {
			return fail(e)
		}
		key, e := s.newHandle(&sftpHandleState{file: f, path: name, operation: func() string {
			if oflag&(os.O_WRONLY|os.O_RDWR) != 0 {
				return "upload"
			}
			return "download"
		}()})
		if e != nil {
			_ = f.Close()
			return fail(e)
		}
		return s.response(id, sftpHandle, sftpStr(key))
	case sftpClose:
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		handle := s.handles[name]
		if handle == nil {
			return s.status(id, sftpFailure, "Invalid file handle")
		}
		delete(s.handles, name)
		if handle.file != nil {
			e = handle.file.Close()
			if handle.bytes > 0 {
				s.server.log(s.peer, handle.operation, handle.path, handle.bytes, e)
			}
		}
		if e != nil {
			return fail(e)
		}
		return s.status(id, sftpOK, "OK")
	case sftpRead:
		if !allowedRead {
			return s.status(id, sftpDenied, "Read access is disabled")
		}
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		offset, e := sftpRead64(&b)
		if e != nil {
			return fail(e)
		}
		n, e := sftpRead32(&b)
		if e != nil {
			return fail(e)
		}
		item := s.handles[name]
		if item == nil || item.file == nil {
			return s.status(id, sftpFailure, "Invalid file handle")
		}
		if n > 32768 {
			n = 32768
		}
		p := make([]byte, n)
		count, e := item.file.ReadAt(p, int64(offset))
		if count > 0 {
			item.bytes += uint64(count)
			return s.response(id, sftpData, sftpBlob(p[:count]))
		}
		if e == io.EOF {
			return s.status(id, sftpEOF, "End of file")
		}
		if e != nil {
			return fail(e)
		}
		return s.status(id, sftpEOF, "End of file")
	case sftpWrite:
		if !allowedWrite {
			return s.status(id, sftpDenied, "Write access is disabled")
		}
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		offset, e := sftpRead64(&b)
		if e != nil {
			return fail(e)
		}
		payload, e := sftpReadBlob(&b)
		if e != nil {
			return fail(e)
		}
		item := s.handles[name]
		if item == nil || item.file == nil {
			return s.status(id, sftpFailure, "Invalid file handle")
		}
		if len(payload) > 256*1024 || offset > 4<<30 || offset+uint64(len(payload)) > 4<<30 {
			return s.status(id, sftpFailure, "Maximum file size is 4 GiB")
		}
		n, e := item.file.WriteAt(payload, int64(offset))
		item.bytes += uint64(n)
		if e != nil {
			return fail(e)
		}
		return s.status(id, sftpOK, "OK")
	case sftpRemove, sftpRmdir, sftpMkdir:
		if !allowedWrite {
			return s.status(id, sftpDenied, "Write access is disabled")
		}
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		_, full, e := s.normalize(name)
		if e != nil {
			return fail(e)
		}
		if full == s.root {
			return s.status(id, sftpDenied, "Cannot modify the root directory")
		}
		if t == sftpMkdir {
			e = os.Mkdir(full, 0700)
		} else if t == sftpRmdir {
			info, ee := os.Stat(full)
			if ee != nil {
				e = ee
			} else if !info.IsDir() {
				e = errors.New("not a directory")
			} else {
				e = os.Remove(full)
			}
		} else {
			info, ee := os.Stat(full)
			if ee != nil {
				e = ee
			} else if info.IsDir() {
				e = errors.New("use Remove directory for a folder")
			} else {
				e = os.Remove(full)
			}
		}
		if e != nil {
			return fail(e)
		}
		action := map[byte]string{sftpRemove: "delete", sftpRmdir: "remove folder", sftpMkdir: "create folder"}[t]
		s.server.log(s.peer, action, name, 0, nil)
		return s.status(id, sftpOK, "OK")
	case sftpRename:
		if !allowedWrite {
			return s.status(id, sftpDenied, "Write access is disabled")
		}
		from, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		to, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		_, src, e := s.normalize(from)
		if e != nil {
			return fail(e)
		}
		_, dst, e := s.normalize(to)
		if e != nil {
			return fail(e)
		}
		if src == s.root || dst == s.root {
			return s.status(id, sftpDenied, "Cannot rename root folder")
		}
		if !s.overwrite {
			if _, e = os.Stat(dst); e == nil {
				return s.status(id, sftpDenied, "Destination exists")
			}
		}
		e = os.Rename(src, dst)
		if e != nil {
			return fail(e)
		}
		s.server.log(s.peer, "rename", from+" to "+to, 0, nil)
		return s.status(id, sftpOK, "OK")
	case sftpSetstat, sftpFsetstat:
		if !allowedWrite {
			return s.status(id, sftpDenied, "Write access is disabled")
		}
		name, e := sftpReadStr(&b)
		if e != nil {
			return fail(e)
		}
		var full string
		if t == sftpSetstat {
			_, full, e = s.normalize(name)
			if e != nil {
				return fail(e)
			}
		} else {
			item := s.handles[name]
			if item == nil || item.file == nil {
				return s.status(id, sftpFailure, "Invalid handle")
			}
			_, full, e = s.normalize(item.path)
			if e != nil {
				return fail(e)
			}
		}
		if full == s.root {
			return s.status(id, sftpDenied, "Cannot modify root attributes")
		}
		flags, e := sftpRead32(&b)
		if e != nil {
			return fail(e)
		}
		if flags&^(sftpFlagSize|sftpFlagPerm|sftpFlagTimes) != 0 {
			return s.status(id, sftpUnsupported, "Only size, permissions and times are supported")
		}
		var size uint64
		var perm uint32
		var access, modified uint32
		if flags&sftpFlagSize != 0 {
			size, e = sftpRead64(&b)
			if e != nil {
				return fail(e)
			}
			if size > 4<<30 {
				return s.status(id, sftpFailure, "File exceeds 4 GiB limit")
			}
		}
		if flags&sftpFlagPerm != 0 {
			perm, e = sftpRead32(&b)
			if e != nil {
				return fail(e)
			}
		}
		if flags&sftpFlagTimes != 0 {
			access, e = sftpRead32(&b)
			if e != nil {
				return fail(e)
			}
			modified, e = sftpRead32(&b)
			if e != nil {
				return fail(e)
			}
		}
		if flags&sftpFlagSize != 0 {
			e = os.Truncate(full, int64(size))
		}
		if e == nil && flags&sftpFlagPerm != 0 {
			e = os.Chmod(full, os.FileMode(perm&0777))
		}
		if e == nil && flags&sftpFlagTimes != 0 {
			e = os.Chtimes(full, time.Unix(int64(access), 0), time.Unix(int64(modified), 0))
		}
		if e != nil {
			return fail(e)
		}
		return s.status(id, sftpOK, "OK")
	case sftpReadlink, sftpSymlink:
		return s.status(id, sftpDenied, "Symbolic links are disabled for sandbox safety")
	default:
		return s.status(id, sftpUnsupported, "Unsupported SFTP operation")
	}
}
func (s *sftpSession) serve() {
	defer s.channel.Close()
	defer func() {
		for _, handle := range s.handles {
			if handle.file != nil {
				_ = handle.file.Close()
				if handle.bytes > 0 {
					s.server.log(s.peer, handle.operation, handle.path, handle.bytes, nil)
				}
			}
		}
	}()
	t, b, e := sftpReadPacket(s.channel)
	if e != nil || t != sftpInit {
		return
	}
	requested, e := sftpRead32(&b)
	if e != nil || requested < 3 {
		return
	}
	if e = sftpWritePacket(s.channel, sftpVersion, sftp32(3)); e != nil {
		return
	}
	for {
		t, b, e = sftpReadPacket(s.channel)
		if e != nil {
			return
		}
		id, e := sftpRead32(&b)
		if e != nil {
			return
		}
		if e = s.handleRequest(t, id, b); e != nil {
			return
		}
	}
}
func loadTransferHostKey(base string) (ssh.Signer, error) {
	folder := filepath.Join(base, "security")
	if e := os.MkdirAll(folder, 0700); e != nil {
		return nil, e
	}
	filename := filepath.Join(folder, "sftp_host_ed25519.pem")
	key, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		_, priv, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return nil, e
		}
		raw, e := x509.MarshalPKCS8PrivateKey(priv)
		if e != nil {
			return nil, e
		}
		key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw})
		f, e := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		_, e = f.Write(key)
		if ee := f.Close(); e == nil {
			e = ee
		}
		if e != nil {
			return nil, e
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, errors.New("invalid SSH host-key PEM")
	}
	raw, e := x509.ParsePKCS8PrivateKey(block.Bytes)
	if e != nil {
		return nil, e
	}
	return ssh.NewSignerFromKey(raw)
}
func parseAuthorizedKeys(filename string) (map[string]bool, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, nil
	}
	content, e := os.ReadFile(filename)
	if e != nil {
		return nil, fmt.Errorf("read authorized keys: %w", e)
	}
	if len(content) > 1<<20 {
		return nil, errors.New("authorized key file is too large")
	}
	keys := make(map[string]bool)
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, _, _, _, e := ssh.ParseAuthorizedKey([]byte(line))
		if e != nil {
			return nil, fmt.Errorf("invalid authorized SSH key: %w", e)
		}
		keys[ssh.FingerprintSHA256(key)] = true
	}
	if len(keys) == 0 {
		return nil, errors.New("authorized-key file contains no usable public keys")
	}
	return keys, nil
}
func startSFTPServer(ctx context.Context, s *transferHost, base, username, password, authorizedPath string) (net.Listener, error) {
	signer, e := loadTransferHostKey(base)
	if e != nil {
		return nil, e
	}
	keys, e := parseAuthorizedKeys(authorizedPath)
	if e != nil {
		return nil, e
	}
	if password == "" && len(keys) == 0 {
		return nil, errors.New("configure a password or authorized SSH keys")
	}
	salt := make([]byte, 32)
	_, _ = rand.Read(salt)
	expected := sha256.Sum256(append(append([]byte{}, salt...), password...))
	config := &ssh.ServerConfig{ServerVersion: "SSH-2.0-NetworkToolkitSFTP", MaxAuthTries: 6}
	if password != "" {
		config.PasswordCallback = func(meta ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			actual := sha256.Sum256(append(append([]byte{}, salt...), pass...))
			if meta.User() != username || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
				return nil, errors.New("authentication failed")
			}
			return nil, nil
		}
	}
	if len(keys) > 0 {
		config.PublicKeyCallback = func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if meta.User() != username || !keys[ssh.FingerprintSHA256(key)] {
				return nil, errors.New("public key not authorized")
			}
			return nil, nil
		}
	}
	config.AddHostKey(signer)
	s.sshFingerprint = ssh.FingerprintSHA256(signer.PublicKey())
	ln, e := net.Listen("tcp", s.address)
	if e != nil {
		return nil, fmt.Errorf("SFTP TCP listener: %w", e)
	}
	go func() {
		defer ln.Close()
		for {
			connection, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			active := s.active
			s.mu.Unlock()
			if active >= 16 {
				_ = connection.Close()
				continue
			}
			go serveSFTPConnection(ctx, s, connection, config)
		}
	}()
	return ln, nil
}
func serveSFTPConnection(ctx context.Context, s *transferHost, conn net.Conn, config *ssh.ServerConfig) {
	s.activity(1)
    s.log(conn.RemoteAddr().String(),"CONNECT","",0,nil)
	defer s.activity(-1)
    defer s.log(conn.RemoteAddr().String(),"DISCONNECT","",0,nil)
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	server, channels, requests, e := ssh.NewServerConn(conn, config)
	if e != nil {s.log(conn.RemoteAddr().String(),"AUTH", "", 0,errors.New("SSH handshake or authentication rejected"));return}
    s.log(conn.RemoteAddr().String(),"LOGIN",server.User(),0,nil)
	defer server.Close()
	_ = conn.SetDeadline(time.Time{})
	go ssh.DiscardRequests(requests)
	for request := range channels {
		if request.ChannelType() != "session" {
			_ = request.Reject(ssh.UnknownChannelType, "SFTP only")
			continue
		}
		ch, reqs, err := request.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer ch.Close()
			opened := false
			for r := range reqs {
				if opened {
					if r.WantReply {
						_ = r.Reply(false, nil)
					}
					continue
				}
				// Reject shell, terminal and command requests. Only the SFTP subsystem is supported.
				if r.Type != "subsystem" || len(r.Payload) < 4 || int(binary.BigEndian.Uint32(r.Payload[:4])) != len("sftp") || string(r.Payload[4:]) != "sftp" {
					if r.WantReply {
						_ = r.Reply(false, nil)
					}
					continue
				}
				opened = true
				if r.WantReply {
					_ = r.Reply(true, nil)
				}
				session := &sftpSession{channel: ch, server: s, peer: server.RemoteAddr().String(), root: s.root, access: s.access, overwrite: s.overwrite, handles: make(map[string]*sftpHandleState)}
				session.serve()
				return
			}
		}()
	}
}
