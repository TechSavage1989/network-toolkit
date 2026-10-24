package diag

// FTP client supporting passive transfers, directory browsing, explicit and
// implicit TLS (FTPS), with certificate validation or SHA-256 pinning.
import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"networktoolkit/internal/model"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ftpClient struct {
	conn    net.Conn
	rd      *bufio.Reader
	host    string
	port    int
	timeout time.Duration
	secure  bool
	protect bool
	tlsCfg  *tls.Config
}
type ftpFileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	Modified string `json:"modified,omitempty"`
}

func (f *ftpClient) reply() (int, string, error) {
	if f.conn != nil {
		_ = f.conn.SetDeadline(time.Now().Add(f.timeout))
	}
	line, e := f.rd.ReadString('\n')
	if e != nil {
		return 0, "", e
	}
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return 0, "", errors.New("malformed FTP reply")
	}
	code, e := strconv.Atoi(line[:3])
	if e != nil {
		return 0, "", errors.New("malformed FTP status code")
	}
	if len(line) > 3 && line[3] == '-' {
		lines := []string{line}
		for i := 0; i < 100; i++ {
			next, x := f.rd.ReadString('\n')
			if x != nil {
				return 0, "", x
			}
			next = strings.TrimSpace(next)
			lines = append(lines, next)
			if strings.HasPrefix(next, fmt.Sprintf("%03d ", code)) {
				return code, strings.Join(lines, "\n"), nil
			}
		}
		return 0, "", errors.New("FTP reply too long")
	}
	return code, line, nil
}
func (f *ftpClient) command(format string, args ...any) (int, string, error) {
	cmd := fmt.Sprintf(format, args...)
	if strings.ContainsAny(cmd, "\r\n") {
		return 0, "", errors.New("FTP command contains a line break")
	}
	if f.conn != nil {
		_ = f.conn.SetDeadline(time.Now().Add(f.timeout))
	}
	if _, e := fmt.Fprintf(f.conn, "%s\r\n", cmd); e != nil {
		return 0, "", e
	}
	return f.reply()
}
func (f *ftpClient) must(want int, format string, args ...any) (string, error) {
	code, msg, e := f.command(format, args...)
	if e != nil {
		return "", e
	}
	if code != want {
		return "", fmt.Errorf("FTP %s: %s", strings.Fields(format)[0], msg)
	}
	return msg, nil
}
func (f *ftpClient) tlsHandshake() error {
	wrapped := tls.Client(f.conn, f.tlsCfg)
	_ = wrapped.SetDeadline(time.Now().Add(f.timeout))
	if e := wrapped.Handshake(); e != nil {
		return e
	}
	f.conn = wrapped
	f.rd = bufio.NewReader(wrapped)
	f.secure = true
	return nil
}
func openFTP(ctx context.Context, host string, port int, security, username, password, pin string, timeout time.Duration, inspect bool) (*ftpClient, string, error) {
	if username == "" && !inspect {
		return nil, "", errors.New("enter an FTP username")
	}
	if username != "" && password == "" && !inspect {
		return nil, "", errors.New("enter an FTP password")
	}
	if security != "plain" && security != "explicit" && security != "implicit" {
		return nil, "", errors.New("choose FTP, explicit FTPS or implicit FTPS")
	}
	if inspect && security == "plain" {
		return nil, "", errors.New("select an FTPS security mode to inspect a certificate")
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, e := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", address)
	if e != nil {
		return nil, "", e
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	_ = stop
	f := &ftpClient{conn: conn, rd: bufio.NewReader(conn), host: host, port: port, timeout: timeout}
	var certHash string
	f.tlsCfg = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if inspect || pin != "" {
		f.tlsCfg.InsecureSkipVerify = true // certificate is either inspected or checked against exact pinned SHA-256
		f.tlsCfg.VerifyPeerCertificate = func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return errors.New("FTPS peer presented no certificate")
			}
			sum := sha256.Sum256(raw[0])
			certHash = "SHA256:" + strings.ToUpper(hex.EncodeToString(sum[:]))
			if inspect {
				return nil
			}
			want := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(pin), "SHA256:"))
			if subtle.ConstantTimeCompare([]byte(want), []byte(strings.TrimPrefix(certHash, "SHA256:"))) != 1 {
				return fmt.Errorf("FTPS certificate fingerprint mismatch: presented %s", certHash)
			}
			return nil
		}
	}
	if security == "implicit" {
		if e = f.tlsHandshake(); e != nil {
			conn.Close()
			return nil, certHash, e
		}
	}
	code, msg, e := f.reply()
	if e != nil {
		conn.Close()
		return nil, certHash, e
	}
	if code != 220 {
		conn.Close()
		return nil, certHash, fmt.Errorf("FTP server greeting: %s", msg)
	}
	if security == "explicit" {
		if _, e = f.must(234, "AUTH TLS"); e != nil {
			conn.Close()
			return nil, certHash, e
		}
		if e = f.tlsHandshake(); e != nil {
			conn.Close()
			return nil, certHash, e
		}
	}
	if inspect {
		conn.Close()
		return nil, certHash, nil
	}
	if f.secure {
		if _, e = f.must(200, "PBSZ 0"); e != nil {
			conn.Close()
			return nil, certHash, e
		}
		if _, e = f.must(200, "PROT P"); e != nil {
			conn.Close()
			return nil, certHash, e
		}
		f.protect = true
	}
	code, msg, e = f.command("USER %s", username)
	if e != nil {
		conn.Close()
		return nil, certHash, e
	}
	if code == 331 {
		code, msg, e = f.command("PASS %s", password)
		if e != nil {
			conn.Close()
			return nil, certHash, e
		}
	}
	if code != 230 {
		conn.Close()
		return nil, certHash, fmt.Errorf("FTP login failed: %s", msg)
	}
	if _, e = f.must(200, "TYPE I"); e != nil {
		conn.Close()
		return nil, certHash, e
	}
	return f, certHash, nil
}
func (f *ftpClient) close() {
	if f.conn != nil {
		_, _, _ = f.command("QUIT")
		_ = f.conn.Close()
	}
}

