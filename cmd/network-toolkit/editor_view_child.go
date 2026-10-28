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
	editorViewBootstrapLimit = 16 << 10
	editorViewResponseLimit  = 16 << 10
	editorViewDirectoryName  = "Network_Toolkit.EditorViews"
	editorViewPulseInterval  = 2 * time.Second
	editorViewRequestTimeout = 3 * time.Second
)

// editorViewLaunch is an in-memory, stdin-only bootstrap for a view client.
// It contains no connection credentials and never creates a session manager.
type editorViewLaunch struct {
	Version         int    `json:"version"`
	OwnerHost       string `json:"ownerHost"`
	OwnerInstanceID string `json:"ownerInstanceId"`
	ViewID          string `json:"viewId"`
	ViewToken       string `json:"viewToken"`
	DataDir         string `json:"dataDir"`
	Title           string `json:"title"`
	NoBrowser       bool   `json:"noBrowser"`
}

func (launch *editorViewLaunch) validate() error {
	if launch.Version != 1 {
		return errors.New("unsupported detached editor bootstrap version")
	}
	host, portText, err := net.SplitHostPort(launch.OwnerHost)
	port, portErr := strconv.Atoi(portText)
	if err != nil || host != "127.0.0.1" || portErr != nil || port < 1 || port > 65535 || launch.OwnerHost != net.JoinHostPort(host, strconv.Itoa(port)) {
		return errors.New("the detached editor owner must be a canonical IPv4 loopback endpoint")
	}
	for _, field := range []struct {
		value string
		bytes int
	}{{launch.OwnerInstanceID, 16}, {launch.ViewID, 16}, {launch.ViewToken, 32}} {
		decoded, err := hex.DecodeString(field.value)
		if err != nil || len(decoded) != field.bytes || strings.ToLower(field.value) != field.value {
			return errors.New("invalid detached editor owner, view, or token identity")
		}
	}
	if len(launch.DataDir) > 4096 || !filepath.IsAbs(launch.DataDir) || filepath.Clean(launch.DataDir) != launch.DataDir ||
		filepath.Base(launch.DataDir) != launch.ViewID || filepath.Base(filepath.Dir(launch.DataDir)) != editorViewDirectoryName ||
		strings.IndexFunc(launch.DataDir, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return errors.New("the detached editor data folder must be an absolute isolated view folder")
	}
	launch.Title = strings.TrimSpace(launch.Title)
	if launch.Title == "" {
		launch.Title = "Network Toolkit Editor"
	}
	if len(launch.Title) > 256 || strings.IndexFunc(launch.Title, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return errors.New("invalid detached editor window title")
	}
	return nil
}

