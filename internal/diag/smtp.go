package diag

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

// SMTP is a controlled diagnostic. Sending mail requires an explicit option.
// Credentials and message bodies are never included in emitted events/results.
func smtpTest(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	host, err := validateHost(req.Target)
	if err != nil {
		return model.Result{}, err
	}
	port, err := intOption(req.Options, "port", 587, 1, 65535)
	if err != nil {
		return model.Result{}, err
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 8000, 500, 30000)
	if err != nil {
		return model.Result{}, err
	}
	mode := strings.ToLower(stringOption(req.Options, "security", "auto"))
	if mode != "auto" && mode != "starttls" && mode != "tls" && mode != "none" {
		return model.Result{}, errors.New("unsupported SMTP security mode")
	}
	username := stringOption(req.Options, "username", "")
	password := stringOption(req.Options, "password", "")
	from := stringOption(req.Options, "from", "")
	to := stringOption(req.Options, "to", "")
	send := boolOption(req.Options, "sendTest", false)
	if len(username) > 320 || len(password) > 4096 {
		return model.Result{}, errors.New("SMTP credentials exceed allowed length")
	}
	if (username == "") != (password == "") {
		return model.Result{}, errors.New("provide both an SMTP username and password")
	}
	if send {
		if username == "" {
			return model.Result{}, errors.New("sending a test message requires authentication")
		}
		if _, e := mail.ParseAddress(from); e != nil || strings.ContainsAny(from, "\r\n") {
			return model.Result{}, errors.New("enter a valid From address")
		}
		if _, e := mail.ParseAddress(to); e != nil || strings.ContainsAny(to, "\r\n") {
			return model.Result{}, errors.New("enter a valid To address")
		}
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	steps := []map[string]any{}
	add := func(stage, status, detail string) {
		step := map[string]any{"time": measurementTime(), "stage": stage, "status": status, "detail": detail}
		steps = append(steps, step)
		level := "info"
		if status == "failed" {
			level = "warning"
		}
		event(emit, "smtp", level, stage+": "+detail, step)
	}
	result := func(summary string) model.Result {
		return model.Result{Summary: summary, Data: map[string]any{"host": host, "port": port, "securityRequested": mode, "steps": steps, "messageSent": send && len(steps) > 0 && steps[len(steps)-1]["stage"] == "Test message" && steps[len(steps)-1]["status"] == "passed"}}
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, e := platform.DialContext(dialCtx, "tcp", address, 0)
	if e != nil {
		add("TCP connection", "failed", "Connection failed: "+e.Error())
		return result("SMTP connection failed"), e
	}
	defer conn.Close()
	add("TCP connection", "passed", "Connected to "+address)
	if deadline, ok := dialCtx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	encrypted := false
	implicit := mode == "tls" || (mode == "auto" && port == 465)
	if implicit {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if e = tlsConn.HandshakeContext(dialCtx); e != nil {
			add("TLS handshake", "failed", e.Error())
			return result("SMTP TLS failed"), e
		}
		conn = tlsConn
		encrypted = true
		add("TLS handshake", "passed", tlsConn.ConnectionState().NegotiatedProtocol+" TLS established and certificate verified")
	}
	client, e := smtp.NewClient(conn, host)
	if e != nil {
		add("SMTP banner", "failed", e.Error())
		return result("SMTP banner rejected"), e
	}
	defer client.Close()
	add("SMTP banner", "passed", "SMTP greeting accepted")
	if e = client.Hello("network-toolkit.local"); e != nil {
		add("EHLO", "failed", e.Error())
		return result("SMTP EHLO failed"), e
	}
	add("EHLO", "passed", "Server accepted EHLO")
	extensions := []string{}
	for _, feature := range []string{"STARTTLS", "AUTH", "SIZE", "8BITMIME", "SMTPUTF8", "PIPELINING", "DSN"} {
		if ok, _ := client.Extension(feature); ok {
			extensions = append(extensions, feature)
		}
	}
	add("Capabilities", "passed", strings.Join(extensions, ", "))
	if !encrypted && mode != "none" {
		supported, _ := client.Extension("STARTTLS")
		if supported {
			if e = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); e != nil {
				add("STARTTLS", "failed", e.Error())
				return result("SMTP STARTTLS failed"), e
			}
			encrypted = true
			add("STARTTLS", "passed", "TLS established and certificate verified")
		} else if mode == "starttls" || mode == "auto" && (port == 587 || username != "") {
			add("STARTTLS", "failed", "Server did not advertise STARTTLS; authentication will not continue")
			return result("SMTP STARTTLS unavailable"), errors.New("SMTP server does not support STARTTLS")
		} else {
			add("STARTTLS", "skipped", "Server does not advertise STARTTLS")
		}
	}
	if username != "" {
		if !encrypted {
			add("Authentication", "failed", "Refusing to transmit credentials without TLS")
			return result("SMTP authentication requires TLS"), errors.New("SMTP credentials require TLS")
		}
		auth := smtp.PlainAuth("", username, password, host)
		if e = client.Auth(auth); e != nil {
			add("Authentication", "failed", "Server rejected authentication (credentials not shown)")
			return result("SMTP authentication failed"), errors.New("SMTP authentication failed")
		}
		add("Authentication", "passed", "Credentials accepted")
	} else {
		add("Authentication", "skipped", "No credentials provided")
	}
	if !send {
		add("Test message", "skipped", "Sending is disabled; no email was sent")
		return result("SMTP connection test completed"), nil
	}
	if e = client.Mail(from); e != nil {
		add("MAIL FROM", "failed", e.Error())
		return result("SMTP sender rejected"), e
	}
	add("MAIL FROM", "passed", "Sender accepted")
	if e = client.Rcpt(to); e != nil {
		add("RCPT TO", "failed", e.Error())
		return result("SMTP recipient rejected"), e
	}
	add("RCPT TO", "passed", "Recipient accepted")
	writer, e := client.Data()
	if e != nil {
		add("DATA", "failed", e.Error())
		return result("SMTP DATA rejected"), e
	}
	body := "From: " + from + "\r\nTo: " + to + "\r\nSubject: Network Toolkit SMTP diagnostic\r\nDate: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nThis is an SMTP connection test initiated in Network Toolkit.\r\n"
	_, e = io.WriteString(writer, body)
	if e == nil {
		e = writer.Close()
	}
	if e != nil {
		add("Test message", "failed", e.Error())
		return result("SMTP test message failed"), e
	}
	add("Test message", "passed", "Server accepted the test message for delivery")
	_ = client.Quit()
	return result("SMTP test message accepted"), nil
}
