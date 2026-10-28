package main

// FTP and explicit FTPS server. The control channel never exposes local paths
// outside the configured server root. Passive data sockets are per-transfer;
// no SSH shell or operating system user login is exposed.
import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ftpCertificate(dataDir string) (tls.Certificate, string, error) {
	dir := filepath.Join(dataDir, "TransferServerKeys")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return tls.Certificate{}, "", err
	}
	certFile, keyFile := filepath.Join(dir, "ftp_tls.crt"), filepath.Join(dir, "ftp_tls.key")
	cert, e := tls.LoadX509KeyPair(certFile, keyFile)
	if e != nil {
		// When either key file exists but cannot be loaded, never silently rotate
		// the certificate clients might have pinned.
		if _, x := os.Stat(certFile); x == nil {
			return tls.Certificate{}, "", fmt.Errorf("load FTPS certificate: %w", e)
		}
		if _, x := os.Stat(keyFile); x == nil {
			return tls.Certificate{}, "", fmt.Errorf("load FTPS private key: %w", e)
		}
		key, x := rsa.GenerateKey(rand.Reader, 2048)
		if x != nil {
			return tls.Certificate{}, "", x
		}
		serial := make([]byte, 16)
		if _, x = rand.Read(serial); x != nil {
			return tls.Certificate{}, "", x
		}
		serial[0] &= 0x7f
		n := new(big.Int).SetBytes(serial)
		tpl := x509.Certificate{SerialNumber: n, Subject: pkix.Name{CommonName: "Network Toolkit FTP Server"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(3, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
		der, x := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &key.PublicKey, key)
		if x != nil {
			return tls.Certificate{}, "", x
		}
		if x = os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); x != nil {
			return tls.Certificate{}, "", x
		}
		if x = os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); x != nil {
			return tls.Certificate{}, "", x
		}
		cert, x = tls.LoadX509KeyPair(certFile, keyFile)
		if x != nil {
			return tls.Certificate{}, "", x
		}
	}
	leaf, e := x509.ParseCertificate(cert.Certificate[0])
	if e != nil {
		return tls.Certificate{}, "", e
	}
	sum := sha256.Sum256(leaf.Raw)
	return cert, "SHA256:" + strings.ToUpper(hex.EncodeToString(sum[:])), nil
}

type ftpServerSession struct {
	server        *transferHost
	conn          net.Conn
	reader        *bufio.Reader
	tlsConf       *tls.Config
	secure        bool
	protectData   bool
	needsTLS      bool
	user          string
	authenticated bool
	password      string
	cwd           string
	renameFrom    string
	passive       net.Listener
	activeAddr    string
	restOffset    int64
}

