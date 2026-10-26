package diag

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

func TestSMTPConnectivityDoesNotSendMailByDefault(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan string, 1)
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			done <- "accept: " + e.Error()
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
		fmt.Fprint(conn, "220 localhost ESMTP test\r\n")
		scanner := bufio.NewScanner(conn)
		transcript := ""
		for scanner.Scan() {
			line := scanner.Text()
			transcript += line + "\n"
			verb := strings.ToUpper(strings.Fields(line)[0])
			switch verb {
			case "EHLO", "HELO":
				fmt.Fprint(conn, "250-localhost\r\n250 SIZE 4096\r\n")
			case "QUIT":
				fmt.Fprint(conn, "221 Bye\r\n")
				done <- transcript
				return
			default:
				fmt.Fprint(conn, "500 Unexpected command\r\n")
				done <- transcript
				return
			}
		}
		done <- transcript
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	req := model.Request{Tool: "smtp", Target: "127.0.0.1", Options: map[string]any{"port": port, "security": "none", "timeoutMs": 3000}}
	var liveSteps []map[string]any
	result, err := smtpTest(context.Background(), req, func(event model.Event) {
		if data, ok := event.Data.(map[string]any); ok && data["type"] == "smtp" {
			liveSteps = append(liveSteps, data["value"].(map[string]any))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary != "SMTP connection test completed" {
		t.Fatalf("unexpected summary %s", result.Summary)
	}
	steps := result.Data.(map[string]any)["steps"].([]map[string]any)
	if len(liveSteps) != len(steps) || len(steps) < 3 {
		t.Fatalf("SMTP stages missing from live events: %+v %+v", liveSteps, steps)
	}
	previous := time.Time{}
	for i, step := range steps {
		stamp := parsedMeasurement(t, step["time"].(string))
		if stamp.Before(previous) || liveSteps[i]["time"] != step["time"] {
			t.Fatalf("SMTP stage clock association changed: %+v %+v", liveSteps, steps)
		}
		previous = stamp
	}
	transcript := <-done
	if strings.Contains(transcript, "MAIL FROM") || strings.Contains(transcript, "RCPT TO") || strings.Contains(transcript, "DATA") {
		t.Fatalf("unexpected email transmission %q", transcript)
	}
}

func TestSMTPRejectsSendWithoutAuthentication(t *testing.T) {
	_, err := smtpTest(context.Background(), model.Request{Tool: "smtp", Target: "mail.example.test", Options: map[string]any{"sendTest": true, "from": "sender@example.test", "to": "recipient@example.test"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "requires authentication") {
		t.Fatalf("unexpected error: %v", err)
	}
}
