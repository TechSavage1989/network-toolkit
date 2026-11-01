package diag

// SFTP v3 client over the application's existing vendored SSH transport.
// The connection is bound to one diagnostic operation, and credentials are
// held only in the request while it runs, never in a retained result.
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/model"
	"networktoolkit/internal/sshpolicy"
)

const (
	fxInit     byte = 1
	fxVersion  byte = 2
	fxOpen     byte = 3
	fxClose    byte = 4
	fxRead     byte = 5
	fxWrite    byte = 6
	fxLstat    byte = 7
	fxSetstat  byte = 9
	fxOpendir  byte = 11
	fxReaddir  byte = 12
	fxRemove   byte = 13
	fxMkdir    byte = 14
	fxRmdir    byte = 15
	fxRename   byte = 18
	fxStatus   byte = 101
	fxHandle   byte = 102
	fxData     byte = 103
	fxName     byte = 104
	fxAttrs    byte = 105
	sftpEOF         = 1
	sftpRead        = 1
	sftpWrite       = 2
	sftpCreate      = 8
	sftpTrunc       = 16
	sftpExcl        = 32
)

type wireSFTP struct {
	posixRename bool
	in          io.Reader
	out         io.Writer
	id          uint32
	conn        net.Conn
	timeout     time.Duration
	abort       func() // closes this SFTP subsystem, never the borrowed SSH connection
}
type sftpFileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Size     uint64 `json:"size"`
	Modified string `json:"modified,omitempty"`
	Mode     string `json:"mode,omitempty"`
}
type sftpAttrs struct {
	size      uint64
	mode      uint32
	modified  uint32
	sizeKnown bool
	modeKnown bool
}
type sftpWireError struct {
	code    uint32
	message string
}

