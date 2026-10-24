//go:build linux

package platform

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"time"
)

func readNeighbors(parent context.Context) ([]Neighbor, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	names := neighborInterfaceNames()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW|syscall.SOCK_CLOEXEC, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("open local neighbor table: %w", err)
	}
	defer syscall.Close(fd)
	interval := syscall.NsecToTimeval((200 * time.Millisecond).Nanoseconds())
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &interval); err != nil {
		return nil, err
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &interval); err != nil {
		return nil, err
	}
	local := &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}
	if err := syscall.Bind(fd, local); err != nil {
		return nil, err
	}
	bound, err := syscall.Getsockname(fd)
	if err != nil {
		return nil, err
	}
	port, ok := bound.(*syscall.SockaddrNetlink)
	if !ok {
		return nil, errors.New("invalid local neighbor socket")
	}
	// Request the existing table from the local kernel. RTM_GETNEIGH does not
	// send ARP/ND requests; it is not RTM_NEWNEIGH or an address resolution API.
	request := make([]byte, 16+12)
	binary.NativeEndian.PutUint32(request[:4], uint32(len(request)))
	binary.NativeEndian.PutUint16(request[4:6], syscall.RTM_GETNEIGH)
	binary.NativeEndian.PutUint16(request[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_DUMP)
	binary.NativeEndian.PutUint32(request[8:12], 1)
	if err := syscall.Sendto(fd, request, 0, local); err != nil {
		return nil, fmt.Errorf("request local neighbor table: %w", err)
	}
	rows := []Neighbor{}
	buffer := make([]byte, 64*1024)
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return rows, err
		}
		n, _, flags, from, err := syscall.Recvmsg(fd, buffer, nil, 0)
		if err == syscall.EAGAIN || err == syscall.EINTR {
			continue
		}
		if err != nil {
			return rows, fmt.Errorf("read local neighbor table: %w", err)
		}
		if flags&syscall.MSG_TRUNC != 0 {
			return rows, errors.New("local neighbor message exceeded the 64 KiB limit")
		}
		sender, ok := from.(*syscall.SockaddrNetlink)
		if !ok || sender.Pid != 0 {
			return rows, errors.New("unexpected local neighbor message sender")
		}
		total += n
		if total > 8*1024*1024 {
			return rows, errors.New("local neighbor table exceeded the 8 MiB limit")
		}
		messages, err := syscall.ParseNetlinkMessage(buffer[:n])
		if err != nil {
			return rows, fmt.Errorf("parse local neighbor table: %w", err)
		}
		for _, msg := range messages {
			if msg.Header.Seq != 1 || msg.Header.Pid != port.Pid {
				return rows, errors.New("unexpected local neighbor message sequence")
			}
			if msg.Header.Flags&0x10 != 0 { // NLM_F_DUMP_INTR
				return rows, errors.New("neighbor table changed during enumeration; refresh to read it again")
			}
			switch msg.Header.Type {
			case syscall.NLMSG_DONE:
				if len(msg.Data) >= 4 && int32(binary.NativeEndian.Uint32(msg.Data[:4])) < 0 {
					return rows, errors.New("local kernel could not complete the neighbor table")
				}
				return rows, nil
			case syscall.NLMSG_ERROR:
				if len(msg.Data) < 4 {
					return rows, errors.New("truncated local neighbor error")
				}
				code := int32(binary.NativeEndian.Uint32(msg.Data[:4]))
				if code < 0 {
					return rows, fmt.Errorf("local neighbor table: %w", syscall.Errno(-int64(code)))
				}
			case syscall.RTM_NEWNEIGH:
				row, include, err := parseLinuxNeighborMessage(msg.Data, names)
				if err != nil {
					return rows, err
				}
				if include {
					if len(rows) >= maxNeighborEntries {
						return rows, fmt.Errorf("neighbor table exceeds the %d-entry limit", maxNeighborEntries)
					}
					rows = append(rows, row)
				}
			}
		}
	}
}
