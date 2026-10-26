package main

// RFC 1350 server with RFC 2347/2348/2349 option negotiation, per-request
// transfer sockets, retransmissions, 16-bit block rollover and atomic uploads.
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
)

const (
	tftpRRQ   = 1
	tftpWRQ   = 2
	tftpDATA  = 3
	tftpACK   = 4
	tftpERROR = 5
	tftpOACK  = 6
)

func tf16s(n uint16) []byte { return []byte{byte(n >> 8), byte(n)} }
func tfMessage(t uint16, data ...[]byte) []byte {
	p := tf16s(t)
	for _, b := range data {
		p = append(p, b...)
	}
	return p
}
func tfErrPacket(code uint16, why string) []byte {
	if len(why) > 160 {
		why = why[:160]
	}
	return tfMessage(tftpERROR, tf16s(code), []byte(why), []byte{0})
}
func parseTFTPRequest(p []byte) (uint16, string, map[string]string, error) {
	if len(p) < 6 || len(p) > 1024 {
		return 0, "", nil, errors.New("invalid TFTP request length")
	}
	op := binary.BigEndian.Uint16(p[:2])
	if op != tftpRRQ && op != tftpWRQ {
		return 0, "", nil, errors.New("unsupported TFTP request")
	}
	if p[len(p)-1] != 0 {
		return 0, "", nil, errors.New("malformed TFTP request")
	}
	fields := bytes.Split(p[2:len(p)-1], []byte{0})
	if len(fields) < 2 || (len(fields)-2)%2 != 0 {
		return 0, "", nil, errors.New("invalid TFTP options")
	}
	filename := string(fields[0])
	if filename == "" || len(filename) > 512 {
		return 0, "", nil, errors.New("invalid filename")
	}
	if !strings.EqualFold(string(fields[1]), "octet") {
		return 0, "", nil, errors.New("only octet (binary) mode is supported")
	}
	options := make(map[string]string)
	for i := 2; i < len(fields); i += 2 {
		key := strings.ToLower(string(fields[i]))
		if key == "" || len(key) > 32 || len(fields[i+1]) > 64 {
			return 0, "", nil, errors.New("invalid option")
		}
		options[key] = string(fields[i+1])
	}
	return op, filename, options, nil
}
func tftpOptions(want map[string]string, size int64) (int, time.Duration, []byte, error) {
	block := 512
	timeout := 3 * time.Second
	resp := []byte{}
	add := func(k, v string) {
		resp = append(resp, []byte(k)...)
		resp = append(resp, 0)
		resp = append(resp, []byte(v)...)
		resp = append(resp, 0)
	}
	if v, ok := want["blksize"]; ok {
		n, e := strconv.Atoi(v)
		if e != nil || n < 8 || n > 65464 {
			return 0, 0, nil, errors.New("invalid requested TFTP block size")
		}
		if n > 32768 {
			n = 32768
		}
		block = n
		add("blksize", strconv.Itoa(n))
	}
	if v, ok := want["timeout"]; ok {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 15 {
			return 0, 0, nil, errors.New("invalid requested TFTP timeout")
		}
		timeout = time.Duration(n) * time.Second
		add("timeout", strconv.Itoa(n))
	}
	if v, ok := want["tsize"]; ok {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil || n < 0 {
			return 0, 0, nil, errors.New("invalid TFTP transfer size")
		}
		if size >= 0 {
			add("tsize", strconv.FormatInt(size, 10))
		} else {
			add("tsize", v)
		}
	}
	return block, timeout, resp, nil
}
func startTFTPServer(ctx context.Context, s *transferHost) (*net.UDPConn, error) {
	addr, e := net.ResolveUDPAddr("udp", s.address)
	if e != nil {
		return nil, e
	}
	conn, e := net.ListenUDP("udp", addr)
	if e != nil {
		return nil, fmt.Errorf("listen on UDP %s: %w", s.address, e)
	}
	go func() {
		defer conn.Close()
		buf := make([]byte, 2048)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			n, peer, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				return
			}
			if n < 2 {
				continue
			}
			packet := append([]byte(nil), buf[:n]...)
			if binary.BigEndian.Uint16(packet[:2]) != tftpRRQ && binary.BigEndian.Uint16(packet[:2]) != tftpWRQ {
				continue
			}
			s.mu.Lock()
			active := s.active
			s.mu.Unlock()
			if active >= 16 {
				_, _ = conn.WriteToUDP(tfErrPacket(0, "Server is busy"), peer)
				continue
			}
			go handleTFTPRequest(ctx, s, peer, packet, addr.IP)
		}
	}()
	return conn, nil
}
func handleTFTPRequest(ctx context.Context, s *transferHost, peer *net.UDPAddr, packet []byte, bindIP net.IP) {
	s.activity(1)
	defer s.activity(-1)
	op, filename, opts, err := parseTFTPRequest(packet)
	if err != nil {
		sendInitialTFTPError(bindIP, peer, 4, err.Error())
		s.log(peer.String(), "request", filename, 0, err)
		return
	}
	action := "download"
	if op == tftpWRQ {
		action = "upload"
	}
	s.log(peer.String(),"REQUEST",filename,0,nil)
 if (op == tftpRRQ && s.access == "write") || (op == tftpWRQ && s.access == "read") {
		err = errors.New("operation disabled by server permissions")
		sendInitialTFTPError(bindIP, peer, 2, err.Error())
		s.log(peer.String(), action, filename, 0, err)
		return
	}
	full, err := transferPath(s.root, filename, false)
	if err != nil {
		sendInitialTFTPError(bindIP, peer, 2, err.Error())
		s.log(peer.String(), action, filename, 0, err)
		return
	}
	var f *os.File
	var temp string
	var size int64 = -1
	if op == tftpRRQ {
		f, err = os.Open(full)
		if err == nil {
			var stat os.FileInfo
			stat, err = f.Stat()
			if err == nil && !stat.Mode().IsRegular() {
				err = errors.New("requested path is not a regular file")
			}
			if err == nil {
				size = stat.Size()
			}
		}
	} else {
		var stat os.FileInfo
		stat, err = os.Stat(full)
		if err == nil && (!s.overwrite || stat.IsDir()) {
			err = errors.New("destination already exists; overwrite is disabled or path is a directory")
		}
		if os.IsNotExist(err) {
			err = nil
		}
		if err == nil {
			f, err = os.CreateTemp(filepath.Dir(full), ".ntk-tftp-*.part")
			if err == nil {
				temp = f.Name()
			}
		}
	}
	if err != nil {
		if f != nil {
			_ = f.Close()
		}
		sendInitialTFTPError(bindIP, peer, 2, err.Error())
		s.log(peer.String(), action, filename, 0, err)
		return
	}
	defer f.Close()
	if temp != "" {
		defer os.Remove(temp)
	}
	var block int
	var timeout time.Duration
	var ackOpts []byte
	block, timeout, ackOpts, err = tftpOptions(opts, size)
	if err != nil {
		sendInitialTFTPError(bindIP, peer, 8, err.Error())
		s.log(peer.String(), action, filename, 0, err)
		return
	}
	socket, e := net.DialUDP("udp", &net.UDPAddr{IP: bindIP, Port: 0}, peer)
	if e != nil {
		s.log(peer.String(), action, filename, 0, e)
		return
	}
	defer socket.Close()
	cancel := context.AfterFunc(ctx, func() { _ = socket.Close() })
	defer cancel()
	var transferred uint64
	if op == tftpRRQ {
		transferred, err = tftpServeRead(ctx, socket, f, block, timeout, ackOpts)
	} else {
		transferred, err = tftpServeWrite(ctx, socket, f, block, timeout, ackOpts, func() error {
			if err := f.Sync(); err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			return atomicTransferPublish(temp, full, s.overwrite)
		})
	}
	s.log(peer.String(), action, filename, transferred, err)
}
func sendInitialTFTPError(ip net.IP, peer *net.UDPAddr, code uint16, why string) {
	sock, e := net.DialUDP("udp", &net.UDPAddr{IP: ip, Port: 0}, peer)
	if e == nil {
		_, _ = sock.Write(tfErrPacket(code, why))
		_ = sock.Close()
	}
}
func tftpExchange(ctx context.Context, s *net.UDPConn, request []byte, timeout time.Duration, valid func([]byte) bool) ([]byte, error) {
	buf := make([]byte, 65540)
	for retry := 0; retry <= 5; retry++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, e := s.Write(request); e != nil {
			return nil, e
		}
		for {
			_ = s.SetReadDeadline(time.Now().Add(timeout))
			n, e := s.Read(buf)
			if ne, ok := e.(net.Error); ok && ne.Timeout() {
				break
			}
			if e != nil {
				return nil, e
			}
			if n < 4 {
				continue
			}
			packet := append([]byte(nil), buf[:n]...)
			if binary.BigEndian.Uint16(packet[:2]) == tftpERROR {
				return nil, fmt.Errorf("TFTP peer error: %s", strings.TrimRight(string(packet[4:]), "\x00"))
			}
			if valid(packet) {
				return packet, nil
			}
		}
	}
	return nil, errors.New("TFTP transfer timed out after retries")
}
func tftpServeRead(ctx context.Context, sock *net.UDPConn, f *os.File, block int, timeout time.Duration, opts []byte) (uint64, error) {
	var total uint64
	if len(opts) > 0 {
		_, e := tftpExchange(ctx, sock, tfMessage(tftpOACK, opts), timeout, func(p []byte) bool {
			return len(p) == 4 && binary.BigEndian.Uint16(p[:2]) == tftpACK && binary.BigEndian.Uint16(p[2:]) == 0
		})
		if e != nil {
			return 0, e
		}
	}
	buf := make([]byte, block)
	for number := uint16(1); ; number++ {
		n, err := io.ReadFull(f, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return total, err
		}
		_, e := tftpExchange(ctx, sock, tfMessage(tftpDATA, tf16s(number), buf[:n]), timeout, func(p []byte) bool {
			return len(p) == 4 && binary.BigEndian.Uint16(p[:2]) == tftpACK && binary.BigEndian.Uint16(p[2:]) == number
		})
		if e != nil {
			return total, e
		}
		total += uint64(n)
		if n < block {
			return total, nil
		}
	}
}
func tftpServeWrite(ctx context.Context, sock *net.UDPConn, f *os.File, block int, timeout time.Duration, opts []byte, finish func() error) (uint64, error) {
	var ack []byte
	if len(opts) > 0 {
		ack = tfMessage(tftpOACK, opts)
	} else {
		ack = tfMessage(tftpACK, tf16s(0))
	}
	var total uint64
	for number := uint16(1); ; number++ {
		payload, e := tftpExchange(ctx, sock, ack, timeout, func(p []byte) bool {
			return len(p) >= 4 && binary.BigEndian.Uint16(p[:2]) == tftpDATA && binary.BigEndian.Uint16(p[2:4]) == number && len(p) <= block+4
		})
		if e != nil {
			return total, e
		}
		n := len(payload) - 4
		if total+uint64(n) > 4<<30 {
			return total, errors.New("upload exceeds maximum file size")
		}
		if _, e = f.Write(payload[4:]); e != nil {
			return total, e
		}
		total += uint64(n)
		ack = tfMessage(tftpACK, tf16s(number))
		if n < block {
			if e := finish(); e != nil {
				_, _ = sock.Write(tfErrPacket(2, e.Error()))
				return total, e
			}
			_, _ = sock.Write(ack)
			// A final ACK can be lost. Briefly answer a duplicate final DATA.
			_ = sock.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			b := make([]byte, block+4)
			if rn, err := sock.Read(b); err == nil && rn >= 4 && binary.BigEndian.Uint16(b[:2]) == tftpDATA && binary.BigEndian.Uint16(b[2:4]) == number {
				_, _ = sock.Write(ack)
			}
			return total, nil
		}
	}
}
func atomicTransferPublish(temp, target string, overwrite bool) error {
	if !overwrite {
		if err := os.Link(temp, target); err == nil {
			return os.Remove(temp)
		}
		if _, err := os.Stat(target); err == nil {
			return errors.New("destination already exists; overwrite disabled")
		}
		// Hard links are unavailable on some filesystems. O_EXCL prevents
		// accidentally replacing a file created during the transfer.
		input, e := os.Open(temp)
		if e != nil {
			return e
		}
		defer input.Close()
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, input)
		if e != nil {
			_ = out.Close()
			_ = os.Remove(target)
			return e
		}
		if e = out.Sync(); e != nil {
			_ = out.Close()
			_ = os.Remove(target)
			return e
		}
		return out.Close()
	}
	// Windows rename fails if target exists, so explicitly replace with a
	// rollback copy rather than deleting a valid destination on failure.
	targetExists := false
	if _, e := os.Stat(target); e == nil {
		targetExists = true
	}
	if !targetExists {
		return os.Rename(temp, target)
	}
	backup := target + ".ntk-backup-" + randomTransferID()
	if e := os.Rename(target, backup); e != nil {
		return e
	}
	if e := os.Rename(temp, target); e != nil {
		_ = os.Rename(backup, target)
		return e
	}
	_ = os.Remove(backup)
	return nil
}
