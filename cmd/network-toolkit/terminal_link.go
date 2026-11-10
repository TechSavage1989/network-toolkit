package main

import (
	"errors"
	"flag"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Terminal links (ssh://, telnet:// and serial:) let Windows Default apps send
// a connection to the Network Toolkit Terminal. A link is only a destination:
// it opens the normal connection form for the user to confirm. Nothing in a
// link is executed, no password is ever taken from it, and it never selects a
// saved credential, host key or profile by itself. The link is held in memory
// only (terminalWindowInbox); it is not written to settings and the updater
// never replays it (updateRestartArgs drops --open-url).

const (
	terminalLinkFlag        = "open-url"
	terminalLinkMaxLength   = 2048
	terminalLinkMaxUsername = 256
	terminalLinkMaxHost     = 253
)

var errTerminalLinkInvalid = errors.New("This link is not a supported SSH, Telnet or serial link. Use ssh://[user@]host[:port], telnet://host[:port] or serial:COM3.")

// terminalWindowLink is the nonsecret description of where a request came
// from. The window uses it only to explain the request to the user.
type terminalWindowLink struct {
	Scheme          string `json:"scheme"`
	PasswordIgnored bool   `json:"passwordIgnored,omitempty"`
}

type terminalLink struct {
	Preset terminalWindowPreset
	Link   terminalWindowLink
}

// parseTerminalLink accepts exactly:
//
//	ssh://[user[;params]@]host[:port][/]
//	telnet://[user@]host[:port][/]
//	serial:DEVICE[?baud=&data=&parity=&stop=&flow=]  (also serial://DEVICE)
//
// DEVICE is COM<n> on Windows and /dev/ttyS*, ttyUSB*, ttyACM*, ttyAMA* or
// ttyTHS* on Linux. Any password (user:password@) is dropped and reported
// through Link.PasswordIgnored. Everything else is rejected.
func parseTerminalLink(raw, goos string) (terminalLink, error) {
	var out terminalLink
	if raw == "" || len(raw) > terminalLinkMaxLength {
		return out, errTerminalLinkInvalid
	}
	for i := 0; i < len(raw); i++ {
		if !terminalLinkURLByte(raw[i]) {
			// Spaces, quotes, control characters, backslashes and non-ASCII
			// bytes never appear in a valid link; refuse rather than repair.
			return out, errTerminalLinkInvalid
		}
	}
	if strings.ContainsRune(raw, '#') {
		return out, errTerminalLinkInvalid
	}
	scheme, rest, ok := strings.Cut(raw, ":")
	if !ok {
		return out, errTerminalLinkInvalid
	}
	scheme = strings.ToLower(scheme)
	var err error
	switch scheme {
	case "ssh", "telnet":
		out, err = parseTerminalNetworkLink(scheme, rest)
	case "serial":
		out, err = parseTerminalSerialLink(rest, goos)
	default:
		return out, errTerminalLinkInvalid
	}
	if err != nil {
		return terminalLink{}, err
	}
	out.Link.Scheme = scheme
	if err = out.Preset.normalize(); err != nil {
		return terminalLink{}, err
	}
	if err = out.Link.validate(&out.Preset); err != nil {
		return terminalLink{}, err
	}
	return out, nil
}

// RFC 3986 unreserved, reserved and percent characters only.
func terminalLinkURLByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("-._~:/?#[]@!$&'()*+,;=%", c) >= 0
}

func parseTerminalNetworkLink(scheme, rest string) (terminalLink, error) {
	var out terminalLink
	if !strings.HasPrefix(rest, "//") {
		return out, errTerminalLinkInvalid
	}
	rest = rest[2:]
	authority, tail := rest, ""
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		authority, tail = rest[:i], rest[i:]
	}
	// A trailing slash is tolerated; paths, queries and commands are not.
	if tail != "" && tail != "/" {
		return out, errors.New("SSH and Telnet links can contain only a user, host and port. Paths, commands and options are not supported.")
	}
	userinfo, hostport, hasUser := "", authority, false
	if i := strings.LastIndexByte(authority, '@'); i >= 0 {
		userinfo, hostport, hasUser = authority[:i], authority[i+1:], true
	}
	username, passwordIgnored := "", false
	if hasUser {
		info := userinfo
		if scheme == "ssh" {
			// draft-ietf-secsh-scp-sftp-ssh-uri connection parameters, such as
			// PuTTY's ;fingerprint=..., are ignored: host keys are verified by
			// the Terminal's own SSH host-key checks, never by a link.
			info, _, _ = strings.Cut(info, ";")
		} else if strings.ContainsRune(info, ';') {
			return out, errTerminalLinkInvalid
		}
		user, password, hasPassword := strings.Cut(info, ":")
		passwordIgnored = hasPassword && password != ""
		decoded, err := terminalLinkUnescape(user)
		if err != nil {
			return out, err
		}
		if err = terminalLinkCheckUsername(decoded); err != nil {
			return out, err
		}
		username = decoded
	}
	host, port, err := terminalLinkHostPort(hostport)
	if err != nil {
		return out, err
	}
	if port == 0 {
		port = 22
		if scheme == "telnet" {
			port = 23
		}
	}
	out.Preset = terminalWindowPreset{Type: scheme, Host: host, Port: port}
	if scheme == "ssh" {
		out.Preset.Username = username
	}
	// Telnet sign-in happens inside the session, so a Telnet user name is not
	// used by the connection form. It is accepted and left out.
	out.Link.PasswordIgnored = passwordIgnored
	return out, nil
}