func decodeEditorViewLaunch(input io.Reader) (editorViewLaunch, error) {
	var launch editorViewLaunch
	if input == nil {
		return launch, errors.New("the detached editor bootstrap is unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(input, editorViewBootstrapLimit+1))
	if err != nil {
		return launch, errors.New("the detached editor bootstrap could not be read")
	}
	defer clear(raw)
	if len(raw) > editorViewBootstrapLimit {
		return launch, errors.New("the detached editor bootstrap exceeds its size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&launch); err != nil {
		return launch, errors.New("the detached editor bootstrap is invalid")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return launch, errors.New("the detached editor bootstrap contains extra data")
	}
	if err := launch.validate(); err != nil {
		return launch, err
	}
	return launch, nil
}

func encodeEditorViewLaunch(launch editorViewLaunch) ([]byte, error) {
	if err := launch.validate(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(launch)
	if err != nil || len(raw)+1 > editorViewBootstrapLimit {
		return nil, errors.New("the detached editor bootstrap exceeds its size limit")
	}
	return append(raw, '\n'), nil
}

func editorViewChildArguments(noBrowser bool) []string {
	args := []string{"--editor-view"}
	if noBrowser {
		args = append(args, "--no-browser")
	}
	return args
}

// launchDetachedEditor launches only this executable. The request context
// bounds the decision to start, not the child's lifetime. The owner observes
// the returned buffered completion channel for crash recovery.
func launchDetachedEditor(ctx context.Context, launch editorViewLaunch) (<-chan error, error) {
	raw, err := encodeEditorViewLaunch(launch)
	if err != nil {
		return nil, err
	}
	executable, err := updateCurrentExecutable()
	if err != nil {
		clear(raw)
		return nil, fmt.Errorf("locate the detached editor executable: %w", err)
	}
	cmd := exec.Command(executable, editorViewChildArguments(launch.NoBrowser)...)
	return startEditorViewChild(ctx, cmd, raw)
}

func startEditorViewChild(ctx context.Context, cmd *exec.Cmd, bootstrap []byte) (<-chan error, error) {
	if ctx == nil {
		clear(bootstrap)
		return nil, errors.New("the detached editor launch context is unavailable")
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
		return nil, fmt.Errorf("start the detached editor window: %w", err)
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

type editorViewPulse struct {
	OwnerInstanceID string `json:"ownerInstanceId"`
	ViewID          string `json:"viewId"`
	State           string `json:"state"`
}

type editorViewControl struct {
	launch editorViewLaunch
	client *http.Client
}

type editorViewProtocolError struct{ message string }

func (err *editorViewProtocolError) Error() string { return err.message }

func newEditorViewControl(launch editorViewLaunch) *editorViewControl {
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            (&net.Dialer{Timeout: editorViewRequestTimeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxResponseHeaderBytes: 16 << 10,
		MaxIdleConnsPerHost:    1,
		IdleConnTimeout:        10 * time.Second,
	}
	return &editorViewControl{launch: launch, client: &http.Client{
		Transport: transport,
		Timeout:   editorViewRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (control *editorViewControl) request(ctx context.Context, action string) (int, []byte, error) {
	// action is selected internally by pulse/release; never accept a URL or
	// caller-provided path from the owner response or a terminal session.
	if action != "pulse" && action != "release" {
		return 0, nil, &editorViewProtocolError{"invalid detached editor control action"}
	}
	url := "http://" + control.launch.OwnerHost + "/api/editor-views/current/" + action
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Session-Token", control.launch.ViewToken)
	response, err := control.client.Do(request)
	if err != nil {
		return 0, nil, errors.New("the detached editor owner could not be reached")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, editorViewResponseLimit+1))
	if err != nil {
		return response.StatusCode, nil, errors.New("the detached editor owner response could not be read")
	}
	if len(raw) > editorViewResponseLimit {
		return response.StatusCode, nil, &editorViewProtocolError{"the detached editor owner response exceeded its size limit"}
	}
	return response.StatusCode, raw, nil
}

func (control *editorViewControl) pulse(ctx context.Context) (editorViewPulse, error) {
	var pulse editorViewPulse
	status, raw, err := control.request(ctx, "pulse")
	if err != nil {
		return pulse, err
	}
	if status == http.StatusGone {
		return editorViewPulse{State: "returned"}, nil
	}
	if status != http.StatusOK {
		if status >= 300 && status < 500 {
			return pulse, &editorViewProtocolError{"the detached editor owner rejected this view"}
		}
		return pulse, errors.New("the detached editor owner is temporarily unavailable")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&pulse); err != nil {
		return pulse, &editorViewProtocolError{"the detached editor owner returned an invalid response"}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return pulse, &editorViewProtocolError{"the detached editor owner returned extra response data"}
	}
	if pulse.OwnerInstanceID != control.launch.OwnerInstanceID || pulse.ViewID != control.launch.ViewID {
		return pulse, &editorViewProtocolError{"the detached editor owner or view identity changed"}
	}
	switch pulse.State {
	case "opening", "ready", "open", "detached", "return_requested", "returned":
		return pulse, nil
	default:
		return pulse, &editorViewProtocolError{"the detached editor owner returned an unknown view state"}
	}
}

func (control *editorViewControl) release() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, _ = control.request(ctx, "release")
}

func ensureEditorViewDirectory(path string) error {
	for _, directory := range []string{filepath.Dir(path), path} {
		info, err := os.Lstat(directory)
		if errors.Is(err, os.ErrNotExist) {
			if err = os.MkdirAll(directory, 0700); err != nil {
				return fmt.Errorf("create the detached editor profile folder: %w", err)
			}
			info, err = os.Lstat(directory)
		}
		if err != nil {
			return fmt.Errorf("inspect the detached editor profile folder: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("the detached editor profile folder must be an isolated directory")
		}
	}
	return nil
}

// runDetachedEditorView owns only one native view and its control heartbeat.
// File capabilities, settings and document ownership stay in the owner.
func runDetachedEditorView(input io.Reader, noBrowser bool) error {
	launch, err := decodeEditorViewLaunch(input)
	if err != nil {
		return err
	}
	control := newEditorViewControl(launch)
	defer control.client.CloseIdleConnections()
	defer control.release()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	first, err := control.pulse(ctx)
	if err != nil || first.State == "returned" {
		return err
	}
	if err := ensureEditorViewDirectory(launch.DataDir); err != nil {
		return err
	}
	watchDone := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(editorViewPulseInterval)
		defer ticker.Stop()
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
					// Do not discard the loaded editor when its owner disappears.
					// The UI reports recovery mode; user can still save a copy.
					continue
				}
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
			Title: launch.Title, URL: "http://" + launch.OwnerHost + "/editor-view/" + launch.ViewID,
			SessionToken:         launch.ViewToken,
			RequireCloseApproval: true,
			DataDir:              launch.DataDir, Stop: cancel,
		})
	}
	cancel()
	watchErr := <-watchDone
	if err != nil {
		return err
	}
	return watchErr
}
