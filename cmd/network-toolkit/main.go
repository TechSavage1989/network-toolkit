package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	resources "networktoolkit"
	"networktoolkit/internal/diag"
	"networktoolkit/internal/desktop"
	"networktoolkit/internal/identity"
	"networktoolkit/internal/macros"
	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
	"networktoolkit/internal/terminal"
)

var web = resources.Web

const appName = "Network Toolkit"
const maxConcurrent = 8
const maxRetained = 100
const maxEvents = 2200
const maxLiveHosts = 1024

var allowedTools = map[string]bool{"quick": true, "ping": true, "trace": true, "dns": true, "smtp": true, "sftp": true, "tftp": true, "ftp": true, "tcp": true, "http": true, "tls": true, "discover": true, "identify": true, "monitor": true, "wol": true, "routes": true, "arp": true, "connections": true, "wifi": true, "ipconfig": true}

type Job struct {
	ID               string         `json:"id"`
	Tool             string         `json:"tool"`
	Target           string         `json:"target"`
	Options          map[string]any `json:"options"`
	Status           string         `json:"status"`
	Started          string         `json:"started"`
	Finished         string         `json:"finished,omitempty"`
	Events           []model.Event  `json:"events"`
	EventSequence    uint64         `json:"eventSequence"`
	LiveHosts        map[string]any `json:"liveHosts,omitempty"`
	SampleStatistics any            `json:"sampleStatistics,omitempty"`
	SampleHistory    *diag.SampleHistory `json:"sampleHistory,omitempty"`
	Result           *model.Result  `json:"result,omitempty"`
	Error            string         `json:"error,omitempty"`
	cancel           context.CancelFunc
}

type application struct {
	ctx                context.Context
	stop               context.CancelFunc
	mu                 sync.RWMutex
	jobs               map[string]*Job
	started            time.Time
	token              string
	host               string
	extraHost          string
	uiHost             string
	dataPath           string
	stateMu            sync.Mutex
	systemMu           sync.Mutex
	system             map[string]any
	systemAt           time.Time
	storageWritable    bool
	storageError       string
	state              json.RawMessage
	terminals          *terminal.Manager
	terminalAutomation *macros.Engine
	terminalError      string
	windowMode         string
	windowInstanceID   string
	windowActivationToken string
	windowActivate     chan struct{}
	windowInbox        terminalWindowInbox
	windowLaunchMu     sync.Mutex
	shutdownRequested  atomic.Bool
	openTerminalWindow func(context.Context, *terminalWindowPreset) (bool, error)
	views              *terminalViewRegistry
	launchTerminalView func(context.Context, terminalViewLaunch) (<-chan error, error)
	headless           bool
	primaryMu          sync.Mutex
	primaryCancel      context.CancelFunc
	primaryWake        chan struct{}
	storage            *appStorageManager
	localTools         *localToolsManager
	publicIP           *publicIPManager
	credentials        *credentialVault
	credentialChanges sync.Mutex
	confirmCredentialDestination func(context.Context, desktop.CredentialDestinationConfirmation) (bool, error)
	inspectCredentialDestination func(context.Context,credentialBinding)(string,error)
	packetCaptures     *packetCaptureManager
	transfers          *transferHostManager
	browserMu          sync.Mutex
	browserConnections map[string]*browserFileConnection
	browserConnecting  int
	dragMu             sync.Mutex
	dragTickets        map[string]remoteDragTicket
	httpGate           sync.RWMutex
	updater            appUpdater
	updateHealthDir    string
	updateFinalizing   atomic.Bool
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("Unable to obtain operating system randomness")
	}
	return hex.EncodeToString(b)
}