func (f *ftpServerSession) response(code int, msg string) error {
	_, e := fmt.Fprintf(f.conn, "%d %s\r\n", code, msg)
	return e
}
func (f *ftpServerSession) welcome() error {
	return f.response(220, "Network Toolkit FTP server ready")
}
func (f *ftpServerSession) realPath(arg string, rootOK bool) (string, string, error) {
	arg = strings.TrimSpace(strings.ReplaceAll(arg, "\\", "/"))
	if strings.ContainsAny(arg, "\x00\r\n") {
		return "", "", errors.New("invalid pathname")
	}
	virtual := f.cwd
	if strings.HasPrefix(arg, "/") {
		virtual = path.Clean(arg)
	} else if arg != "" {
		virtual = path.Clean(path.Join(f.cwd, arg))
	}
	if virtual == "." {
		virtual = "/"
	}
	// Parent navigation is naturally clamped at FTP's virtual root.
	candidate := strings.TrimPrefix(virtual, "/")
	full, e := transferPath(f.server.root, candidate, rootOK)
	return full, virtual, e
}
func (f *ftpServerSession) dataListener() (net.Conn, error) {
	if f.passive != nil {
		l := f.passive
		f.passive = nil
		defer l.Close()
		// Use an accept deadline rather than leaving uploads hung indefinitely.
		if x, ok := l.(*net.TCPListener); ok {
			_ = x.SetDeadline(time.Now().Add(15 * time.Second))
		}
		c, e := l.Accept()
		if e != nil {
			return nil, e
		}
		// Passive connections must originate from the authenticated control peer.
		peer, _, _ := net.SplitHostPort(f.conn.RemoteAddr().String())
		src, _, _ := net.SplitHostPort(c.RemoteAddr().String())
		if !net.ParseIP(src).Equal(net.ParseIP(peer)) {
			c.Close()
			return nil, errors.New("FTP passive peer differs from control connection")
		}
		_ = c.SetDeadline(time.Now().Add(90 * time.Second))
		if f.protectData {
			wrapped := tls.Server(c, f.tlsConf)
			if e = wrapped.Handshake(); e != nil {
				c.Close()
				return nil, e
			}
			return wrapped, nil
		}
		return c, nil
	}
	if f.activeAddr != "" {
		addr := f.activeAddr
		f.activeAddr = ""
		c, e := net.DialTimeout("tcp", addr, 10*time.Second)
		if e != nil {
			return nil, e
		}
		_ = c.SetDeadline(time.Now().Add(90 * time.Second))
		if f.protectData {
			wrapped := tls.Server(c, f.tlsConf)
			if e = wrapped.Handshake(); e != nil {
				c.Close()
				return nil, e
			}
			return wrapped, nil
		}
		return c, nil
	}
	return nil, errors.New("request PASV, EPSV, PORT or EPRT before transfer")
}
func (f *ftpServerSession) closeData() {
	if f.passive != nil {
		_ = f.passive.Close()
		f.passive = nil
	}
	f.activeAddr = ""
}
func (f *ftpServerSession) preparePassive() (net.Addr, error) {
	f.closeData()
	host := f.server.bind
	if host == "" {
		host = "127.0.0.1"
	}
	// Passive sockets must be on the same adapter as the control server.
	l, e := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if e != nil {
		return nil, e
	}
	f.passive = l
	return l.Addr(), nil
}
func (f *ftpServerSession) setActive(host string, port int) error {
	if port < 1 || port > 65535 {
		return errors.New("invalid data port")
	}
	controlHost, _, _ := net.SplitHostPort(f.conn.RemoteAddr().String())
	if !net.ParseIP(controlHost).Equal(net.ParseIP(host)) {
		return errors.New("active-mode host must match the connected client")
	}
	f.closeData()
	f.activeAddr = net.JoinHostPort(host, strconv.Itoa(port))
	return nil
}
func ftpTime(info os.FileInfo) string { return info.ModTime().UTC().Format("20060102150405") }
func (f *ftpServerSession) fileListing(arg, cmd string) (string, error) {
	full, virtual, e := f.realPath(arg, true)
	if e != nil {
		return "", e
	}
	st, e := os.Stat(full)
	if e != nil {
		return "", e
	}
	var entries []os.FileInfo
	if st.IsDir() {
		entries, e = readFTPEntries(full)
	} else {
		entries = []os.FileInfo{st}
	}
	if e != nil {
		return "", e
	}
	var b strings.Builder
	for _, entry := range entries {
		name := entry.Name()
		if strings.ContainsAny(name, "\r\n") {
			continue
		}
		size := entry.Size()
		if entry.IsDir() {
			size = 0
		}
		switch cmd {
		case "NLST":
			fmt.Fprintf(&b, "%s\r\n", name)
		case "MLSD":
			fmt.Fprintf(&b, "type=%s;size=%d;modify=%s; %s\r\n", map[bool]string{true: "dir", false: "file"}[entry.IsDir()], size, ftpTime(entry), name)
		default:
			mode := "-rw-r--r--"
			if entry.IsDir() {
				mode = "drwxr-xr-x"
			}
			fmt.Fprintf(&b, "%s 1 owner group %d %s %s\r\n", mode, size, entry.ModTime().Format("Jan _2 15:04"), name)
		}
	}
	_ = virtual
	return b.String(), nil
}
func readFTPEntries(p string) ([]os.FileInfo, error) {
	raw, e := os.ReadDir(p)
	if e != nil {
		return nil, e
	}
	if len(raw) > 10000 {
		return nil, errors.New("directory contains more than 10,000 entries")
	}
	entries := make([]os.FileInfo, 0, len(raw))
	for _, item := range raw {
		info, e := item.Info()
		if e != nil || item.Type()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
			continue
		}
		entries = append(entries, info)
	}
	return entries, nil
}
func (f *ftpServerSession) transfer(op, arg string) {
	allowed := f.server.access
	isWrite := op == "STOR" || op == "APPE" || op == "STOU"
	if isWrite && allowed == "read" || !isWrite && allowed == "write" {
		_ = f.response(550, "Permission denied")
		return
	}
	full, virtual, e := f.realPath(arg, false)
	if e != nil {
		_ = f.response(550, "Invalid path: "+e.Error())
		return
	}
	if isWrite {
		if !f.server.overwrite {
			if _, e = os.Stat(full); e == nil {
				_ = f.response(553, "File exists; overwrite disabled")
				return
			}
		}
	} else {
		info, x := os.Stat(full)
		if x != nil || !info.Mode().IsRegular() {
			_ = f.response(550, "File does not exist")
			return
		}
	}
	if f.passive == nil && f.activeAddr == "" {
		_ = f.response(425, "Select PASV/EPSV or PORT/EPRT first")
		return
	}
	_ = f.response(150, "Opening binary data connection")
	data, e := f.dataListener()
	if e != nil {
		_ = f.response(425, "Cannot open data connection: "+e.Error())
		return
	}
	defer data.Close()
	f.server.activity(1)
	defer f.server.activity(-1)
	var n int64
	if isWrite {
		temp, e := os.CreateTemp(filepath.Dir(full), ".ntk-ftp-*")
		if e == nil {
			defer os.Remove(temp.Name())
			n, e = io.Copy(temp, io.LimitReader(data, (64<<30)+1))
			if n > 64<<30 && e == nil {
				e = errors.New("FTP upload exceeds 64 GiB limit")
			}
			if closeErr := temp.Close(); e == nil {
				e = closeErr
			}
			if e == nil {
				// Atomic publishing of a completed upload.
				if !f.server.overwrite {
					if _, x := os.Stat(full); x == nil {
						e = errors.New("destination already exists")
					}
				}
				if e == nil {
					e = os.Rename(temp.Name(), full)
				}
			}
		}
	} else {
		var file *os.File
		file, e = os.Open(full)
		if e == nil {
			if f.restOffset > 0 {
				_, e = file.Seek(f.restOffset, io.SeekStart)
			}
			if e == nil {
				n, e = io.Copy(data, file)
			}
			file.Close()
		}
	}
	f.restOffset = 0
	f.server.log(f.conn.RemoteAddr().String(), op, virtual, uint64(maxInt64(n, 0)), e)
	if e != nil {
		_ = f.response(451, "Transfer failed: "+e.Error())
		return
	}
	_ = f.response(226, fmt.Sprintf("Transfer complete (%d bytes)", n))
}
func maxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}
func (f *ftpServerSession) listTransfer(cmd, arg string) {
	if f.server.access == "write" {
		_ = f.response(550, "Directory reading is disabled")
		return
	}
	data, e := f.fileListing(arg, cmd)
	if e != nil {
		_ = f.response(550, "Cannot list files: "+e.Error())
		return
	}
	if f.passive == nil && f.activeAddr == "" {
		_ = f.response(425, "Choose a data connection first")
		return
	}
	_ = f.response(150, "Opening directory data connection")
	conn, e := f.dataListener()
	if e != nil {
		_ = f.response(425, "Cannot open data connection: "+e.Error())
		return
	}
	_, e = io.WriteString(conn, data)
	_ = conn.Close()
	f.server.log(f.conn.RemoteAddr().String(), cmd, arg, uint64(len(data)), e)
	if e != nil {
		_ = f.response(451, "Listing failed")
	} else {
		_ = f.response(226, "Directory transfer complete")
	}
}
func (f *ftpServerSession) handleCommand(cmd, arg string) bool {
	if !f.authenticated && cmd != "USER" && cmd != "PASS" && cmd != "AUTH" && cmd != "QUIT" && cmd != "FEAT" && cmd != "SYST" && cmd != "NOOP" && cmd != "OPTS" && cmd != "PBSZ" && cmd != "PROT" {
		_ = f.response(530, "Login required")
		return true
	}
	switch cmd {
	case "QUIT":
		_ = f.response(221, "Goodbye")
		return false
	case "NOOP":
		_ = f.response(200, "OK")
	case "SYST":
		_ = f.response(215, "UNIX Type: L8")
	case "FEAT":
		_, _ = io.WriteString(f.conn, "211-Features\r\n UTF8\r\n MLST type*;size*;modify*;\r\n EPSV\r\n PASV\r\n SIZE\r\n MDTM\r\n REST STREAM\r\n")
		if f.tlsConf != nil {
			_, _ = io.WriteString(f.conn, " AUTH TLS\r\n PBSZ\r\n PROT\r\n")
		}
		_, _ = io.WriteString(f.conn, "211 End\r\n")
	case "OPTS":
		_ = f.response(200, "UTF8 enabled")
	case "AUTH":
		if f.tlsConf == nil || !strings.EqualFold(arg, "TLS") {
			_ = f.response(502, "TLS not available")
			break
		}
		if f.secure {
			_ = f.response(503, "Already secured")
			break
		}
		_ = f.response(234, "Proceed with TLS")
		conn := tls.Server(f.conn, f.tlsConf)
		if e := conn.Handshake(); e != nil {
			return false
		}
		f.conn = conn
		f.reader = bufio.NewReader(conn)
		f.secure = true
	case "PBSZ":
		if !f.secure {
			_ = f.response(503, "TLS required")
		} else if arg != "0" {
			_ = f.response(501, "PBSZ must be 0")
		} else {
			_ = f.response(200, "PBSZ=0")
		}
	case "PROT":
		if !f.secure {
			_ = f.response(503, "TLS required")
		} else if arg == "P" {
			f.protectData = true
			_ = f.response(200, "Private data protection enabled")
		} else if arg == "C" && !f.needsTLS {
			f.protectData = false
			_ = f.response(200, "Clear data enabled")
		} else {
			_ = f.response(534, "Private data protection required")
		}
	case "USER":
		if f.needsTLS && !f.secure {
			_ = f.response(534, "Use AUTH TLS first")
			break
		}
		f.user = arg
		f.authenticated = false
		_ = f.response(331, "Password required")
	case "PASS":
		if f.needsTLS && !f.secure {
			_ = f.response(534, "TLS required")
			break
		}
		if f.user == "" {
			_ = f.response(503, "Send USER first")
			break
		}
		if f.user == f.serverUser() && f.comparePassword(arg) {
			f.authenticated = true
			_ = f.response(230, "Logged in")
			f.server.log(f.conn.RemoteAddr().String(), "LOGIN", f.user, 0, nil)
		} else {
			_ = f.response(530, "Invalid credentials")
			f.server.log(f.conn.RemoteAddr().String(), "LOGIN", f.user, 0, errors.New("invalid credentials"))
		}
	case "TYPE":
		if arg == "I" || arg == "A" {
			_ = f.response(200, "Type set")
		} else {
			_ = f.response(504, "Use binary or ASCII type")
		}
	case "MODE", "STRU":
		_ = f.response(200, "OK")
	case "PWD", "XPWD":
		_ = f.response(257, fmt.Sprintf("\"%s\"", f.cwd))
	case "CWD", "XCWD", "CDUP", "XCUP":
		if cmd == "CDUP" || cmd == "XCUP" {
			arg = ".."
		}
		full, virtual, e := f.realPath(arg, true)
		if e == nil {
			var st os.FileInfo
			st, e = os.Stat(full)
			if e == nil && !st.IsDir() {
				e = errors.New("not a directory")
			}
		}
		if e != nil {
			_ = f.response(550, "Cannot change directory: "+e.Error())
		} else {
			f.cwd = virtual
			_ = f.response(250, "Directory changed")
		}
	case "PASV", "EPSV":
		a, e := f.preparePassive()
		if e != nil {
			_ = f.response(425, "Cannot listen for data")
			break
		}
		host, portStr, _ := net.SplitHostPort(a.String())
		port, _ := strconv.Atoi(portStr)
		if cmd == "EPSV" {
			_ = f.response(229, fmt.Sprintf("Entering Extended Passive Mode (|||%d|)", port))
		} else {
			ip := net.ParseIP(host).To4()
			if ip == nil || ip.IsUnspecified() {
				ip = net.ParseIP(f.server.bind).To4()
			}
			if ip == nil || ip.IsUnspecified() {
				ip = net.ParseIP(f.conn.LocalAddr().(*net.TCPAddr).IP.String()).To4()
			}
			if ip == nil {
				_ = f.response(522, "Use EPSV for IPv6")
				break
			}
			_ = f.response(227, fmt.Sprintf("Entering Passive Mode (%s,%d,%d)", strings.ReplaceAll(ip.String(), ".", ","), port/256, port%256))
		}
	case "PORT":
		parts := strings.Split(arg, ",")
		if len(parts) != 6 {
			_ = f.response(501, "Invalid PORT command")
			break
		}
		host := strings.Join(parts[:4], ".")
		hi, e1 := strconv.Atoi(parts[4])
		lo, e2 := strconv.Atoi(parts[5])
		if e1 != nil || e2 != nil || hi < 0 || hi > 255 || lo < 0 || lo > 255 {
			_ = f.response(501, "Invalid PORT command")
			break
		}
		if e := f.setActive(host, hi*256+lo); e != nil {
			_ = f.response(501, e.Error())
		} else {
			_ = f.response(200, "Active mode accepted")
		}
	case "EPRT":
		parts := strings.Split(arg, "|")
		if len(parts) < 4 {
			_ = f.response(501, "Invalid EPRT")
			break
		}
		port, e := strconv.Atoi(parts[3])
		if e != nil {
			_ = f.response(501, "Invalid port")
			break
		}
		if e = f.setActive(parts[2], port); e != nil {
			_ = f.response(501, e.Error())
		} else {
			_ = f.response(200, "Extended active mode accepted")
		}
	case "LIST", "NLST", "MLSD":
		f.listTransfer(cmd, arg)
	case "RETR", "STOR":
		f.transfer(cmd, arg)
	case "REST":
		n, e := strconv.ParseInt(arg, 10, 64)
		if e != nil || n < 0 {
			_ = f.response(501, "Invalid restart position")
		} else {
			f.restOffset = n
			_ = f.response(350, "Restart position accepted")
		}
	case "SIZE", "MDTM":
		full, _, e := f.realPath(arg, false)
		if e != nil {
			_ = f.response(550, "Invalid pathname")
			break
		}
		stat, e := os.Stat(full)
		if e != nil || !stat.Mode().IsRegular() {
			_ = f.response(550, "File not found")
			break
		}
		if cmd == "SIZE" {
			_ = f.response(213, strconv.FormatInt(stat.Size(), 10))
		} else {
			_ = f.response(213, ftpTime(stat))
		}
	case "MKD", "XMKD", "RMD", "XRMD", "DELE":
		if f.server.access == "read" {
			_ = f.response(550, "Read-only")
			break
		}
		full, virtual, e := f.realPath(arg, false)
		if e != nil {
			_ = f.response(550, e.Error())
			break
		}
		if cmd == "MKD" || cmd == "XMKD" {
			e = os.Mkdir(full, 0700)
		} else {
			e = os.Remove(full)
		}
		f.server.log(f.conn.RemoteAddr().String(), cmd, virtual, 0, e)
		if e != nil {
			_ = f.response(550, e.Error())
		} else if cmd == "MKD" || cmd == "XMKD" {
			_ = f.response(257, fmt.Sprintf("\"%s\" created", virtual))
		} else {
			_ = f.response(250, "Operation complete")
		}
	case "RNFR":
		if f.server.access == "read" {
			_ = f.response(550, "Read-only")
			break
		}
		full, _, e := f.realPath(arg, false)
		if e != nil {
			_ = f.response(550, e.Error())
			break
		}
		if _, e = os.Stat(full); e != nil {
			_ = f.response(550, "Source not found")
			break
		}
		f.renameFrom = full
		_ = f.response(350, "Send RNTO")
	case "RNTO":
		if f.renameFrom == "" {
			_ = f.response(503, "Send RNFR first")
			break
		}
		from := f.renameFrom
		f.renameFrom = ""
		to, virtual, e := f.realPath(arg, false)
		if e == nil {
			if _, x := os.Stat(to); x == nil {
				e = errors.New("destination already exists")
			}
		}
		if e == nil {
			e = os.Rename(from, to)
		}
		f.server.log(f.conn.RemoteAddr().String(), "RENAME", virtual, 0, e)
		if e != nil {
			_ = f.response(550, e.Error())
		} else {
			_ = f.response(250, "Renamed")
		}
	default:
		_ = f.response(502, "Command not implemented")
	}
	return true
}
func (f *ftpServerSession) serverUser() string { return f.server.authUser }
func (f *ftpServerSession) comparePassword(p string) bool {
	return subtle.ConstantTimeCompare([]byte(f.server.authPassword), []byte(p)) == 1
}
func handleFTPSession(ctx context.Context, s *transferHost, c net.Conn, cert *tls.Config, requireTLS bool) {
	defer c.Close()
	s.activity(1)
	defer s.activity(-1)
	f := &ftpServerSession{server: s, conn: c, reader: bufio.NewReader(c), tlsConf: cert, needsTLS: requireTLS, cwd: "/"}
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	s.log(c.RemoteAddr().String(), "CONNECT", "", 0, nil)
	defer f.closeData()
	if f.welcome() != nil {
		return
	}
	for {
		_ = f.conn.SetDeadline(time.Now().Add(4 * time.Minute))
		line, e := f.reader.ReadString('\n')
		if e != nil {
			return
		}
		if len(line) > 8192 {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		cmd, arg, _ := strings.Cut(line, " ")
		cmd = strings.ToUpper(cmd)
		if !f.handleCommand(cmd, arg) {
			return
		}
	}
}
func startFTPServer(ctx context.Context, s *transferHost, base, username, password, security string) (net.Listener, error) {
	if username == "" || password == "" {
		return nil, errors.New("FTP server requires username and password")
	}
	if security != "plain" && security != "explicit" {
		return nil, errors.New("Choose FTP or explicit FTPS")
	}
	var tlsConf *tls.Config
	if security == "explicit" {
		cert, fp, e := ftpCertificate(base)
		if e != nil {
			return nil, e
		}
		tlsConf = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}
		s.sshFingerprint = fp // shared fingerprint field, labelled as TLS fingerprint for FTP
	}
	l, e := net.Listen("tcp", s.address)
	if e != nil {
		return nil, e
	}
	s.authUser = username
	s.authPassword = password
	go func() { <-ctx.Done(); _ = l.Close() }()
	go func() {
		defer s.servingStopped()
		defer l.Close()
		for {
			conn, e := l.Accept()
			if e != nil {
				return
			}
			go handleFTPSession(ctx, s, conn, tlsConf, security == "explicit")
		}
	}()
	return l, nil
}
