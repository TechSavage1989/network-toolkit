package terminal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// These fixtures stop before NEWKEYS, so their SSH packets are deliberately
// unencrypted. They exercise the real client's existing GEX validation without
// changing the library or accepting a malformed group in a successful session.
type fixtureKexInit struct {
	Cookie                  [16]byte `sshtype:"20"`
	Kex                     []string
	HostKeys                []string
	CiphersClientServer     []string
	CiphersServerClient     []string
	MACsClientServer        []string
	MACsServerClient        []string
	CompressionClientServer []string
	CompressionServerClient []string
	LanguagesClientServer   []string
	LanguagesServerClient   []string
	FirstKexFollows         bool
	Reserved                uint32
}

type fixtureGexRequest struct {
	MinBits       uint32 `sshtype:"34"`
	PreferredBits uint32
	MaxBits       uint32
}

type fixtureGexGroup struct {
	P *big.Int `sshtype:"31"`
	G *big.Int
}

func readFixturePlainSSHPacket(reader io.Reader) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(reader, length[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(length[:])
	if n < 6 || n > 256*1024 {
		return nil, fmt.Errorf("invalid fixture packet length %d", n)
	}
	packet := make([]byte, int(n))
	if _, err := io.ReadFull(reader, packet); err != nil {
		return nil, err
	}
	padding := int(packet[0])
	if padding < 4 || padding >= len(packet)-1 {
		return nil, fmt.Errorf("invalid fixture packet padding %d", padding)
	}
	return packet[1 : len(packet)-padding], nil
}

func writeFixturePlainSSHPacket(writer io.Writer, payload []byte) error {
	padding := 8 - (len(payload)+5)%8
	if padding < 4 {
		padding += 8
	}
	packet := make([]byte, 5+len(payload)+padding)
	binary.BigEndian.PutUint32(packet[:4], uint32(len(packet)-4))
	packet[4] = byte(padding)
	copy(packet[5:], payload)
	_, err := writer.Write(packet)
	return err
}

func TestSSHLegacyGEXKeepsGroupValidation(t *testing.T) {
	for _, test := range []struct {
		name string
		bits uint
		g    int64
		want string
	}{
		{"1024-bit group rejected", 1024, 2, "gex p is out of range (1024 bits)"},
		{"oversized group rejected", 8193, 2, "gex p is out of range (8193 bits)"},
		{"unsafe generator rejected", 2048, 1, "gex g is not safe"},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			finished := make(chan error, 1)
			bounds := make(chan fixtureGexRequest, 1)
			go func() {
				connection, err := listener.Accept()
				if err != nil {
					finished <- err
					return
				}
				defer connection.Close()
				connection.SetDeadline(time.Now().Add(3 * time.Second))
				reader := bufio.NewReader(connection)
				if _, err = io.WriteString(connection, "SSH-2.0-NetworkToolkit_GEX_fixture\r\n"); err != nil {
					finished <- err
					return
				}
				if _, err = reader.ReadString('\n'); err != nil {
					finished <- err
					return
				}
				packet, err := readFixturePlainSSHPacket(reader)
				if err != nil || len(packet) == 0 || packet[0] != 20 {
					finished <- fmt.Errorf("read client KEXINIT: %v", err)
					return
				}
				init := fixtureKexInit{
					Kex: []string{ssh.InsecureKeyExchangeDHGEXSHA1, ssh.InsecureKeyExchangeDH1SHA1}, HostKeys: []string{ssh.KeyAlgoED25519},
					CiphersClientServer: []string{ssh.CipherAES128CTR}, CiphersServerClient: []string{ssh.CipherAES128CTR},
					MACsClientServer: []string{ssh.HMACSHA256}, MACsServerClient: []string{ssh.HMACSHA256},
					CompressionClientServer: []string{"none"}, CompressionServerClient: []string{"none"},
				}
				if err = writeFixturePlainSSHPacket(connection, ssh.Marshal(&init)); err != nil {
					finished <- err
					return
				}
				packet, err = readFixturePlainSSHPacket(reader)
				var request fixtureGexRequest
				if err != nil {
					finished <- err
					return
				}
				if err = ssh.Unmarshal(packet, &request); err != nil {
					finished <- err
					return
				}
				bounds <- request
				p := new(big.Int).Lsh(big.NewInt(1), test.bits-1)
				p.Add(p, big.NewInt(1))
				if err = writeFixturePlainSSHPacket(connection, ssh.Marshal(&fixtureGexGroup{P: p, G: big.NewInt(test.g)})); err != nil {
					finished <- err
					return
				}
				// The rejected group must cause the client to close its transport.
				_, err = reader.ReadByte()
				if err == io.EOF {
					err = nil
				} else if err == nil {
					err = fmt.Errorf("client continued after invalid GEX parameters")
				}
				finished <- err
			}()
			m := sshManager(t, t.TempDir())
			req := terminalRequest()
			req.SSHCompatibility, req.TimeoutMs = SSHCompatibilityLegacy, 1000
			req.Host, _, _ = net.SplitHostPort(listener.Addr().String())
			_, port, _ := net.SplitHostPort(listener.Addr().String())
			req.Port, _ = strconv.Atoi(port)
			info, err := m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			failed := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "error" })
			if !strings.Contains(failed.Error, test.want) || failed.ErrorCode != "" || failed.Prompt != nil {
				t.Fatalf("GEX validation weakened or mislabeled: %+v", failed)
			}
			select {
			case request := <-bounds:
				if request.MinBits != 2048 || request.PreferredBits != 2048 || request.MaxBits != 8192 {
					t.Fatalf("GEX request bounds changed: %+v", request)
				}
			default:
				t.Fatal("client did not request GEX with bounded parameters")
			}
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("invalid GEX group left the socket open")
			}
		})
	}
}