var ftpEPSV = regexp.MustCompile(`\(\|\|\|(\d+)\|\)`)
var ftpPASV = regexp.MustCompile(`(\d+),(\d+),(\d+),(\d+),(\d+),(\d+)`)

func (f *ftpClient) openData() (net.Conn, error) {
	var port int
	code, msg, e := f.command("EPSV")
	if e != nil {
		return nil, e
	}
	if code == 229 {
		m := ftpEPSV.FindStringSubmatch(msg)
		if m == nil {
			return nil, errors.New("malformed EPSV response")
		}
		port, _ = strconv.Atoi(m[1])
	} else {
		code, msg, e = f.command("PASV")
		if e != nil {
			return nil, e
		}
		if code != 227 {
			return nil, fmt.Errorf("FTP server does not support passive mode: %s", msg)
		}
		m := ftpPASV.FindStringSubmatch(msg)
		if m == nil {
			return nil, errors.New("malformed PASV response")
		}
		hi, _ := strconv.Atoi(m[5])
		lo, _ := strconv.Atoi(m[6])
		port = hi*256 + lo
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("invalid FTP data port")
	}
	// Never allow untrusted FTP PASV replies to redirect a client to a different host.
	c, e := net.DialTimeout("tcp", net.JoinHostPort(f.host, strconv.Itoa(port)), f.timeout)
	if e != nil {
		return nil, e
	}
	_ = c.SetDeadline(time.Now().Add(maxFTPTimeout(f.timeout)))
	return c, nil
}
func maxFTPTimeout(t time.Duration) time.Duration {
	if t < 2*time.Minute {
		return 2 * time.Minute
	}
	return t
}
func (f *ftpClient) dataCommand(cmd string, writer io.Writer, reader io.Reader) (int64, error) {
	data, e := f.openData()
	if e != nil {
		return 0, e
	}
	code, msg, e := f.command("%s", cmd)
	if e != nil {
		data.Close()
		return 0, e
	}
	if code != 150 && code != 125 {
		data.Close()
		return 0, fmt.Errorf("FTP data command: %s", msg)
	}
	if f.protect {
		wrapped := tls.Client(data, f.tlsCfg)
		if e = wrapped.Handshake(); e != nil {
			data.Close()
			return 0, e
		}
		data = wrapped
	}
	var n int64
	if writer != nil {
		n, e = io.Copy(writer, io.LimitReader(data, 64<<30))
	} else if reader != nil {
		n, e = io.Copy(data, reader)
	}
	_ = data.Close()
	if e != nil {
		return n, e
	}
	code, msg, e = f.reply()
	if e != nil {
		return n, e
	}
	if code != 226 && code != 250 {
		return n, fmt.Errorf("FTP transfer incomplete: %s", msg)
	}
	return n, nil
}
func (f *ftpClient) listing(dir string) ([]ftpFileInfo, error) {
	if dir == "" {
		dir = "."
	}
	var b strings.Builder
	_, e := f.dataCommand("MLSD "+dir, &b, nil)
	if e != nil {
		b.Reset()
		_, e = f.dataCommand("LIST "+dir, &b, nil)
		if e != nil {
			return nil, e
		}
		return ftpParseLIST(b.String(), dir), nil
	}
	return ftpParseMLSD(b.String(), dir), nil
}
func ftpParseMLSD(data, base string) []ftpFileInfo {
	result := []ftpFileInfo{}
	for _, line := range strings.Split(strings.ReplaceAll(data, "\r", ""), "\n") {
		ix := strings.Index(line, " ")
		if ix < 0 {
			continue
		}
		facts, name := line[:ix], strings.TrimSpace(line[ix+1:])
		if name == "" || name == "." || name == ".." {
			continue
		}
		entry := ftpFileInfo{Name: name, Path: path.Join(base, name)}
		for _, field := range strings.Split(facts, ";") {
			pair := strings.SplitN(field, "=", 2)
			if len(pair) != 2 {
				continue
			}
			switch strings.ToLower(pair[0]) {
			case "type":
				if pair[1] == "dir" {
					entry.Kind = "directory"
				} else {
					entry.Kind = "file"
				}
			case "size":
				entry.Size, _ = strconv.ParseInt(pair[1], 10, 64)
			case "modify":
				entry.Modified = pair[1]
			}
		}
		result = append(result, entry)
	}
	return result
}
func ftpParseLIST(data, base string) []ftpFileInfo {
	result := []ftpFileInfo{}
	for _, line := range strings.Split(strings.ReplaceAll(data, "\r", ""), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		name := strings.Join(fields[8:], " ")
		if name == "." || name == ".." {
			continue
		}
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		kind := "file"
		if strings.HasPrefix(fields[0], "d") {
			kind = "directory"
		}
		result = append(result, ftpFileInfo{Name: name, Path: path.Join(base, name), Size: size, Kind: kind})
	}
	return result
}
func ftpTest(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	host, e := validateHost(req.Target)
	if e != nil {
		return model.Result{}, e
	}
	opt := req.Options
	port, e := intOption(opt, "port", 21, 1, 65535)
	if e != nil {
		return model.Result{}, e
	}
	timeout, e := durationOption(opt, "timeoutMs", 10000, 500, 60000)
	if e != nil {
		return model.Result{}, e
	}
	operation := stringOption(opt, "operation", "list")
	security := stringOption(opt, "security", "explicit")
	remote := stringOption(opt, "remotePath", ".")
	local := stringOption(opt, "localPath", "")
	newPath := stringOption(opt, "newPath", "")
	if strings.ContainsAny(remote, "\r\n\x00") || strings.ContainsAny(newPath, "\r\n\x00") || len(remote) > 4096 || len(local) > 4096 || len(newPath) > 4096 {
		return model.Result{}, errors.New("invalid FTP filename")
	}
	if (operation == "remove" || operation == "rmdir") && !boolOption(opt, "confirmDelete", false) {
		return model.Result{}, errors.New("confirm remote deletion first")
	}
	if (operation == "upload" || operation == "download") && local == "" {
		return model.Result{}, errors.New("choose a local file")
	}
	if operation == "rename" && newPath == "" {
		return model.Result{}, errors.New("choose a destination filename")
	}
	switch operation {
	case "fingerprint", "list", "stat", "download", "upload", "mkdir", "remove", "rmdir", "rename":
	default:
		return model.Result{}, errors.New("invalid FTP operation")
	}
	username := stringOption(opt, "username", "")
	password := stringOption(opt, "password", "")
	pin := stringOption(opt, "fingerprint", "")
	f, observed, e := openFTP(ctx, host, port, security, username, password, pin, timeout, operation == "fingerprint")
	data := map[string]any{"protocol": "FTP", "server": host, "port": port, "security": security, "operation": operation, "remotePath": remote, "certificateFingerprint": observed}
	if operation == "fingerprint" && e == nil {
		return model.Result{Summary: "FTPS certificate fingerprint inspected; verify it independently", Data: data}, nil
	}
	if e != nil {
		return model.Result{}, e
	}
	defer f.close()
	start := time.Now()
	var summary string
	switch operation {
	case "list":
		var items []ftpFileInfo
		items, e = f.listing(remote)
		if e == nil {
			data["entries"] = items
			data["entryCount"] = len(items)
			summary = fmt.Sprintf("FTP directory: %d entries", len(items))
		}
	case "stat":
		var m string
		m, e = f.must(213, "SIZE %s", remote)
		if e == nil {
			raw := strings.TrimSpace(strings.TrimPrefix(m, "213"))
			data["sizeBytes"], _ = strconv.ParseInt(raw, 10, 64)
			summary = "FTP file details"
			if _, mt, x := f.command("MDTM %s", remote); x == nil {
				data["modified"] = strings.TrimSpace(strings.TrimPrefix(mt, "213"))
			}
		}
	case "mkdir":
		_, e = f.must(257, "MKD %s", remote)
		summary = "FTP folder created"
	case "remove":
		_, e = f.must(250, "DELE %s", remote)
		summary = "FTP file deleted"
	case "rmdir":
		_, e = f.must(250, "RMD %s", remote)
		summary = "FTP empty folder deleted"
	case "rename":
		if _, e = f.must(350, "RNFR %s", remote); e == nil {
			_, e = f.must(250, "RNTO %s", newPath)
		}
		summary = "FTP path renamed"
	case "download":
		if !boolOption(opt, "overwrite", false) {
			if _, x := os.Stat(local); x == nil {
				return model.Result{}, errors.New("destination already exists; enable overwrite")
			}
		}
		if _, x := os.Stat(filepath.Dir(local)); x != nil {
			return model.Result{}, x
		}
		var file *os.File
		file, e = os.CreateTemp(filepath.Dir(local), ".ntk-ftp-*")
		if e != nil {
			break
		}
		defer os.Remove(file.Name())
		var n int64
		n, e = f.dataCommand("RETR "+remote, file, nil)
		if closeErr := file.Close(); e == nil {
			e = closeErr
		}
		if e == nil {
			e = os.Rename(file.Name(), local)
		}
		data["bytesTransferred"] = n
		summary = "FTP download complete"
	case "upload":
		// FTP STOR normally truncates an existing file. Refuse an implicit
		// overwrite unless the server can confirm the destination is absent.
		if !boolOption(opt, "overwrite", false) {
			code, _, checkErr := f.command("SIZE %s", remote)
			if checkErr != nil {
				return model.Result{}, checkErr
			}
			if code == 213 {
				return model.Result{}, errors.New("remote destination already exists; enable overwrite")
			}
			if code != 550 {
				return model.Result{}, errors.New("FTP server cannot confirm the destination is absent; refusing a potentially destructive upload")
			}
		}
		var file *os.File
		file, e = os.Open(local)
		if e != nil {
			break
		}
		defer file.Close()
		var n int64
		n, e = f.dataCommand("STOR "+remote, nil, file)
		data["bytesTransferred"] = n
		summary = "FTP upload complete"
	}
	if e != nil {
		return model.Result{Summary: "FTP operation failed", Data: data}, e
	}
	data["elapsedMs"] = time.Since(start).Milliseconds()
	return model.Result{Summary: summary, Data: data}, nil
}
