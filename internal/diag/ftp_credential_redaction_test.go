package diag

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFTPAuthenticationReplyCannotEchoPassword(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	secret := "synthetic-password-must-not-be-in-error"
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
		fmt.Fprint(connection, "220 synthetic fixture\r\n")
		reader := bufio.NewReader(connection)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "USER ") {
				fmt.Fprint(connection, "331 Password required\r\n")
			} else if strings.HasPrefix(line, "PASS ") {
				fmt.Fprintf(connection, "530 rejected supplied password %s\r\n", secret)
				return
			} else {
				return
			}
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err = openFTP(ctx, host, port, "plain", "fixture", secret, "", 3*time.Second, false)
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "rejected supplied") {
		t.Fatalf("unredacted authentication error: %v", err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("synthetic FTP fixture did not finish")
	}
}