func main() {
	if err := waitUpdateReopenIfRequested(); err != nil {
		platform.NotifyFatal("Network Toolkit could not finish reopening: " + err.Error())
		return
	}
	if runUpdateHelperIfRequested() {
		return
	}
	if runUpdateCompatibilityLauncherIfRequested() {
		return
	}
	updateHealth := flag.String("update-health", "", "Private update startup health marker")
	port := flag.Int("port", 0, "Local port, 0 chooses an available port")
	noBrowser := flag.Bool("no-browser", false, "Run the local diagnostic engine without opening its interface")
	dataDir := flag.String("data-dir", "", "Override the configured app-data folder")
	previewHost := flag.String("preview-host", "", "Additional development proxy host")
	printVersion := flag.Bool("version", false, "Print version")
	terminalOnly := flag.Bool("terminal", false, "Open the independent Terminal application window")
	terminalPreset := flag.String("terminal-preset", "", "Encoded nonsecret settings for a Terminal connection form")
	terminalView := flag.Bool("terminal-view", false, "Open a live terminal view using its private stdin bootstrap")
	localHelper := flag.String("local-tools-helper", "", "Private local configuration helper pipe")
	localParent := flag.Uint("local-tools-parent", 0, "Expected local configuration helper parent process")
	packetHelper := flag.String("packet-capture-helper", "", "Private packet capture helper pipe")
	packetParent := flag.Uint("packet-capture-parent", 0, "Expected packet capture helper parent process")
	flag.Parse()
	packetRequested, packetOther := false, false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "packet-capture-helper" || f.Name == "packet-capture-parent" {
			packetRequested = true
		} else {
			packetOther = true
		}
	})
	if packetRequested {
		if packetOther || flag.NArg() != 0 || *packetHelper == "" || *packetParent == 0 || uint64(*packetParent) > uint64(^uint32(0)) {
			platform.NotifyFatal("Invalid private packet capture request.")
			return
		}
		if err := runPacketCaptureHelper(*packetHelper, uint32(*packetParent)); err != nil {
			log.Print("Packet capture helper: ", err)
		}
		return
	}
	localHelperRequested, otherFlags := false, false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "local-tools-helper" || f.Name == "local-tools-parent" {
			localHelperRequested = true
		} else {
			otherFlags = true
		}
	})
	if localHelperRequested {
		if otherFlags || flag.NArg() != 0 || *localHelper == "" || *localParent == 0 || uint64(*localParent) > uint64(^uint32(0)) {
			platform.NotifyFatal("The local configuration helper was not started with a valid private request.")
			return
		}
		if err := runLocalToolsHelper(*localHelper, uint32(*localParent)); err != nil {
			// The requesting workspace reports operation failures. A helper can
			// lose its pipe during normal cancellation or UAC dismissal.
			log.Print("Local configuration helper: ", err)
		}
		return
	}
	if *printVersion {
		fmt.Println(appName, model.Version, runtime.GOOS, runtime.GOARCH)
		return
	}
	if *terminalView {
		if err := runDetachedTerminalView(os.Stdin, *noBrowser); err != nil {
			platform.NotifyFatal("Unable to open the detached terminal: " + err.Error())
		}
		return
	}
	if *port < 0 || *port > 65535 {
		platform.NotifyFatal("The local port must be between 0 and 65535.")
		return
	}
	executable, err := os.Executable()
	if err != nil {
		platform.NotifyFatal("The application path could not be resolved: " + err.Error())
		return
	}
	storageConfig, err := resolveAppStorage(filepath.Dir(executable), *dataDir)
	if err != nil {
		platform.NotifyFatal("The app-data folder could not be resolved: " + err.Error())
		return
	}
	storage := newAppStorage(storageConfig)
	if err = storage.Open(); err != nil {
		platform.NotifyFatal("The app-data folder could not be opened: " + err.Error())
		return
	}
	defer func() {
		if err := storage.Close(); err != nil {
			if errors.Is(err, ErrAppStorageCleanupPending) {
				// The browser can release cache handles shortly after its window
				// closes. The durable marker retries cleanup before next startup.
				log.Print("App-data cleanup will retry on the next launch: ", err)
			} else {
				platform.NotifyFatal("App-data cleanup: " + err.Error())
			}
		}
	}()
	rootDataDir := storageConfig.RootDir
	*dataDir = rootDataDir
	initialPreset, err := decodeTerminalWindowPreset(*terminalPreset)
	if err != nil || (!*terminalOnly && *terminalPreset != "") {
		if err == nil {
			err = errors.New("Connection presets require the independent Terminal mode.")
		}
		platform.NotifyFatal(err.Error())
		return
	}
	if *terminalOnly {
		terminalDir, release, reused, err := prepareTerminalWindow(rootDataDir, initialPreset)
		if err != nil {
			platform.NotifyFatal("Unable to open the Terminal workspace: " + err.Error())
			return
		}
		if reused {
			return
		}
		defer release()
		*dataDir = terminalDir
	}
	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Print("Settings folder unavailable: ", err)
	}
	if err := identity.Configure(*dataDir); err != nil {
		log.Print("Vendor database: ", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	a := newApplication(ctx, filepath.Join(*dataDir, "Network_Toolkit.settings.json"))
	a.updateHealthDir = *updateHealth
	if _, official := updateOfficialExecutableVersion(filepath.Base(executable)); official {
		a.updateFinalizing.Store(true)
	}
	defer a.cleanupStagedUpdate()
	a.storage = storage
	a.headless = *noBrowser
	if runtime.GOOS == "windows" || *noBrowser {
		a.launchTerminalView = launchDetachedTerminal
	}
	title := appName
	if *terminalOnly {
		a.windowMode = "terminal"
		a.windowInbox.add(initialPreset)
		title = appName + " Terminal"
	} else if runtime.GOOS == "windows" && !*noBrowser {
		a.openTerminalWindow = func(ctx context.Context, preset *terminalWindowPreset) (bool, error) {
			return launchIndependentTerminal(ctx, rootDataDir, preset)
		}
	}
	if runtime.GOOS == "windows" && !*noBrowser {
		a.uiHost = "webview2"
	}
	var serverShutdownDone chan struct{}
	defer func() {
		a.stop()
		// Local configuration requests can be waiting for Windows approval.
		// Cancel that private handshake and give an active helper time to stop
		// before the owner exits or app-data maintenance begins.
		if a.localTools != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := a.localTools.Close(shutdownCtx); err != nil {
				log.Print("Local configuration shutdown: ", err)
			}
			cancel()
		}
		// The native window can return as soon as cancellation is signaled.
		// Wait for terminal transports to finish closing, including clearing
		// serial break and completing cancelled native I/O, before main exits.
		a.closeBrowserConnections()
		if a.terminals != nil {
			_ = a.terminals.Close()
		}
		if a.packetCaptures != nil {
			a.packetCaptures.close()
		}
		if a.transfers != nil {
			a.transfers.Close()
		}
		if serverShutdownDone != nil {
			<-serverShutdownDone
		}
		// Drain writers after listener shutdown and forced connection close.
		// Later requests see the cancelled app context before entering a route.
		a.httpGate.Lock()
		a.httpGate.Unlock()
		// A child native window may be returning when its owner shuts down.
		// Keep the data lease until those windows have released their caches.
		a.views.mu.Lock()
		a.views.homeClosing = true
		a.views.mu.Unlock()
		a.views.children.Wait()
	}()
	listenHost := "127.0.0.1"
	// Only the Unix development binary supports the supervised preview proxy.
	// The Windows deliverable always binds to loopback, including with flags.
	if runtime.GOOS != "windows" && *previewHost != "" && *noBrowser {
		listenHost = "0.0.0.0"
	}
	ln, err := net.Listen("tcp4", fmt.Sprintf("%s:%d", listenHost, *port))
	if err != nil {
		platform.NotifyFatal("Unable to start Network Toolkit: " + err.Error())
		return
	}
	a.host = net.JoinHostPort("127.0.0.1", fmt.Sprint(ln.Addr().(*net.TCPAddr).Port))
	a.extraHost = *previewHost
	url := "http://" + a.host + "/"
	srv := &http.Server{Handler: a, BaseContext: func(net.Listener) context.Context { return a.ctx }, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	serverShutdownDone = make(chan struct{})
	go func() {
		defer close(serverShutdownDone)
		<-a.ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		_ = srv.Close()
	}()
	go func() {
		if e := srv.Serve(ln); e != nil && !errors.Is(e, http.ErrServerClosed) {
			log.Print(e)
			a.stop()
		}
	}()
	if *terminalOnly {
		unregister, err := a.registerTerminalWindow(*dataDir)
		if err != nil {
			platform.NotifyFatal(err.Error())
			return
		}
		defer unregister()
	}
	fmt.Printf("%s %s\n%s\n", appName, model.Version, url)
	if *noBrowser {
		a.updateStartupReady()
	}
	a.runPrimaryLifecycle(url, *noBrowser, *dataDir, title)
}

func newApplication(parent context.Context, dataPath string) *application {
	ctx, cancel := context.WithCancel(parent)
	a := &application{ctx: ctx, stop: cancel, jobs: make(map[string]*Job), started: time.Now(), token: randomHex(32), uiHost: "browser", dataPath: dataPath, state: json.RawMessage(`{}`), windowMode: "main", windowInstanceID: randomHex(16), windowActivationToken: randomHex(32), windowActivate: make(chan struct{}, 1), primaryWake: make(chan struct{}, 1)}
	a.confirmCredentialDestination = desktop.ConfirmCredentialDestination
	a.inspectCredentialDestination = inspectCredentialDestination
	a.localTools = newLocalToolsManager(ctx)
	a.packetCaptures = newPacketCaptureManager(ctx)
	a.transfers = newTransferHostManager(ctx, filepath.Dir(dataPath))
	a.publicIP = newPublicIPManager(ctx)
	if vault, err := newCredentialVault(filepath.Dir(dataPath)); err == nil {
		a.credentials = vault
	} else {
		a.storageError = "Credential vault could not be opened: " + err.Error()
	}
	var terminalErr error
	a.terminals, terminalErr = terminal.NewManager(ctx, filepath.Dir(dataPath))
	if terminalErr != nil {
		a.terminalError = terminalErr.Error()
	} else {
		a.terminals.SetCredentialResolver(a.resolveTerminalCredential)
		a.terminalAutomation = macros.NewEngine(ctx, a.terminals)
	}
	a.views = newTerminalViewRegistry(a)
	if b, err := os.ReadFile(dataPath); err == nil {
		if len(b) <= 2<<20 && json.Valid(b) && len(strings.TrimSpace(string(b))) > 0 && strings.HasPrefix(strings.TrimSpace(string(b)), "{") {
			var state map[string]any
			if json.Unmarshal(b, &state) == nil {
				sanitizeTerminalSettings(state)
				sanitizeFTPProfiles(state)
				sanitizeTransferClientSettings(state)
				a.state, _ = json.Marshal(state)
			}
		} else {
			a.storageError = "Saved settings were invalid. Export any new settings before replacing this file."
		}
	}
	if f, err := os.CreateTemp(filepath.Dir(dataPath), ".network-toolkit-write-check-*"); err == nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		a.storageWritable = true
	} else {
		a.storageError = "Settings cannot be saved in this folder: " + err.Error()
	}
	return a
}