func terminalLinkUnescape(value string) (string, error) {
	// PathUnescape keeps '+' literal and rejects malformed %XX sequences.
	decoded, err := url.PathUnescape(value)
	if err != nil || !utf8.ValidString(decoded) {
		return "", errTerminalLinkInvalid
	}
	return decoded, nil
}

func terminalLinkCheckUsername(user string) error {
	if len(user) > terminalLinkMaxUsername || strings.HasPrefix(user, "-") {
		return errors.New("The user name in this link is too long or not valid.")
	}
	for _, r := range user {
		if unicode.IsControl(r) || unicode.IsSpace(r) || r == utf8.RuneError {
			return errors.New("The user name in this link contains spaces or control characters.")
		}
	}
	return nil
}

func terminalLinkHostPort(hostport string) (string, int, error) {
	var host, port string
	if strings.HasPrefix(hostport, "[") {
		end := strings.IndexByte(hostport, ']')
		if end < 0 {
			return "", 0, errTerminalLinkInvalid
		}
		host, port = hostport[:end+1], hostport[end+1:]
		if port != "" {
			if port[0] != ':' || len(port) == 1 {
				return "", 0, errTerminalLinkInvalid
			}
			port = port[1:]
		}
	} else {
		if strings.Count(hostport, ":") > 1 {
			// An IPv6 address must be written in brackets.
			return "", 0, errTerminalLinkInvalid
		}
		var hasPort bool
		host, port, hasPort = strings.Cut(hostport, ":")
		if hasPort && port == "" {
			return "", 0, errors.New("The port in this link must be between 1 and 65535.")
		}
	}
	decoded, err := terminalLinkUnescape(host)
	if err != nil {
		return "", 0, err
	}
	if decoded == "" || len(decoded) > terminalLinkMaxHost+2 {
		return "", 0, errors.New("The link does not name a valid host.")
	}
	// The same strict destination rules as connection launch: an IP address or
	// ASCII DNS name, no scope IDs, no unspecified, multicast or broadcast
	// addresses, and no numeric IPv4 aliases.
	if decoded, err = connectionOpenHost(decoded); err != nil {
		return "", 0, errors.New("The link does not name a valid host: " + err.Error())
	}
	number := 0
	if port != "" {
		if len(port) > 5 {
			return "", 0, errors.New("The port in this link must be between 1 and 65535.")
		}
		for _, c := range port {
			if c < '0' || c > '9' {
				return "", 0, errors.New("The port in this link must be between 1 and 65535.")
			}
		}
		number, _ = strconv.Atoi(port)
		if number < 1 || number > 65535 {
			return "", 0, errors.New("The port in this link must be between 1 and 65535.")
		}
	}
	return decoded, number, nil
}

