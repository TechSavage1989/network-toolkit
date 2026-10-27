package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/desktop"
)

const (
	terminalViewBootstrapLimit = 16 << 10
	terminalViewResponseLimit  = 16 << 10
	terminalViewDirectoryName  = "Network_Toolkit.Views"
	terminalViewPulseInterval  = 2 * time.Second
	terminalViewRequestTimeout = 3 * time.Second
	terminalViewFailureLimit   = 3
)

// terminalViewLaunch is an in-memory, stdin-only bootstrap for a view client.
// It contains no connection credentials and never creates a session manager.
type terminalViewLaunch struct {
	Version         int    `json:"version"`
	OwnerHost       string `json:"ownerHost"`
	OwnerInstanceID string `json:"ownerInstanceId"`
	ViewID          string `json:"viewId"`
	ViewToken       string `json:"viewToken"`
	DataDir         string `json:"dataDir"`
	Title           string `json:"title"`
	NoBrowser       bool   `json:"noBrowser"`
}

func (launch *terminalViewLaunch) validate() error {
	if launch.Version != 1 {
		return errors.New("unsupported detached Terminal bootstrap version")
	}
	host, portText, err := net.SplitHostPort(launch.OwnerHost)
	port, portErr := strconv.Atoi(portText)
	if err != nil || host != "127.0.0.1" || portErr != nil || port < 1 || port > 65535 || launch.OwnerHost != net.JoinHostPort(host, strconv.Itoa(port)) {
		return errors.New("the detached Terminal owner must be a canonical IPv4 loopback endpoint")
	}
	for _, field := range []struct {
		value string
		bytes int
	}{{launch.OwnerInstanceID, 16}, {launch.ViewID, 16}, {launch.ViewToken, 32}} {
		decoded, err := hex.DecodeString(field.value)
		if err != nil || len(decoded) != field.bytes || strings.ToLower(field.value) != field.value {
			return errors.New("invalid detached Terminal owner, view, or token identity")
		}
	}
	if len(launch.DataDir) > 4096 || !filepath.IsAbs(launch.DataDir) || filepath.Clean(launch.DataDir) != launch.DataDir ||
		filepath.Base(launch.DataDir) != launch.ViewID || filepath.Base(filepath.Dir(launch.DataDir)) != terminalViewDirectoryName ||
		strings.IndexFunc(launch.DataDir, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return errors.New("the detached Terminal data folder must be an absolute isolated view folder")
	}
	launch.Title = strings.TrimSpace(launch.Title)
	if launch.Title == "" {
		launch.Title = "Network Toolkit Terminal"
	}
	if len(launch.Title) > 256 || strings.IndexFunc(launch.Title, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return errors.New("invalid detached Terminal window title")
	}
	return nil
}