func (a *application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.httpGate.RLock()
	defer a.httpGate.RUnlock()
	if r.URL.Path == "/api/updates/install" {
		a.updater.requestGate.Lock()
		defer a.updater.requestGate.Unlock()
	} else {
		a.updater.requestGate.RLock()
		defer a.updater.requestGate.RUnlock()
	}
	a.updater.mu.Lock()
	updating := a.updater.installing
	a.updater.mu.Unlock()
	if updating {
		jsonError(w, http.StatusServiceUnavailable, "The application is restarting to install an update.")
		return
	}
	if a.updateFinalizing.Load() && r.Method != http.MethodGet && r.Method != http.MethodHead {
		jsonError(w, http.StatusServiceUnavailable, "Finishing the update and reopening the current executable. Please wait.")
		return
	}
	if a.ctx.Err() != nil {
		jsonError(w, http.StatusServiceUnavailable, "The application is closing.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	if r.Host != a.host && (a.extraHost == "" || r.Host != a.extraHost) {
		jsonError(w, http.StatusForbidden, "This host is not allowed.")
		return
	}
	if r.URL.Path == "/" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		b, err := web.ReadFile("web/index.html")
		if err != nil {
			jsonError(w, 500, "Interface unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := strings.ReplaceAll(string(b), "__SESSION_TOKEN__", httpBootstrapToken(a.token))
		html = strings.ReplaceAll(html, "__WINDOW_MODE__", a.windowMode)
		html = strings.ReplaceAll(html, "__TERMINAL_VIEW__", "null")
		html = embeddedAssetPaths(html)
		if a.updateFinalizing.Load() {
			html = updateFinalizingHTML(html, httpBootstrapToken(a.token))
		}
		_, _ = io.WriteString(w, html)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/terminal-view/") {
		a.serveTerminalView(w, r)
		return
	}
	if r.URL.Path == "/favicon.ico" || strings.HasPrefix(r.URL.Path, "/assets/") {
		serveWebAsset(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/drag-download/") {
		a.handleRemoteDragDownload(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	requestView := ""
	activationOnly := a.windowMode == "terminal" && r.Method == http.MethodPost && r.URL.Path == "/api/terminal-window" && a.windowActivationToken != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Window-Activation-Token")), []byte(a.windowActivationToken)) == 1
	if !activationOnly && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Session-Token")), []byte(a.token)) != 1 {
		var found bool
		requestView, found = a.views.authenticate(r.Header.Get("X-Session-Token"))
		if !found {
			jsonError(w, http.StatusUnauthorized, "The app session is unavailable. Reopen the interface from Network Toolkit.")
			return
		}
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+a.host && (a.extraHost == "" || (origin != "http://"+a.extraHost && origin != "https://"+a.extraHost)) {
		jsonError(w, http.StatusForbidden, "Cross-origin requests are not allowed.")
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		jsonError(w, http.StatusForbidden, "Cross-site requests are not allowed.")
		return
	}
	if activationOnly {
		a.handleTerminalWindow(w, r)
		return
	}
	if requestView != "" {
		r = r.WithContext(context.WithValue(r.Context(), terminalViewContextKey{}, requestView))
		a.serveDetachedAPI(w, r, requestView)
		return
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/updates/"):
		a.handleUpdates(w, r)
	case r.URL.Path == "/api/status":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		capabilities := make([]string, 0, len(allowedTools))
		for k := range allowedTools {
			capabilities = append(capabilities, k)
		}
		sort.Strings(capabilities)
		a.stateMu.Lock()
		writable, storageErr := a.storageWritable, a.storageError
		a.stateMu.Unlock()
		jsonResponse(w, 200, map[string]any{"name": appName, "version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "uiHost": a.uiHost, "windowMode": a.windowMode, "localShellAvailable": terminal.LocalShellAvailable(), "terminalWindowAvailable": a.windowMode == "terminal" || a.openTerminalWindow != nil, "terminalDetachAvailable": a.launchTerminalView != nil, "connectionLaunchAvailable": runtime.GOOS == "windows", "started": a.started.UTC().Format(time.RFC3339), "portable": true, "storagePath": a.dataPath, "storageWritable": writable, "storageError": storageErr, "capabilities": capabilities, "limits": map[string]int{"maxJobs": maxConcurrent, "maxHistory": maxRetained}, "system": a.getSystem(false)})
	case r.URL.Path == "/api/terminal-views" || strings.HasPrefix(r.URL.Path, "/api/terminal-views/"):
		a.handleTerminalViews(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/terminal-automation/"):
		a.handleTerminalAutomation(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/local-tools/"):
		a.handleLocalTools(w, r)
	case r.URL.Path == "/api/packet-captures" || strings.HasPrefix(r.URL.Path, "/api/packet-captures/"):
		a.handlePacketCapture(w, r)
	case r.URL.Path == "/api/public-ip" || strings.HasPrefix(r.URL.Path, "/api/public-ip/"):
		a.handlePublicIP(w, r)
	case r.URL.Path == "/api/transfer/local":
		a.handleTransferLocal(w, r)
	case r.URL.Path == "/api/transfer/local-files":
		a.handleTransferLocalFiles(w, r)
	case r.URL.Path == "/api/transfer/server":
		a.handleTransferServer(w, r)
	case r.URL.Path == "/api/transfer/files":
		a.handleTransferFiles(w, r)
	case r.URL.Path == "/api/transfer/open-root":
		a.handleTransferOpenRoot(w, r)
	case r.URL.Path == "/api/remote-browser/drag-ticket":
		a.handleRemoteDragTicket(w, r)
	case r.URL.Path == "/api/remote-browser/drop-upload":
		a.handleRemoteDropUpload(w, r)
	case r.URL.Path == "/api/file-editor/local":
		a.handleLocalTextEditor(w, r)
	case r.URL.Path == "/api/remote-browser":
		a.handleRemoteBrowser(w, r)
	case r.URL.Path == "/api/connections/open":
		a.handleConnectionOpen(w, r)
	case r.URL.Path == "/api/storage" || strings.HasPrefix(r.URL.Path, "/api/storage/"):
		a.handleStorage(w, r)
	case r.URL.Path == "/api/terminal-window" || strings.HasPrefix(r.URL.Path, "/api/terminal-window/"):
		a.handleTerminalWindow(w, r)
	case r.URL.Path == "/api/credentials" || strings.HasPrefix(r.URL.Path, "/api/credentials/"):
		a.handleCredentials(w, r)
	case r.URL.Path == "/api/system":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		jsonResponse(w, 200, a.getSystem(true))
	case r.URL.Path == "/api/scan-networks":
		a.handleScanNetworks(w, r)
	case r.URL.Path == "/api/state":
		a.handleState(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/vendors/"):
		a.handleVendors(w, r)
	case r.URL.Path == "/api/terminals" || strings.HasPrefix(r.URL.Path, "/api/terminals/"):
		a.handleTerminals(w, r)
	case r.URL.Path == "/api/jobs":
		switch r.Method {
		case http.MethodGet:
			a.writeJobs(w, r.URL.Query().Get("brief") == "1")
		case http.MethodPost:
			a.createJob(w, r)
		case http.MethodDelete:
			a.mu.Lock()
			for k, v := range a.jobs {
				if v.Status != "running" {
					delete(a.jobs, k)
				}
			}
			a.mu.Unlock()
			jsonResponse(w, 200, map[string]bool{"ok": true})
		default:
			methodError(w)
		}
	case strings.HasPrefix(r.URL.Path, "/api/jobs/"):
		a.handleJob(w, r)
	case r.URL.Path == "/api/report":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		var jobs any
		a.mu.RLock()
		b, _ := json.Marshal(a.jobList())
		a.mu.RUnlock()
		_ = json.Unmarshal(b, &jobs)
		w.Header().Set("Content-Disposition", `attachment; filename="Network_Toolkit_Report.json"`)
		jsonResponse(w, 200, map[string]any{"app": appName, "version": model.Version, "exported": time.Now().UTC().Format(time.RFC3339), "system": a.getSystem(false), "jobs": jobs})
	case r.URL.Path == "/api/shutdown":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		// Stop accepting window activations before acknowledging shutdown.
		// The short response grace must not make a closing host look reusable.
		a.beginPrimaryClose()
		jsonResponse(w, 200, map[string]bool{"ok": true, "detached": a.views.count() > 0})
		go func() { time.Sleep(200 * time.Millisecond); a.closePrimary() }()
	default:
		http.NotFound(w, r)
	}
}

func (a *application) getSystem(refresh bool) map[string]any {
	if a.windowMode == "terminal" {
		// A terminal-only startup needs no diagnostic adapter enumeration.
		host, _ := os.Hostname()
		return map[string]any{"hostname": host, "interfaces": []any{}}
	}
	a.systemMu.Lock()
	defer a.systemMu.Unlock()
	if a.system != nil && (!refresh || time.Since(a.systemAt) < time.Second) {
		return a.system
	}
	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()
	sys, err := platform.System(ctx)
	if sys == nil {
		sys = map[string]any{"interfaces": []any{}}
	}
	if err != nil {
		sys["error"] = err.Error()
	}
	a.system = sys
	a.systemAt = time.Now()
	return sys
}

func (a *application) createJob(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		jsonError(w, 415, "Send an application/json request.")
		return
	}
	var req model.Request
	if err := decodeBody(w, r, 64<<10, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	req.Tool = strings.TrimSpace(strings.ToLower(req.Tool))
	req.Target = strings.TrimSpace(req.Target)
	if !allowedTools[req.Tool] {
		jsonError(w, 400, "Unknown diagnostic tool.")
		return
	}
	// Discovery and network-wide ping/TCP requests accept one scope per line.
	// Keep single-target tools strict and bound the larger list independently of
	// the request body limit. The diagnostic parser validates every scope before
	// starting any network activity.
	scope, _ := req.Options["scope"].(string)
	multiScope := req.Tool == "discover" || ((req.Tool == "ping" || req.Tool == "tcp") && strings.TrimSpace(scope) == "subnet")
	maxTargetBytes := 2048
	if multiScope {
		maxTargetBytes = 16 << 10
	}
	if len(req.Target) > maxTargetBytes || strings.ContainsRune(req.Target, '\x00') || (!multiScope && strings.ContainsAny(req.Target, "\r\n")) {
		jsonError(w, 400, "Invalid or excessively long target.")
		return
	}
	if req.Options == nil {
		req.Options = map[string]any{}
	}
	if len(req.Options) > 30 {
		jsonError(w, 400, "Too many diagnostic options.")
		return
	}
	// Reject malformed source selections before allocating a job. Adapter state
	// and address ownership are checked again by the diagnostic immediately
	// before its probes, since the local interfaces may change after selection.
	if err := diag.ValidateSourceOptions(req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	a.mu.Lock()
	running := 0
	for _, v := range a.jobs {
		if v.Status == "running" {
			running++
		}
	}
	if running >= maxConcurrent {
		a.mu.Unlock()
		jsonError(w, 429, "Eight diagnostics are already running. Stop one or wait for completion.")
		return
	}
	for len(a.jobs) >= maxRetained {
		var oldest *Job
		for _, v := range a.jobs {
			if v.Status != "running" && (oldest == nil || v.Started < oldest.Started) {
				oldest = v
			}
		}
		if oldest == nil {
			break
		}
		delete(a.jobs, oldest.ID)
	}
	ctx, cancel := diagnosticJobContext(a.ctx, req)
	safeOptions := make(map[string]any, len(req.Options))
	for key, value := range req.Options {
		if key != "password" && key != "passphrase" && key != "privateKey" {
			safeOptions[key] = value
		}
	}
	j := &Job{ID: randomHex(12), Tool: req.Tool, Target: req.Target, Options: safeOptions, Status: "running", Started: time.Now().UTC().Format(time.RFC3339Nano), Events: []model.Event{}, cancel: cancel}
	a.jobs[j.ID] = j
	b, _ := json.Marshal(j)
	a.mu.Unlock()
	go func() {
		defer cancel()
		result, err := runDiagnostic(ctx, req, func(event model.Event) {
			a.recordJobEvent(j, event)
		})
		a.mu.Lock()
		defer a.mu.Unlock()
		j.Finished = time.Now().UTC().Format(time.RFC3339Nano)
		j.Result = &result
		// The completed result contains every started host, including partial
		// cancelled probes. Its rows now replace the transient live index.
		j.LiveHosts = nil
		if ctx.Err() != nil {
			j.Status = "cancelled"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				j.Error = "Stopped after the maximum diagnostic duration."
			} else if result.Summary == "" {
				result.Summary = "Diagnostic stopped."
			} else {
				result.Summary = "Stopped. " + result.Summary
			}
		} else if err != nil {
			j.Status = "failed"
			j.Error = err.Error()
		} else {
			j.Status = "completed"
		}
	}()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(b)
}

// EventSequence keeps live polling observable after the retained event window
// fills. The number of stored events alone no longer changes at that point.
func (a *application) recordJobEvent(j *Job, event model.Event) {
	if event.Time == "" {
		event.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if len(event.Message) > 16384 {
		event.Message = event.Message[:16384] + " [truncated]"
	}
	// Freeze event payloads before the diagnostic can reuse its buffers.
	if data, e := json.Marshal(event.Data); e == nil {
		if len(data) > 256<<10 {
			event.Data = map[string]string{"note": "Large event details omitted; see completed results."}
		} else {
			var safe any
			_ = json.Unmarshal(data, &safe)
			event.Data = safe
		}
	}
	a.mu.Lock()
	// Activity history is bounded independently of the live inventory. A slow
	// scan can publish many progress events before its last host finishes;
	// retaining each latest immutable host prevents earlier rows disappearing.
	if envelope, ok := event.Data.(map[string]any); ok && envelope["type"] == "sample" {
		if stats, ok := envelope["statistics"].(map[string]any); ok {
			j.SampleStatistics = stats
		}
	}
	if envelope, ok := event.Data.(map[string]any); ok && (envelope["type"] == "host" || envelope["type"] == "subnet-host") {
		if row, ok := envelope["value"].(map[string]any); ok {
			address, _ := row["address"].(string)
			if ip := net.ParseIP(address); ip != nil && ip.To4() != nil {
				if j.LiveHosts == nil {
					j.LiveHosts = make(map[string]any)
				}
				if _, exists := j.LiveHosts[address]; exists || len(j.LiveHosts) < maxLiveHosts {
					j.LiveHosts[address] = row
				}
			}
		}
	}
	if len(j.Events) >= maxEvents {
		// Live graph rows come from the retained activity window, which is
		// smaller than the completed diagnostic sample window. Summarize
		// each discarded original before trimming, including on reconnect.
		if j.Tool == "ping" || j.Tool == "monitor" {
			for _, old := range j.Events[:len(j.Events)-maxEvents+1] {
				if data, ok := old.Data.(map[string]any); ok && data["type"] == "sample" {
					if row, ok := data["value"].(map[string]any); ok {
						encoded, err := json.Marshal(row)
						var sample platform.PingReply
						if err == nil && json.Unmarshal(encoded, &sample) == nil {
							if sample.Time == "" { sample.Time = old.Time }
							if j.SampleHistory == nil { j.SampleHistory = &diag.SampleHistory{} }
							j.SampleHistory.Add(sample)
						}
					}
				}
			}
		}
		j.Events = append(j.Events[:0], j.Events[len(j.Events)-maxEvents+1:]...)
	}
	j.Events = append(j.Events, event)
	j.EventSequence++
	a.mu.Unlock()
}

func (a *application) jobList() []*Job {
	list := make([]*Job, 0, len(a.jobs))
	for _, j := range a.jobs {
		list = append(list, j)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Started > list[j].Started })
	return list
}

func (a *application) writeJobs(w http.ResponseWriter, brief bool) {
	a.mu.RLock()
	list := a.jobList()
	var b []byte
	if brief {
		items := make([]map[string]any, 0, len(list))
		for _, j := range list {
			var summary string
			if j.Result != nil {
				summary = j.Result.Summary
			}
			items = append(items, map[string]any{"id": j.ID, "tool": j.Tool, "target": j.Target, "status": j.Status, "started": j.Started, "finished": j.Finished, "error": j.Error, "summary": summary, "eventCount": len(j.Events), "eventSequence": j.EventSequence})
		}
		b, _ = json.Marshal(items)
	} else {
		b, _ = json.Marshal(list)
	}
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(b)
}

func (a *application) handleJob(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/jobs/"), "/")
	if len(parts) > 2 || len(parts[0]) != 24 {
		jsonError(w, 404, "Diagnostic not found.")
		return
	}
	a.mu.RLock()
	j, ok := a.jobs[parts[0]]
	if !ok {
		a.mu.RUnlock()
		jsonError(w, 404, "Diagnostic not found.")
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" {
		cancel := j.cancel
		a.mu.RUnlock()
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		cancel()
		jsonResponse(w, 200, map[string]bool{"ok": true})
		return
	}
	if len(parts) != 1 {
		a.mu.RUnlock()
		jsonError(w, 404, "Diagnostic not found.")
		return
	}
	if r.Method != http.MethodGet {
		a.mu.RUnlock()
		methodError(w)
		return
	}
	b, _ := json.Marshal(j)
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(b)
}

func (a *application) handleState(w http.ResponseWriter, r *http.Request) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(a.state)
	case http.MethodPut:
		var state map[string]any
		if err := decodeBody(w, r, 2<<20, &state); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		if state == nil {
			jsonError(w, 400, "Settings must be a JSON object.")
			return
		}
		sanitizeTerminalSettings(state)
		sanitizeFTPProfiles(state)
		sanitizeTransferClientSettings(state)
		sanitizeListPreferences(state)
		b, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			jsonError(w, 400, "Invalid settings.")
			return
		}
		if len(b) > 2<<20 {
			jsonError(w, 400, "Saved settings would exceed the 2 MiB limit. Reduce the saved data first.")
			return
		}
		if err = writeAtomic(a.dataPath, b); err != nil {
			a.storageWritable = false
			a.storageError = err.Error()
			jsonError(w, 500, "Settings were not saved: "+err.Error())
			return
		}
		a.state = b
		a.storageWritable = true
		a.storageError = ""
		jsonResponse(w, 200, map[string]bool{"ok": true})
	default:
		methodError(w)
	}
}

func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".network-toolkit-settings-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return replaceSettingsFile(tmp, path)
}

func decodeBody(w http.ResponseWriter, r *http.Request, limit int64, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("Invalid JSON request: %v", err)
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("Request must contain one JSON value")
	}
	return nil
}

func jsonResponse(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		jsonError(w, 500, "Unable to encode response.")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func methodError(w http.ResponseWriter) {
	jsonError(w, http.StatusMethodNotAllowed, "This method is not supported.")
}

func runDiagnostic(ctx context.Context, req model.Request, emit model.Emit) (result model.Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("The diagnostic stopped unexpectedly: %v", recovered)
			model.Log(emit, "error", err.Error(), nil)
		}
	}()
	return diag.Run(ctx, req, emit)
}