func parseTerminalSerialLink(rest, goos string) (terminalLink, error) {
	var out terminalLink
	device, query, _ := strings.Cut(rest, "?")
	if strings.HasPrefix(device, "//") {
		device = device[2:]
	}
	if goos == "windows" {
		// serial://COM3/ is accepted the same way as an HTTP trailing slash.
		device = strings.TrimSuffix(device, "/")
	}
	decoded, err := terminalLinkUnescape(device)
	if err != nil {
		return out, err
	}
	port, err := terminalLinkSerialDevice(decoded, goos)
	if err != nil {
		return out, err
	}
	out.Preset = terminalWindowPreset{Type: "serial", SerialPort: port, BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}
	if query == "" {
		return out, nil
	}
	seen := map[string]bool{}
	for _, pair := range strings.Split(query, "&") {
		key, value, ok := strings.Cut(pair, "=")
		key = strings.ToLower(key)
		value = strings.ToLower(value)
		if !ok || value == "" || seen[key] {
			return out, errors.New("The serial settings in this link are not valid. Use baud, data, parity, stop and flow once each.")
		}
		seen[key] = true
		switch key {
		case "baud":
			n, ok := terminalLinkDigits(value, 7)
			if !ok || n < 50 || n > 4000000 {
				return out, errors.New("The baud rate in this link must be between 50 and 4000000.")
			}
			out.Preset.BaudRate = n
		case "data":
			n, ok := terminalLinkDigits(value, 1)
			if !ok || n < 5 || n > 8 {
				return out, errors.New("Data bits in this link must be 5, 6, 7 or 8.")
			}
			out.Preset.DataBits = n
		case "parity":
			parity := map[string]string{"none": "none", "n": "none", "odd": "odd", "o": "odd", "even": "even", "e": "even", "mark": "mark", "m": "mark", "space": "space", "s": "space"}[value]
			if parity == "" {
				return out, errors.New("Parity in this link must be none, odd, even, mark or space.")
			}
			out.Preset.Parity = parity
		case "stop":
			if value != "1" && value != "1.5" && value != "2" {
				return out, errors.New("Stop bits in this link must be 1, 1.5 or 2.")
			}
			out.Preset.StopBits = value
		case "flow":
			if value != "none" && value != "xonxoff" && value != "rtscts" {
				return out, errors.New("Flow control in this link must be none, xonxoff or rtscts.")
			}
			out.Preset.FlowControl = value
		default:
			return out, errors.New("The serial settings in this link are not valid. Use baud, data, parity, stop and flow once each.")
		}
	}
	// The same combinations the serial transport accepts.
	if out.Preset.StopBits == "1.5" && out.Preset.DataBits != 5 {
		return out, errors.New("1.5 stop bits requires 5 data bits.")
	}
	if out.Preset.StopBits == "2" && out.Preset.DataBits == 5 {
		return out, errors.New("2 stop bits requires 6, 7 or 8 data bits.")
	}
	return out, nil
}

func terminalLinkDigits(value string, max int) (int, bool) {
	if value == "" || len(value) > max {
		return 0, false
	}
	n := 0
	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func terminalLinkSerialDevice(device, goos string) (string, error) {
	switch goos {
	case "windows":
		upper := strings.ToUpper(device)
		if strings.HasPrefix(upper, "COM") {
			digits := upper[3:]
			// The same COM range as the serial transport (normalizeSerialPort).
			if n, ok := terminalLinkDigits(digits, 5); ok && digits[0] != '0' && n >= 1 && n <= 65535 {
				return "COM" + digits, nil
			}
		}
		return "", errors.New("Serial links on Windows name a COM port, such as serial:COM3.")
	case "linux":
		// Only real serial adapters; never pseudo-terminals of other sessions.
		for _, prefix := range []string{"/dev/ttyS", "/dev/ttyUSB", "/dev/ttyACM", "/dev/ttyAMA", "/dev/ttyTHS"} {
			if suffix, ok := strings.CutPrefix(device, prefix); ok {
				if _, digits := terminalLinkDigits(suffix, 4); digits {
					return device, nil
				}
			}
		}
		return "", errors.New("Serial links on Linux name a serial device, such as serial:/dev/ttyUSB0.")
	}
	return "", errors.New("Serial links are not supported on this system.")
}

func (l *terminalWindowLink) validate(p *terminalWindowPreset) error {
	if l == nil {
		return nil
	}
	if p == nil || l.Scheme != p.Type {
		return errTerminalLinkInvalid
	}
	switch p.Type {
	case "ssh", "telnet":
		if p.Host == "" || p.SerialPort != "" || (p.Type == "telnet" && p.Username != "") {
			return errTerminalLinkInvalid
		}
	case "serial":
		if p.SerialPort == "" || p.Host != "" || p.Username != "" {
			return errTerminalLinkInvalid
		}
	default:
		return errTerminalLinkInvalid
	}
	// A link never selects a saved profile, a local folder or elevation.
	if p.ProfileID != "" || p.WorkingDirectory != "" || p.RunAsAdministrator {
		return errTerminalLinkInvalid
	}
	return nil
}

// terminalLinkLaunch reads --open-url. Windows substitutes the clicked link
// into "... --terminal --open-url "%1"", so a crafted link containing quotes
// could otherwise smuggle in further switches. Exactly --terminal and
// --open-url, and no positional arguments, are accepted with a link.
func terminalLinkLaunch(flags *flag.FlagSet, goos string) (*terminalLink, error) {
	set, terminalSet, other := false, false, false
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case terminalLinkFlag:
			set = true
		case "terminal":
			terminalSet = f.Value.String() == "true"
		default:
			other = true
		}
	})
	if !set {
		return nil, nil
	}
	if !terminalSet || other || flags.NArg() != 0 {
		return nil, errors.New("A terminal link is opened with exactly: Network_Toolkit.exe --terminal --open-url \"<link>\".")
	}
	link, err := parseTerminalLink(flags.Lookup(terminalLinkFlag).Value.String(), goos)
	if err != nil {
		return nil, err
	}
	return &link, nil
}