func decodeTerminalViewLaunch(input io.Reader) (terminalViewLaunch, error) {
	var launch terminalViewLaunch
	if input == nil {
		return launch, errors.New("the detached Terminal bootstrap is unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(input, terminalViewBootstrapLimit+1))
	if err != nil {
		return launch, errors.New("the detached Terminal bootstrap could not be read")
	}
	defer clear(raw)
	if len(raw) > terminalViewBootstrapLimit {
		return launch, errors.New("the detached Terminal bootstrap exceeds its size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&launch); err != nil {
		return launch, errors.New("the detached Terminal bootstrap is invalid")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return launch, errors.New("the detached Terminal bootstrap contains extra data")
	}
	if err := launch.validate(); err != nil {
		return launch, err
	}
	return launch, nil
}

func encodeTerminalViewLaunch(launch terminalViewLaunch) ([]byte, error) {
	if err := launch.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(launch)
	if err != nil || len(raw)+1 > terminalViewBootstrapLimit {
		return nil, errors.New("the detached Terminal bootstrap exceeds its size limit")
	}
	return append(raw, '\n'), nil
}

func terminalViewChildArguments(noBrowser bool) []string {
	args := []string{"--terminal-view"}
	if noBrowser {
		args = append(args, "--no-browser")
	}
	return args
}

// launchDetachedTerminal launches only this executable. The request context
// bounds the decision to start, not the child's lifetime. The owner observes
// the returned buffered completion channel for crash recovery.
func launchDetachedTerminal(ctx context.Context, launch terminalViewLaunch) (<-chan error, error) {
	raw, err := encodeTerminalViewLaunch(launch)
	if err != nil {
		return nil, err
	}
	executable, err := updateCurrentExecutable()
	if err != nil {
		clear(raw)
		return nil, fmt.Errorf("locate the detached Terminal executable: %w", err)
	}
	cmd := exec.Command(executable, terminalViewChildArguments(launch.NoBrowser)...)
	return startTerminalViewChild(ctx, cmd, raw)
}

func startTerminalViewChild(ctx context.Context, cmd *exec.Cmd, bootstrap []byte) (<-chan error, error) {
	if ctx == nil {
		clear(bootstrap)
		return nil, errors.New("the detached Terminal launch context is unavailable")
	}
	if err := ctx.Err(); err != nil {
		clear(bootstrap)
		return nil, err
	}
	configureTerminalViewChild(cmd)
	// os/exec closes its stdin pipe when this bounded reader reaches EOF.
	// No capability token is placed in argv, the environment, or a ticket file.
	cmd.Stdin = bytes.NewReader(bootstrap)
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		clear(bootstrap)
		return nil, fmt.Errorf("start the detached Terminal window: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		clear(bootstrap)
		done <- err
		close(done)
	}()
	return done, nil
}

type terminalViewPulse struct {
	OwnerInstanceID string `json:"ownerInstanceId"`
	ViewID          string `json:"viewId"`
	State           string `json:"state"`
}

type terminalViewControl struct {
	launch terminalViewLaunch
	client *http.Client
}

type terminalViewProtocolError struct{ message string }

func (err *terminalViewProtocolError) Error() string { return err.message }

func newTerminalViewControl(launch terminalViewLaunch) *terminalViewControl {
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            (&net.Dialer{Timeout: terminalViewRequestTimeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxResponseHeaderBytes: 16 << 10,
		MaxIdleConnsPerHost:    1,
		IdleConnTimeout:        10 * time.Second,
	}
	return &terminalViewControl{launch: launch, client: &http.Client{
		Transport: transport,
		Timeout:   terminalViewRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (control *terminalViewControl) request(ctx context.Context, action string) (int, []byte, error) {
	// action is selected internally by pulse/release; never accept a URL or
	// caller-provided path from the owner response or a terminal session.
	if action != "pulse" && action != "release" {
		return 0, nil, &terminalViewProtocolError{"invalid detached Terminal control action"}
	}
	url := "http://" + control.launch.OwnerHost + "/api/terminal-views/current/" + action
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Session-Token", control.launch.ViewToken)
	response, err := control.client.Do(request)
	if err != nil {
		return 0, nil, errors.New("the detached Terminal owner could not be reached")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, terminalViewResponseLimit+1))
	if err != nil {
		return response.StatusCode, nil, errors.New("the detached Terminal owner response could not be read")
	}
	if len(raw) > terminalViewResponseLimit {
		return response.StatusCode, nil, &terminalViewProtocolError{"the detached Terminal owner response exceeded its size limit"}
	}
	return response.StatusCode, raw, nil
}

func (control *terminalViewControl) pulse(ctx context.Context) (terminalViewPulse, error) {
	var pulse terminalViewPulse
	status, raw, err := control.request(ctx, "pulse")
	if err != nil {
		return pulse, err
	}
	if status == http.StatusGone {
		return terminalViewPulse{State: "returned"}, nil
	}
	if status != http.StatusOK {
		if status >= 300 && status < 500 {
			return pulse, &terminalViewProtocolError{"the detached Terminal owner rejected this view"}
		}
		return pulse, errors.New("the detached Terminal owner is temporarily unavailable")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&pulse); err != nil {
		return pulse, &terminalViewProtocolError{"the detached Terminal owner returned an invalid response"}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return pulse, &terminalViewProtocolError{"the detached Terminal owner returned extra response data"}
	}
	if pulse.OwnerInstanceID != control.launch.OwnerInstanceID || pulse.ViewID != control.launch.ViewID {
		return pulse, &terminalViewProtocolError{"the detached Terminal owner or view identity changed"}
	}
	switch pulse.State {
	case "opening", "detached", "return_requested", "returned":
		return pulse, nil
	default:
		return pulse, &terminalViewProtocolError{"the detached Terminal owner returned an unknown view state"}
	}
}

func (control *terminalViewControl) release() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, _ = control.request(ctx, "release")
}

func ensureTerminalViewDirectory(path string) error {
	for _, directory := range []string{filepath.Dir(path), path} {
		info, err := os.Lstat(directory)
		if errors.Is(err, os.ErrNotExist) {
			if err = os.MkdirAll(directory, 0700); err != nil {
				return fmt.Errorf("create the detached Terminal profile folder: %w", err)
			}
			info, err = os.Lstat(directory)
		}
		if err != nil {
			return fmt.Errorf("inspect the detached Terminal profile folder: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("the detached Terminal profile folder must be an isolated directory")
		}
	}
	return nil
}

// runDetachedTerminalView owns only one native view and its control heartbeat.
// Device connections, settings, host trust and terminal input stay in the owner.
func runDetachedTerminalView(input io.Reader, noBrowser bool) error {
	launch, err := decodeTerminalViewLaunch(input)
	if err != nil {
		return err
	}
	control := newTerminalViewControl(launch)
	defer control.client.CloseIdleConnections()
	defer control.release()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	first, err := control.pulse(ctx)
	if err != nil || first.State == "returned" {
		return err
	}
	if err := ensureTerminalViewDirectory(launch.DataDir); err != nil {
		return err
	}
	watchDone := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(terminalViewPulseInterval)
		defer ticker.Stop()
		failures := 0
		for {
			select {
			case <-ctx.Done():
				watchDone <- nil
				return
			case <-ticker.C:
				pulse, err := control.pulse(ctx)
				if ctx.Err() != nil {
					watchDone <- nil
					return
				}
				if err != nil {
					failures++
					var protocol *terminalViewProtocolError
					if errors.As(err, &protocol) {
						watchDone <- err
						cancel()
						return
					}
					if failures >= terminalViewFailureLimit {
						// A validated owner can shut down intentionally (including
						// an app-data reset). Its unavailable listener ends this
						// view cleanly; startup and identity failures still report.
						watchDone <- nil
						cancel()
						return
					}
					continue
				}
				failures = 0
				if pulse.State == "returned" {
					watchDone <- nil
					cancel()
					return
				}
				// return_requested keeps the renderer open to provide its final
				// checkpoint. Only returned/410 or local exit closes this view.
			}
		}
	}()
	if noBrowser || launch.NoBrowser {
		<-ctx.Done()
	} else {
		err = desktop.Run(ctx, desktop.Config{
			Title: launch.Title, URL: "http://" + launch.OwnerHost + "/terminal-view/" + launch.ViewID,
			SessionToken: launch.ViewToken,
			DataDir:      launch.DataDir, Stop: cancel,
		})
	}
	cancel()
	watchErr := <-watchDone
	if err != nil {
		return err
	}
	return watchErr
}
