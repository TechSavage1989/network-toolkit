// Command browser-window-standin plays Edge for the hosted Windows check of
// the browser window (browser_window_windows_test.go): it records how the app
// started it, opens the one-time launch link as a browser would, polls the
// engine like the page does, and exits (its window "closes") when the check
// creates the file named "close" beside it. Synthetic: no real browser.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type report struct {
	Args     []string       `json:"args"`
	Launch   string         `json:"launch"`
	Status   int            `json:"status"`
	Path     string         `json:"path"`
	Token    bool           `json:"token"`
	Engine   map[string]any `json:"engine"`
	Problems []string       `json:"problems"`
}

var tokenPattern = regexp.MustCompile(`__networkToolkitNativeSession:'([0-9a-f]{64})'`)

func main() {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	r := report{Args: os.Args[1:]}
	write := func() {
		data, _ := json.MarshalIndent(r, "", " ")
		_ = os.WriteFile(filepath.Join(dir, "report.json.tmp"), data, 0o600)
		_ = os.Rename(filepath.Join(dir, "report.json.tmp"), filepath.Join(dir, "report.json"))
	}
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--app=") {
			r.Launch = strings.TrimPrefix(arg, "--app=")
		}
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}
	token := ""
	if response, err := client.Get(r.Launch); err != nil {
		r.Problems = append(r.Problems, "open the launch link: "+err.Error())
	} else {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		r.Status, r.Path = response.StatusCode, response.Request.URL.Path
		if m := tokenPattern.FindSubmatch(body); m != nil {
			token, r.Token = string(m[1]), true
		}
	}
	base := ""
	if i := strings.Index(r.Launch, "/launch"); i > 0 {
		base = r.Launch[:i]
	}
	status := func() map[string]any {
		request, _ := http.NewRequest(http.MethodGet, base+"/api/status", nil)
		request.Header.Set("X-Session-Token", token)
		response, err := client.Do(request)
		if err != nil {
			return nil
		}
		defer response.Body.Close()
		var value map[string]any
		_ = json.NewDecoder(response.Body).Decode(&value)
		return value
	}
	if token != "" {
		if value := status(); value != nil {
			r.Engine = map[string]any{}
			for _, key := range []string{"os", "uiHost", "terminalDetachAvailable", "editorWindowAvailable", "windowMode"} {
				r.Engine[key] = value[key]
			}
		}
	}
	write()
	// Poll like the page until the check closes the "window".
	for {
		if _, err := os.Stat(filepath.Join(dir, "close")); err == nil {
			return
		}
		if token != "" {
			status()
		}
		time.Sleep(time.Second)
	}
}