func (e sftpWireError) Error() string { return fmt.Sprintf("SFTP status %d: %s", e.code, e.message) }
func sfU32(v uint32) []byte           { var b [4]byte; binary.BigEndian.PutUint32(b[:], v); return b[:] }
func sfU64(v uint64) []byte           { var b [8]byte; binary.BigEndian.PutUint64(b[:], v); return b[:] }
func sfString(s string) []byte        { b := []byte(s); return append(sfU32(uint32(len(b))), b...) }
func sfBytes(b []byte) []byte         { return append(sfU32(uint32(len(b))), b...) }
func sfTake(b *[]byte, n uint32) ([]byte, error) {
	if uint64(n) > uint64(len(*b)) {
		return nil, errors.New("malformed SFTP reply")
	}
	v := (*b)[:n]
	*b = (*b)[n:]
	return v, nil
}
func sfGetU32(b *[]byte) (uint32, error) {
	s, e := sfTake(b, 4)
	if e != nil {
		return 0, e
	}
	return binary.BigEndian.Uint32(s), nil
}
func sfGetString(b *[]byte) (string, error) {
	n, e := sfGetU32(b)
	if e != nil {
		return "", e
	}
	v, e := sfTake(b, n)
	return string(v), e
}
func sfGetData(b *[]byte) ([]byte, error) {
	n, e := sfGetU32(b)
	if e != nil {
		return nil, e
	}
	return sfTake(b, n)
}
func (c *wireSFTP) send(t byte, p []byte) error {
	if c.conn != nil && c.timeout > 0 {
		if e := c.conn.SetDeadline(time.Now().Add(c.timeout)); e != nil {
			return e
		}
	}
	if len(p) > 1<<20 {
		return errors.New("SFTP request exceeds size limit")
	}
	var frame bytes.Buffer
	frame.Write(sfU32(uint32(1 + len(p))))
	frame.WriteByte(t)
	frame.Write(p)
	// Some writers accept a partial frame without reporting an error.
	// Continue until the full packet is sent, or fail rather than hang on no progress.
	remaining := frame.Bytes()
	for len(remaining) > 0 {
		n, err := c.out.Write(remaining)
		if n < 0 || n > len(remaining) {
			return io.ErrShortWrite
		}
		remaining = remaining[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
func (c *wireSFTP) recv() (byte, []byte, error) {
	var lenBuf [4]byte
	if _, e := io.ReadFull(c.in, lenBuf[:]); e != nil {
		return 0, nil, e
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n < 1 || n > 4<<20 {
		return 0, nil, errors.New("SFTP response size outside supported bounds")
	}
	p := make([]byte, n)
	if _, e := io.ReadFull(c.in, p); e != nil {
		return 0, nil, e
	}
	return p[0], p[1:], nil
}
// The underlying SSH transport belongs to the interactive terminal when
// conn is nil. Bound each SFTP response separately so a stalled server does
// not hold file browsing indefinitely, without imposing a total transfer
// time limit or changing deadlines on the shared SSH connection.
func (c *wireSFTP) exchange(send func() error) (byte, []byte, error) {
	if c.conn != nil || c.timeout <= 0 || c.abort == nil {
		if err := send(); err != nil { return 0, nil, err }
		return c.recv()
	}
	var expired atomic.Bool
	timer := time.AfterFunc(c.timeout, func() {
		expired.Store(true)
		c.abort()
	})
	defer timer.Stop()
	if err := send(); err != nil {
		if expired.Load() { return 0, nil, context.DeadlineExceeded }
		return 0, nil, err
	}
	kind, reply, err := c.recv()
	if expired.Load() { return 0, nil, context.DeadlineExceeded }
	return kind, reply, err
}
func (c *wireSFTP) init() error {
	t, p, e := c.exchange(func() error { return c.send(fxInit, sfU32(3)) })
	if e != nil {
		return e
	}
	if t != fxVersion || len(p) < 4 {
		return errors.New("remote does not support the SFTP subsystem")
	}
	v := binary.BigEndian.Uint32(p[:4])
	if v != 3 {
		return fmt.Errorf("SFTP version %d is unsupported; this client uses version 3", v)
	}
	p = p[4:]
	for len(p) > 0 {
		name, err := sfGetString(&p)
		if err != nil {
			return err
		}
		value, err := sfGetString(&p)
		if err != nil {
			return err
		}
		if name == "posix-rename@openssh.com" && value == "1" {
			c.posixRename = true
		}
	}
	return nil
}
func (c *wireSFTP) request(t byte, parts ...[]byte) (byte, []byte, error) {
	c.id++
	body := sfU32(c.id)
	for _, p := range parts {
		body = append(body, p...)
	}
	typ, reply, e := c.exchange(func() error { return c.send(t, body) })
	if e != nil {
		return 0, nil, e
	}
	id, e := sfGetU32(&reply)
	if e != nil {
		return 0, nil, e
	}
	if id != c.id {
		return 0, nil, fmt.Errorf("SFTP response ID %d did not match request %d", id, c.id)
	}
	if typ == fxStatus {
		code, e := sfGetU32(&reply)
		if e != nil {
			return 0, nil, e
		}
		if code != 0 {
			message, _ := sfGetString(&reply)
			if message == "" {
				message = "remote operation rejected"
			}
			return typ, nil, sftpWireError{code, message}
		}
	}
	return typ, reply, nil
}
func sfDecodeAttrs(b *[]byte) (sftpAttrs, error) {
	var a sftpAttrs
	flags, e := sfGetU32(b)
	if e != nil {
		return a, e
	}
	if flags&1 != 0 {
		raw, e := sfTake(b, 8)
		if e != nil {
			return a, e
		}
		a.size = binary.BigEndian.Uint64(raw)
		a.sizeKnown = true
	}
	if flags&2 != 0 {
		if _, e := sfTake(b, 8); e != nil {
			return a, e
		}
	}
	if flags&4 != 0 {
		a.mode, e = sfGetU32(b)
		if e != nil {
			return a, e
		}
		a.modeKnown = true
	}
	if flags&8 != 0 {
		if _, e := sfTake(b, 4); e != nil {
			return a, e
		}
		a.modified, e = sfGetU32(b)
		if e != nil {
			return a, e
		}
	}
	if flags&0x80000000 != 0 {
		count, e := sfGetU32(b)
		if e != nil {
			return a, e
		}
		if count > 64 {
			return a, errors.New("too many SFTP attribute extensions")
		}
		for i := uint32(0); i < count; i++ {
			if _, e := sfGetData(b); e != nil {
				return a, e
			}
			if _, e := sfGetData(b); e != nil {
				return a, e
			}
		}
	}
	return a, nil
}
func sfInfo(name, remote string, a sftpAttrs) sftpFileInfo {
	kind := "File"
	switch a.mode & 0170000 {
	case 0040000:
		kind = "Directory"
	case 0120000:
		kind = "Link"
	}
	if name == "." || name == ".." {
		kind = "Directory"
	}
	v := sftpFileInfo{Name: name, Path: path.Join(remote, name), Kind: kind, Size: a.size}
	if a.modified != 0 {
		v.Modified = time.Unix(int64(a.modified), 0).UTC().Format(time.RFC3339)
	}
	if a.modeKnown {
		v.Mode = fmt.Sprintf("%04o", a.mode&07777)
	}
	return v
}
func (c *wireSFTP) handle(t byte, p []byte) ([]byte, error) {
	kind, data, e := c.request(t, p)
	if e != nil {
		return nil, e
	}
	if kind != fxHandle {
		return nil, errors.New("expected SFTP file handle")
	}
	return sfGetData(&data)
}
func (c *wireSFTP) closeHandle(handle []byte) error {
	kind, _, e := c.request(fxClose, sfBytes(handle))
	if e != nil {
		return e
	}
	if kind != fxStatus {
		return errors.New("invalid SFTP close acknowledgement")
	}
	return nil
}
func (c *wireSFTP) listing(dir string) ([]sftpFileInfo, error) {
	h, e := c.handle(fxOpendir, sfString(dir))
	if e != nil {
		return nil, e
	}
	defer c.closeHandle(h)
	list := []sftpFileInfo{}
	for {
		kind, data, e := c.request(fxReaddir, sfBytes(h))
		if e != nil {
			var status sftpWireError
			if errors.As(e, &status) && status.code == sftpEOF {
				break
			}
			return nil, e
		}
		if kind != fxName {
			return nil, errors.New("invalid SFTP directory listing")
		}
		n, e := sfGetU32(&data)
		if e != nil {
			return nil, e
		}
		if n > 5000 || len(list)+int(n) > 10000 {
			return nil, errors.New("directory listing exceeds 10,000 entries")
		}
		for i := uint32(0); i < n; i++ {
			name, e := sfGetString(&data)
			if e != nil {
				return nil, e
			}
			if _, e = sfGetString(&data); e != nil {
				return nil, e
			}
			attrs, e := sfDecodeAttrs(&data)
			if e != nil {
				return nil, e
			}
			if name != "." && name != ".." {
				list = append(list, sfInfo(name, dir, attrs))
			}
		}
	}
	return list, nil
}
func (c *wireSFTP) stat(file string) (sftpAttrs, error) {
	t, data, e := c.request(fxLstat, sfString(file))
	if e != nil {
		return sftpAttrs{}, e
	}
	if t != fxAttrs {
		return sftpAttrs{}, errors.New("invalid SFTP stat response")
	}
	return sfDecodeAttrs(&data)
}
func (c *wireSFTP) remove(t byte, remote string) error {
	typ, _, e := c.request(t, sfString(remote))
	if e != nil {
		return e
	}
	if typ != fxStatus {
		return errors.New("unexpected SFTP acknowledgement")
	}
	return nil
}
func (c *wireSFTP) open(remote string, flags uint32) ([]byte, error) {
	return c.handle(fxOpen, append(append(sfString(remote), sfU32(flags)...), sfU32(0)...))
}

// Read small editor files directly, with one authenticated connection and one
// subsystem. Enforce the limit again while reading in case the file grows.
func (c *wireSFTP) readText(ctx context.Context, remote string) ([]byte, error) {
	const limit = 16 << 20
	attrs, err := c.stat(remote)
	if err != nil {
		return nil, err
	}
	if attrs.mode&0170000 == 0040000 {
		return nil, errors.New("remote path is a directory; choose a file")
	}
	if attrs.size > limit {
		return nil, errors.New("Text file exceeds the 16 MiB editor limit")
	}
	handle, err := c.open(remote, sftpRead)
	if err != nil {
		return nil, err
	}
	defer c.closeHandle(handle)
	var contents bytes.Buffer
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Preserve the 32 KiB request size supported by older network devices.
		kind, reply, err := c.request(fxRead, sfBytes(handle), sfU64(uint64(contents.Len())), sfU32(32768))
		if err != nil {
			var status sftpWireError
			if errors.As(err, &status) && status.code == sftpEOF {
				return contents.Bytes(), nil
			}
			return nil, err
		}
		if kind != fxData {
			return nil, errors.New("unexpected SFTP read response")
		}
		chunk, err := sfGetData(&reply)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			return contents.Bytes(), nil
		}
		if contents.Len()+len(chunk) > limit {
			return nil, errors.New("Text file exceeds the 16 MiB editor limit")
		}
		contents.Write(chunk)
	}
}
func (c *wireSFTP) download(ctx context.Context, remote, local string, overwrite bool, report func(uint64, uint64), safeExport ...int64) (uint64, error) {
	attrs, e := c.stat(remote)
	if e != nil {
		return 0, e
	}
	bounded := len(safeExport) > 0 && safeExport[0] != -1
	if bounded && (!attrs.modeKnown || attrs.mode&0170000 != 0100000 || attrs.size > 2<<30 || attrs.size != uint64(safeExport[0])) {
		return 0, errors.New("prepared export requires a known regular file no larger than 2 GiB")
	}
	if attrs.mode&0170000 == 0040000 {
		return 0, errors.New("remote path is a directory; choose a file")
	}
	if !overwrite {
		if _, e = os.Stat(local); e == nil {
			return 0, errors.New("local file already exists; enable Overwrite to replace it")
		}
		if !errors.Is(e, os.ErrNotExist) {
			return 0, e
		}
	}
	h, e := c.open(remote, sftpRead)
	if e != nil {
		return 0, e
	}
	defer c.closeHandle(h)
	dir := filepath.Dir(local)
	temp, e := os.CreateTemp(dir, ".ntk-sftp-*.part")
	if e != nil {
		return 0, e
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	var total uint64
	for {
		if e = ctx.Err(); e != nil {
			return total, e
		}
		t, b, er := c.request(fxRead, sfBytes(h), sfU64(total), sfU32(32768))
		if er != nil {
			var status sftpWireError
			if errors.As(er, &status) && status.code == sftpEOF {
				break
			}
			return total, er
		}
		if t != fxData {
			return total, errors.New("unexpected SFTP read response")
		}
		chunk, er := sfGetData(&b)
		if er != nil {
			return total, er
		}
		if len(chunk) == 0 {
			break
		}
		if bounded && (total+uint64(len(chunk)) > attrs.size || total+uint64(len(chunk)) > 2<<30) {
			return total, errors.New("remote file changed size or exceeds the prepared-copy limit")
		}
		if _, er = temp.Write(chunk); er != nil {
			return total, er
		}
		total += uint64(len(chunk))
		report(total, attrs.size)
	}
	if bounded && total != attrs.size {
		return total, errors.New("remote file changed size during prepared copy")
	}
	if e = temp.Sync(); e != nil {
		return total, e
	}
	if e = temp.Close(); e != nil {
		return total, e
	}
	if overwrite {
		if e = os.Remove(local); e != nil && !os.IsNotExist(e) {
			return total, e
		}
	}
	if e = os.Rename(temp.Name(), local); e != nil {
		return total, e
	}
	return total, nil
}
func (c *wireSFTP) upload(ctx context.Context, local, remote string, overwrite bool, report func(uint64, uint64), onCreated ...func()) (uint64, error) {
	file, e := os.Open(local)
	if e != nil {
		return 0, e
	}
	defer file.Close()
	info, e := file.Stat()
	if e != nil {
		return 0, e
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("upload requires a regular local file")
	}
	flags := uint32(sftpWrite | sftpCreate | sftpExcl)
	if overwrite {
		flags = sftpWrite | sftpCreate | sftpTrunc
	}
	h, e := c.open(remote, flags)
	if e != nil {
		return 0, e
	}
	// Ownership exists only after the server acknowledged our exclusive OPEN.
	// An unacknowledged/failed creation must never authorize pathname cleanup.
	if len(onCreated) > 0 && onCreated[0] != nil {
		onCreated[0]()
	}
	closed := false
	defer func() {
		if !closed {
			_ = c.closeHandle(h)
		}
	}()
	buf := make([]byte, 32768)
	var total uint64
	for {
		if e = ctx.Err(); e != nil {
			return total, e
		}
		n, er := file.Read(buf)
		if n > 0 {
			payload := append(append(sfBytes(h), sfU64(total)...), sfBytes(buf[:n])...)
			t, _, ee := c.request(fxWrite, payload)
			if ee != nil {
				return total, ee
			}
			if t != fxStatus {
				return total, errors.New("invalid SFTP write acknowledgement")
			}
			total += uint64(n)
			report(total, uint64(info.Size()))
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			return total, er
		}
	}
	closed = true
	if e = c.closeHandle(h); e != nil {
		return total, fmt.Errorf("remote file close/flush failed: %w", e)
	}
	return total, nil
}

// Protocol references: SFTP v3 section 6.5 forbids an existing RENAME target:
// https://www.openssh.org/txt/draft-ietf-secsh-filexfer-02.txt
// POSIX replacement requires negotiated extension version 1 (OpenSSH PROTOCOL 4.3):
// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL
// uploadStaged is opt-in for browser-granted drops. Never truncate the original.
// SFTP v3 RENAME refuses an existing target; an overwrite requires the advertised
// atomic POSIX rename extension. Unsupported servers fail before any file bytes.
func (c *wireSFTP) uploadStaged(ctx context.Context, local, remote string, overwrite bool, report func(uint64, uint64)) (total uint64, err error) {
	if overwrite && !c.posixRename {
		return 0, errors.New("server does not advertise atomic overwrite; choose Skip or Rename")
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return 0, err
	}
	stage := path.Join(path.Dir(remote), ".ntk-upload-"+hex.EncodeToString(random[:])+".tmp")
	published, stageOwned := false, false
	defer func() {
		if stageOwned && !published {
			cleanup := c.remove(fxRemove, stage)
			if cleanup != nil {
				var missing sftpWireError
				if !errors.As(cleanup, &missing) || missing.code != 2 {
					err = fmt.Errorf("%w; staging cleanup not confirmed for %s (inspect remote folder before retrying)", err, stage)
				}
			}
		}
	}()
	total, err = c.upload(ctx, local, stage, false, report, func() { stageOwned = true })
	if err != nil {
		return total, err
	}
	if err = ctx.Err(); err != nil {
		return total, err
	}
	var kind byte
	if overwrite {
		kind, _, err = c.request(200, sfString("posix-rename@openssh.com"), sfString(stage), sfString(remote))
	} else {
		kind, _, err = c.request(fxRename, sfString(stage), sfString(remote))
	}
	if err != nil {
		var status sftpWireError
		if !errors.As(err, &status) {
			err = fmt.Errorf("publication result is uncertain for %s: %w; refresh before retrying", remote, err)
		}
		return total, err
	}
	if kind != fxStatus {
		return total, errors.New("invalid SFTP publication acknowledgement; refresh destination before retrying")
	}
	published = true
	return total, nil
}

var errSFTPInspect = errors.New("SFTP inspection complete")

func sftpTest(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	host, e := validateHost(req.Target)
	if e != nil {
		return model.Result{}, e
	}
	port, e := intOption(req.Options, "port", 22, 1, 65535)
	if e != nil {
		return model.Result{}, e
	}
	timeout, e := durationOption(req.Options, "timeoutMs", 10000, 500, 60000)
	if e != nil {
		return model.Result{}, e
	}
	operation := stringOption(req.Options, "operation", "list")
	switch operation {
	case "fingerprint", "list", "stat", "download", "read-text", "upload", "mkdir", "remove", "rmdir", "rename":
	default:
		return model.Result{}, errors.New("invalid SFTP operation")
	}
	username := stringOption(req.Options, "username", "")
	if username == "" && operation != "fingerprint" {
		return model.Result{}, errors.New("enter an SFTP username")
	}
	remote := stringOption(req.Options, "remotePath", ".")
	local := stringOption(req.Options, "localPath", "")
	newRemote := stringOption(req.Options, "newPath", "")
	if len(remote) > 4096 || len(local) > 4096 || len(newRemote) > 4096 {
		return model.Result{}, errors.New("file path exceeds 4,096 characters")
	}
	if strings.ContainsRune(remote, 0) || strings.ContainsRune(newRemote, 0) {
		return model.Result{}, errors.New("remote path contains invalid characters")
	}
	if local == "" && (operation == "upload" || operation == "download") {
		return model.Result{}, errors.New("enter a local file path")
	}
	if (operation == "remove" || operation == "rmdir") && !boolOption(req.Options, "confirmDelete", false) {
		return model.Result{}, errors.New("remote deletion requires Confirm remote delete")
	}
	if newRemote == "" && operation == "rename" {
		return model.Result{}, errors.New("enter the new remote path")
	}
	authMode := stringOption(req.Options, "authMode", "password")
	pass := stringOption(req.Options, "password", "")
	passphrase := stringOption(req.Options, "passphrase", "")
	fingerprint := stringOption(req.Options, "fingerprint", "")
	keyPath := stringOption(req.Options, "keyPath", "")
	privateKey := stringOption(req.Options, "privateKey", "")
	if len(pass) > 4096 || len(passphrase) > 4096 || len(keyPath) > 4096 || len(privateKey) > 1<<20 {
		return model.Result{}, errors.New("authentication field exceeds supported length")
	}
	if authMode != "password" && authMode != "key" {
		return model.Result{}, errors.New("use password or SSH private key authentication")
	}
	if operation != "fingerprint" && authMode == "password" && pass == "" {
		return model.Result{}, errors.New("enter an SFTP password")
	}
	if operation != "fingerprint" && authMode == "key" && keyPath == "" && privateKey == "" {
		return model.Result{}, errors.New("enter the local SSH private key path")
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	data := map[string]any{"protocol": "SFTP", "server": host, "port": port, "operation": operation, "remotePath": remote}
	if local != "" {
		data["localPath"] = local
	}
	if newRemote != "" {
		data["newPath"] = newRemote
	}
	started := time.Now()
	var auth []ssh.AuthMethod
	if operation != "fingerprint" {
		if authMode == "password" {
			auth = []ssh.AuthMethod{ssh.Password(pass)}
		} else {
			var raw []byte
			var err error
			if privateKey != "" {
				raw = []byte(privateKey)
			} else {
				raw, err = os.ReadFile(keyPath)
				if err != nil {
					return model.Result{}, fmt.Errorf("read SSH private key: %w", err)
				}
			}
			if len(raw) > 1<<20 {
				return model.Result{}, errors.New("SSH private key exceeds 1 MiB")
			}
			var signer ssh.Signer
			if passphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(raw, []byte(passphrase))
			} else {
				signer, err = ssh.ParsePrivateKey(raw)
			}
			if err != nil {
				return model.Result{}, fmt.Errorf("parse SSH private key: %w", err)
			}
			auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	}
	var presented string
	callback := func(hostport string, addr net.Addr, key ssh.PublicKey) error {
		presented = ssh.FingerprintSHA256(key)
		data["hostKeyFingerprint"] = presented
		data["hostKeyType"] = key.Type()
		if operation == "fingerprint" {
			return errSFTPInspect
		}
		if fingerprint != "" {
			want := strings.TrimSpace(fingerprint)
			if !strings.HasPrefix(want, "SHA256:") {
				want = "SHA256:" + want
			}
			if subtle.ConstantTimeCompare([]byte(want), []byte(presented)) != 1 {
				return fmt.Errorf("SSH host key mismatch: expected %s, presented %s", want, presented)
			}
			return nil
		}
		return fmt.Errorf("SSH host key is not yet verified. Confirm %s using an independent trusted source, then enter it in Expected host fingerprint", presented)
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, e := (&net.Dialer{Timeout: timeout}).DialContext(dialCtx, "tcp", address)
	if e != nil {
		return model.Result{}, fmt.Errorf("SFTP connect: %w", e)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	// Use the terminal's exact algorithm negotiation, including the explicit
	// legacy profile if selected, to avoid negotiating a different host key
	// for the same saved SSH connection.
	compatibility := stringOption(req.Options, "sshCompatibility", "modern")
	if compatibility == "" {
		compatibility = "modern"
	}
	if compatibility != "modern" && compatibility != "legacy" {
		return model.Result{}, errors.New("invalid SSH compatibility policy")
	}
	algorithms := sshpolicy.Algorithms(compatibility)
	config := &ssh.ClientConfig{User: username, Auth: auth, HostKeyCallback: callback, Timeout: timeout, ClientVersion: "SSH-2.0-NetworkToolkit",
		HostKeyAlgorithms: algorithms.HostKeys,
		Config:            ssh.Config{KeyExchanges: algorithms.KeyExchanges, Ciphers: algorithms.Ciphers, MACs: algorithms.MACs},
	}
	sshConn, channels, requests, e := ssh.NewClientConn(conn, address, config)
	if operation == "fingerprint" && presented != "" {
		return model.Result{Summary: "Server SSH host-key fingerprint inspected. Verify it before connecting.", Data: data}, nil
	}
	if e != nil {
		return model.Result{}, fmt.Errorf("SFTP SSH handshake: %w", e)
	}
	client := ssh.NewClient(sshConn, channels, requests)
	defer client.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	event(emit, "sftp", "info", "SSH connection established; host key verified", nil)
	return runSFTPChannel(ctx, req, emit, client, conn, timeout, data, started, operation, remote, local, newRemote)
}

// RunSFTPOnClient opens a separate SFTP subsystem channel on a live SSH
// terminal connection. Host-key trust and authentication have already been
// completed by the terminal manager. No password/key material is accepted here.
// A cancelled browser request closes only its subsystem channel.
func RunSFTPOnClient(ctx context.Context, client *ssh.Client, req model.Request, emit model.Emit) (model.Result, error) {
	if client == nil {
		return model.Result{}, errors.New("SSH terminal is not connected")
	}
	operation := stringOption(req.Options, "operation", "list")
	switch operation {
	case "probe", "list", "stat", "upload", "download", "read-text", "mkdir", "remove", "rmdir", "rename":
	default:
		return model.Result{}, errors.New("invalid SFTP operation on terminal")
	}
	remote := stringOption(req.Options, "remotePath", ".")
	local := stringOption(req.Options, "localPath", "")
	newRemote := stringOption(req.Options, "newPath", "")
	if len(remote) > 4096 || len(local) > 4096 || len(newRemote) > 4096 || strings.ContainsRune(remote, 0) || strings.ContainsRune(newRemote, 0) {
		return model.Result{}, errors.New("file path is invalid or exceeds 4096 characters")
	}
	if local == "" && (operation == "upload" || operation == "download") {
		return model.Result{}, errors.New("enter a local file path")
	}
	if newRemote == "" && operation == "rename" {
		return model.Result{}, errors.New("enter the new remote path")
	}
	if (operation == "remove" || operation == "rmdir") && !boolOption(req.Options, "confirmDelete", false) {
		return model.Result{}, errors.New("remote deletion requires confirmation")
	}
	timeout, e := durationOption(req.Options, "timeoutMs", 15000, 500, 60000)
	if e != nil {
		return model.Result{}, e
	}
	data := map[string]any{"protocol": "SFTP", "operation": operation, "remotePath": remote, "connection": "terminal"}
	if local != "" {
		data["localPath"] = local
	}
	if newRemote != "" {
		data["newPath"] = newRemote
	}
	return runSFTPChannel(ctx, req, emit, client, nil, timeout, data, time.Now(), operation, remote, local, newRemote)
}

func runSFTPChannel(ctx context.Context, req model.Request, emit model.Emit, client *ssh.Client, conn net.Conn, timeout time.Duration, data map[string]any, started time.Time, operation, remote, local, newRemote string) (model.Result, error) {
	// Cancelled operations must not allocate a new channel on an existing
	// authenticated terminal connection.
	if err := ctx.Err(); err != nil {
		return model.Result{}, err
	}
	sess, e := client.NewSession()
	if e != nil {
		return model.Result{}, fmt.Errorf("SFTP session: %w", e)
	}
	defer sess.Close()
	stop := context.AfterFunc(ctx, func() { _ = sess.Close() })
	defer stop()
	stdin, e := sess.StdinPipe()
	if e != nil {
		return model.Result{}, e
	}
	stdout, e := sess.StdoutPipe()
	if e != nil {
		return model.Result{}, e
	}
	if e = sess.RequestSubsystem("sftp"); e != nil {
		if ctx.Err() != nil {
			return model.Result{}, ctx.Err()
		}
		return model.Result{}, fmt.Errorf("server rejected SFTP subsystem: %w", e)
	}
	protocol := wireSFTP{in: stdout, out: stdin, conn: conn, timeout: timeout, abort: func() { _ = sess.Close() }}
	if e = protocol.init(); e != nil {
		if ctx.Err() != nil {
			return model.Result{}, ctx.Err()
		}
		return model.Result{}, e
	}
	if err := ctx.Err(); err != nil {
		return model.Result{}, err
	}
	// Availability detection is intentionally read-only: successful SFTP
	// subsystem negotiation and version exchange suffice. No directory listing.
	if operation == "probe" {
		return model.Result{Summary: "SFTP subsystem supported", Data: map[string]any{"supported": true}}, nil
	}
	reportAt := time.Time{}
	progress := func(done, all uint64) {
		if time.Since(reportAt) < 300*time.Millisecond && done < all {
			return
		}
		reportAt = time.Now()
		fraction := 0.0
		if all > 0 {
			fraction = 100 * float64(done) / float64(all)
		}
		event(emit, "transfer", "info", fmt.Sprintf("Transferred %d of %d bytes (%.1f%%)", done, all, fraction), map[string]any{"type": "transfer-progress", "bytes": done, "totalBytes": all, "percent": fraction})
	}
	var summary string
	switch operation {
	case "read-text":
		var contents []byte
		contents, e = protocol.readText(ctx, remote)
		if e == nil {
			data["contentBytes"] = contents
			summary = "SFTP file read completed"
		}
	case "list":
		var items []sftpFileInfo
		items, e = protocol.listing(remote)
		if e == nil {
			data["entries"] = items
			data["entryCount"] = len(items)
			summary = fmt.Sprintf("SFTP directory: %d entries", len(items))
		}
	case "stat":
		var a sftpAttrs
		a, e = protocol.stat(remote)
		if e == nil {
			data["sizeBytes"] = a.size
			if a.mode&0170000 == 0040000 {
				data["kind"] = "directory"
			} else if a.modeKnown && a.mode&0170000 == 0120000 {
				data["kind"] = "link"
			} else if a.modeKnown && a.mode&0170000 != 0100000 {
				data["kind"] = "special"
			} else {
				data["kind"] = "file"
			}
			data["mode"] = fmt.Sprintf("%04o", a.mode&07777)
			if a.modified != 0 {
				data["modified"] = time.Unix(int64(a.modified), 0).UTC().Format(time.RFC3339)
			}
			summary = "Remote file information retrieved"
		}
	case "mkdir":
		var kind byte
		kind, _, e = protocol.request(fxMkdir, sfString(remote), sfU32(0))
		if e == nil && kind != fxStatus {
			e = errors.New("invalid SFTP mkdir acknowledgement")
		}
		summary = "Remote directory created"
	case "remove":
		e = protocol.remove(fxRemove, remote)
		summary = "Remote file removed"
	case "rmdir":
		e = protocol.remove(fxRmdir, remote)
		summary = "Remote directory removed"
	case "rename":
		t, _, err := protocol.request(fxRename, sfString(remote), sfString(newRemote))
		e = err
		if e == nil && t != fxStatus {
			e = errors.New("invalid SFTP rename acknowledgement")
		}
		summary = "Remote path renamed"
	case "chmod":
		textMode := stringOption(req.Options, "permissions", "")
		value, parseErr := strconv.ParseUint(textMode, 8, 16)
		if parseErr != nil || len(textMode) < 3 || len(textMode) > 4 || value > 07777 {
			e = errors.New("permissions must be an octal mode such as 0644 or 0755")
		} else {
			var answer byte
			answer, _, e = protocol.request(fxSetstat, sfString(remote), sfU32(0x4), sfU32(uint32(value)))
			if e == nil && answer != fxStatus {
				e = errors.New("invalid SFTP permissions acknowledgement")
			}
		}
		summary = "Remote permissions updated"
	case "download":
		var n uint64
		n, e = protocol.download(ctx, remote, local, boolOption(req.Options, "overwrite", false), progress, func() int64 {
			if !boolOption(req.Options, "safeExport", false) {
				return -1
			}
			n, err := intOption(req.Options, "exportSize", -2, 0, 2<<30)
			if err != nil {
				return -2
			}
			return int64(n)
		}())
		data["bytesTransferred"] = n
		summary = "SFTP download completed"
	case "upload":
		var n uint64
		if boolOption(req.Options, "stagedUpload", false) {
			n, e = protocol.uploadStaged(ctx, local, remote, boolOption(req.Options, "overwrite", false), progress)
		} else {
			n, e = protocol.upload(ctx, local, remote, boolOption(req.Options, "overwrite", false), progress)
		}
		data["bytesTransferred"] = n
		summary = "SFTP upload completed"
	}
	// Session.Close may turn a blocked read into EOF after the user cancels.
	// Preserve the cancellation reason rather than reporting an unrelated
	// transport error, or a late success after the browser request closed.
	if err := ctx.Err(); err != nil {
		return model.Result{Summary: "SFTP operation could not complete", Data: data}, err
	}
	if e != nil {
		return model.Result{Summary: "SFTP operation could not complete", Data: data}, e
	}
	data["elapsedMs"] = time.Since(started).Milliseconds()
	return model.Result{Summary: summary, Data: data}, nil
}
