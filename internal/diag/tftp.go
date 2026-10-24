package diag

// TFTP RRQ/WRQ client: RFC 1350 + RFC 2347/2348/2349 negotiation.
// TFTP has no authentication or encryption; intended for trusted LANs.
import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/model"
)

const (
	tftpReadRequest  = 1
	tftpWriteRequest = 2
	tftpData         = 3
	tftpAck          = 4
	tftpError        = 5
	tftpOACK         = 6
)

func tf16(x uint16) []byte { return []byte{byte(x >> 8), byte(x)} }
func tfPacket(op uint16, parts ...[]byte) []byte {
	b := tf16(op)
	for _, p := range parts {
		b = append(b, p...)
	}
	return b
}
func tfRequest(op uint16, filename string, block int, timeout int, size int64) []byte {
	b := tfPacket(op, []byte(filename), []byte{0}, []byte("octet"), []byte{0})
	for _, kv := range [][2]string{{"blksize", strconv.Itoa(block)}, {"timeout", strconv.Itoa(timeout)}, {"tsize", strconv.FormatInt(size, 10)}} {
		b = append(b, kv[0]...)
		b = append(b, 0)
		b = append(b, kv[1]...)
		b = append(b, 0)
	}
	return b
}
func tfOptions(data []byte) (map[string]string, error) {
	parts := bytes.Split(data, []byte{0})
	if len(parts) < 3 || len(parts)%2 != 1 || len(parts[len(parts)-1]) != 0 {
		return nil, errors.New("invalid TFTP option acknowledgement")
	}
	result := make(map[string]string)
	for i := 0; i < len(parts)-1; i += 2 {
		result[strings.ToLower(string(parts[i]))] = string(parts[i+1])
	}
	return result, nil
}
func tfRemoteError(packet []byte) error {
	if len(packet) < 5 {
		return errors.New("TFTP server returned invalid error packet")
	}
	message := strings.TrimSpace(strings.TrimRight(string(packet[4:]), "\x00"))
	if len(message) > 256 {
		message = message[:256]
	}
	return fmt.Errorf("TFTP server error %d: %s", binary.BigEndian.Uint16(packet[2:4]), message)
}
func tftpTest(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	host, e := validateHost(req.Target)
	if e != nil {
		return model.Result{}, e
	}
	port, e := intOption(req.Options, "port", 69, 1, 65535)
	if e != nil {
		return model.Result{}, e
	}
	block, e := intOption(req.Options, "blockSize", 1428, 8, 65464)
	if e != nil {
		return model.Result{}, e
	}
	timeout, e := intOption(req.Options, "timeoutSec", 3, 1, 15)
	if e != nil {
		return model.Result{}, e
	}
	retries, e := intOption(req.Options, "retries", 5, 1, 20)
	if e != nil {
		return model.Result{}, e
	}
	operation := stringOption(req.Options, "operation", "download")
	if operation != "download" && operation != "upload" {
		return model.Result{}, errors.New("choose TFTP upload or download")
	}
	remote := stringOption(req.Options, "remotePath", "")
	local := stringOption(req.Options, "localPath", "")
	if remote == "" || local == "" || len(remote) > 1024 || len(local) > 4096 || strings.ContainsAny(remote, "\x00\r\n") {
		return model.Result{}, errors.New("enter valid local and remote file paths")
	}
	overwrite := boolOption(req.Options, "overwrite", false)
	var localFile *os.File
	var size int64
	if operation == "upload" {
		localFile, e = os.Open(local)
		if e != nil {
			return model.Result{}, e
		}
		defer localFile.Close()
		stat, err := localFile.Stat()
		if err != nil {
			return model.Result{}, err
		}
		if !stat.Mode().IsRegular() {
			return model.Result{}, errors.New("select a regular local file")
		}
		size = stat.Size()
	} else if !overwrite {
		_, err := os.Stat(local)
		if err == nil {
			return model.Result{}, errors.New("local file already exists; enable Overwrite to replace it")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return model.Result{}, err
		}
	}
	endpoint, e := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(port)))
	if e != nil {
		return model.Result{}, e
	}
	if endpoint.IP == nil {
		return model.Result{}, errors.New("unable to resolve TFTP server")
	}
	network := "udp4"
	if endpoint.IP.To4() == nil {
		network = "udp6"
	}
	sock, e := net.ListenUDP(network, nil)
	if e != nil {
		return model.Result{}, e
	}
	defer sock.Close()
	done := context.AfterFunc(ctx, func() { _ = sock.Close() })
	defer done()
	base := tfRequest(tftpReadRequest, remote, block, timeout, 0)
	if operation == "upload" {
		base = tfRequest(tftpWriteRequest, remote, block, timeout, size)
	}
	data := map[string]any{"protocol": "TFTP", "server": host, "port": port, "operation": operation, "remotePath": remote, "localPath": local, "requestedBlockSize": block, "timeoutSeconds": timeout, "retries": retries}
	started := time.Now()
	total := int64(0)
	negotiated := 512
	var server *net.UDPAddr
	pending := base
	destination := endpoint
	buf := make([]byte, 65540)
	send := func(packet []byte) error { _, e := sock.WriteToUDP(packet, destination); return e }
	receive := func() ([]byte, error) {
		attempts := 0
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if e := send(pending); e != nil {
				return nil, e
			}
			for {
				_ = sock.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
				n, source, e := sock.ReadFromUDP(buf)
				if ne, ok := e.(net.Error); ok && ne.Timeout() {
					attempts++
					if attempts > retries {
						return nil, fmt.Errorf("TFTP server did not respond after %d retries", retries)
					}
					break
				}
				if e != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					return nil, e
				}
				if source.IP.To16() == nil || !source.IP.Equal(endpoint.IP) {
					continue
				}
				if server != nil && source.Port != server.Port {
					_, _ = sock.WriteToUDP(tfPacket(tftpError, tf16(5), []byte("Unknown transfer ID"), []byte{0}), source)
					continue
				}
				if n < 4 {
					continue
				}
				op := binary.BigEndian.Uint16(buf[:2])
				if op != tftpData && op != tftpAck && op != tftpError && op != tftpOACK {
					continue
				}
				if server == nil {
					server = source
					destination = source
				}
				return append([]byte(nil), buf[:n]...), nil
			}
		}
	}
	// For reliable stop-and-wait, pending is always the last unacknowledged
	// request/ack/data packet and is retransmitted after timeout.
	var output *os.File
	var tempName string
	if operation == "download" {
		output, e = os.CreateTemp(filepath.Dir(local), ".ntk-tftp-*.part")
		if e != nil {
			return model.Result{}, e
		}
		tempName = output.Name()
		defer func() { _ = output.Close(); _ = os.Remove(tempName) }()
	}
	// The initial response is OACK, the first DATA packet, or ACK(0).
	packet, e := receive()
	if e != nil {
		return model.Result{}, e
	}
	lastProgress := time.Time{}
	report := func() {
		if time.Since(lastProgress) < 350*time.Millisecond {
			return
		}
		lastProgress = time.Now()
		event(emit, "transfer", "info", fmt.Sprintf("TFTP transferred %d bytes", total), map[string]any{"type": "transfer-progress", "bytes": total, "totalBytes": size})
	}
	if binary.BigEndian.Uint16(packet[:2]) == tftpError {
		return model.Result{}, tfRemoteError(packet)
	}
	if binary.BigEndian.Uint16(packet[:2]) == tftpOACK {
		opts, err := tfOptions(packet[2:])
		if err != nil {
			return model.Result{}, err
		}
		if n, err := strconv.Atoi(opts["blksize"]); err == nil {
			if n < 8 || n > block {
				return model.Result{}, errors.New("server returned an invalid TFTP block size")
			}
			negotiated = n
		}
		if ts, err := strconv.ParseInt(opts["tsize"], 10, 64); err == nil && ts >= 0 {
			data["declaredSize"] = ts
			if operation == "download" {
				size = ts
			}
		}
		if operation == "download" {
			pending = tfPacket(tftpAck, tf16(0))
			packet, e = receive()
			if e != nil {
				return model.Result{}, e
			}
		} else {
			pending = nil
			packet = tfPacket(tftpAck, tf16(0))
		}
	} else {
		data["optionNegotiation"] = "server did not negotiate options; using 512-byte blocks"
	}
	data["negotiatedBlockSize"] = negotiated
	if operation == "download" {
		expected := uint16(1)
		for {
			if ctx.Err() != nil {
				return model.Result{}, ctx.Err()
			}
			if len(packet) < 4 {
				return model.Result{}, errors.New("short TFTP packet")
			}
			op := binary.BigEndian.Uint16(packet[:2])
			if op == tftpError {
				return model.Result{}, tfRemoteError(packet)
			}
			if op != tftpData {
				return model.Result{}, fmt.Errorf("unexpected TFTP packet opcode %d", op)
			}
			seq := binary.BigEndian.Uint16(packet[2:4])
			if seq != expected {
				if seq == expected-1 {
					pending = tfPacket(tftpAck, tf16(seq))
					packet, e = receive()
					if e != nil {
						return model.Result{}, e
					}
					continue
				}
				return model.Result{}, fmt.Errorf("out-of-sequence TFTP packet %d, expected %d", seq, expected)
			}
			chunk := packet[4:]
			if len(chunk) > negotiated {
				return model.Result{}, errors.New("TFTP data block exceeds negotiated size")
			}
			if _, e = output.Write(chunk); e != nil {
				return model.Result{}, e
			}
			total += int64(len(chunk))
			report()
			pending = tfPacket(tftpAck, tf16(expected))
			if len(chunk) < negotiated {
				if e = send(pending); e != nil {
					return model.Result{}, e
				}
				break
			}
			expected++
			packet, e = receive()
			if e != nil {
				return model.Result{}, e
			}
		}
		if e = output.Sync(); e != nil {
			return model.Result{}, e
		}
		if e = output.Close(); e != nil {
			return model.Result{}, e
		}
		if overwrite {
			if e = os.Remove(local); e != nil && !os.IsNotExist(e) {
				return model.Result{}, e
			}
		}
		if e = os.Rename(tempName, local); e != nil {
			return model.Result{}, e
		}
	} else {
		if len(packet) < 4 || binary.BigEndian.Uint16(packet[:2]) != tftpAck || binary.BigEndian.Uint16(packet[2:4]) != 0 {
			return model.Result{}, errors.New("TFTP upload did not receive an ACK 0 or option acknowledgement")
		}
		next := uint16(1)
		chunk := make([]byte, negotiated)
		for {
			if ctx.Err() != nil {
				return model.Result{}, ctx.Err()
			}
			n, err := io.ReadFull(localFile, chunk)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return model.Result{}, err
			}
			pending = tfPacket(tftpData, tf16(next), chunk[:n])
			packet, e = receive()
			if e != nil {
				return model.Result{}, e
			}
			if len(packet) < 4 {
				return model.Result{}, errors.New("short TFTP acknowledgement")
			}
			if binary.BigEndian.Uint16(packet[:2]) == tftpError {
				return model.Result{}, tfRemoteError(packet)
			}
			if binary.BigEndian.Uint16(packet[:2]) != tftpAck || binary.BigEndian.Uint16(packet[2:4]) != next {
				return model.Result{}, fmt.Errorf("unexpected TFTP acknowledgement for block %d", next)
			}
			total += int64(n)
			report()
			if n < negotiated {
				break
			}
			next++
		}
	}
	data["bytesTransferred"] = total
	data["elapsedMs"] = time.Since(started).Milliseconds()
	return model.Result{Summary: fmt.Sprintf("TFTP %s completed: %d bytes", operation, total), Data: data}, nil
}
