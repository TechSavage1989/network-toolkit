package main

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestTerminalLinkParsesSupportedLinks(t *testing.T) {
	cases := []struct {
		raw, goos string
		want      terminalWindowPreset
		password  bool
	}{
		{"ssh://router.example.com", "windows", terminalWindowPreset{Type: "ssh", Host: "router.example.com", Port: 22}, false},
		{"ssh://router.example.com/", "windows", terminalWindowPreset{Type: "ssh", Host: "router.example.com", Port: 22}, false},
		{"SSH://Admin@Router.Example.com:2222", "windows", terminalWindowPreset{Type: "ssh", Host: "router.example.com", Port: 2222, Username: "Admin"}, false},
		{"ssh://operator@192.0.2.10:22/", "linux", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22, Username: "operator"}, false},
		{"ssh://operator@[2001:db8::1]:830", "windows", terminalWindowPreset{Type: "ssh", Host: "2001:db8::1", Port: 830, Username: "operator"}, false},
		{"ssh://[2001:db8::1]", "windows", terminalWindowPreset{Type: "ssh", Host: "2001:db8::1", Port: 22}, false},
		{"ssh://operator:hunter2@192.0.2.10", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22, Username: "operator"}, true},
		{"ssh://operator:@192.0.2.10", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22, Username: "operator"}, false},
		{"ssh://operator;fingerprint=ssh-ed25519-AAAAC3NzaC1lZDI1NTE5@192.0.2.10", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22, Username: "operator"}, false},
		{"ssh://operator:secret;fingerprint=ssh-rsa-00-11@192.0.2.10:2200", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 2200, Username: "operator"}, true},
		{"ssh://;fingerprint=ssh-rsa-00@192.0.2.10", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22}, false},
		{"ssh://first%2Elast%40corp.example@192.0.2.10", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22, Username: "first.last@corp.example"}, false},
		{"ssh://op%C3%A9rateur@192.0.2.10", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22, Username: "opérateur"}, false},
		{"ssh://%31%39%32.0.2.10", "windows", terminalWindowPreset{Type: "ssh", Host: "192.0.2.10", Port: 22}, false},
		{"ssh://switch-01.lab.example.:22", "windows", terminalWindowPreset{Type: "ssh", Host: "switch-01.lab.example.", Port: 22}, false},
		{"ssh://host:00022", "windows", terminalWindowPreset{Type: "ssh", Host: "host", Port: 22}, false},
		{"ssh://host:65535", "windows", terminalWindowPreset{Type: "ssh", Host: "host", Port: 65535}, false},
		{"telnet://192.0.2.20", "windows", terminalWindowPreset{Type: "telnet", Host: "192.0.2.20", Port: 23}, false},
		{"telnet://console.example.com:2001/", "windows", terminalWindowPreset{Type: "telnet", Host: "console.example.com", Port: 2001}, false},
		{"telnet://admin@192.0.2.20:2323", "windows", terminalWindowPreset{Type: "telnet", Host: "192.0.2.20", Port: 2323}, false},
		{"telnet://admin:pw@192.0.2.20", "windows", terminalWindowPreset{Type: "telnet", Host: "192.0.2.20", Port: 23}, true},
		{"serial:COM3", "windows", terminalWindowPreset{Type: "serial", SerialPort: "COM3", BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}, false},
		{"serial:com12", "windows", terminalWindowPreset{Type: "serial", SerialPort: "COM12", BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}, false},
		{"serial://COM4/", "windows", terminalWindowPreset{Type: "serial", SerialPort: "COM4", BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}, false},
		{"serial://COM4?baud=115200&data=7&parity=e&stop=2&flow=rtscts", "windows", terminalWindowPreset{Type: "serial", SerialPort: "COM4", BaudRate: 115200, DataBits: 7, StopBits: "2", Parity: "even", FlowControl: "rtscts"}, false},
		{"serial:COM1?PARITY=Mark&Flow=XONXOFF", "windows", terminalWindowPreset{Type: "serial", SerialPort: "COM1", BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "mark", FlowControl: "xonxoff"}, false},
		{"serial:COM2?data=5&stop=1.5&parity=s", "windows", terminalWindowPreset{Type: "serial", SerialPort: "COM2", BaudRate: 9600, DataBits: 5, StopBits: "1.5", Parity: "space", FlowControl: "none"}, false},
		{"serial:/dev/ttyUSB0?baud=57600", "linux", terminalWindowPreset{Type: "serial", SerialPort: "/dev/ttyUSB0", BaudRate: 57600, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}, false},
		{"serial:///dev/ttyACM1", "linux", terminalWindowPreset{Type: "serial", SerialPort: "/dev/ttyACM1", BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}, false},
		{"serial:/dev/ttyS0?parity=o", "linux", terminalWindowPreset{Type: "serial", SerialPort: "/dev/ttyS0", BaudRate: 9600, DataBits: 8, StopBits: "1", Parity: "odd", FlowControl: "none"}, false},
	}
	for _, tc := range cases {
		got, err := parseTerminalLink(tc.raw, tc.goos)
		if err != nil {
			t.Fatalf("%s (%s): %v", tc.raw, tc.goos, err)
		}
		want := tc.want
		if err := want.normalize(); err != nil {
			t.Fatal(err)
		}
		if got.Preset != want {
			t.Fatalf("%s: preset\n got %+v\nwant %+v", tc.raw, got.Preset, want)
		}
		if got.Link.PasswordIgnored != tc.password || got.Link.Scheme != want.Type {
			t.Fatalf("%s: link %+v, want password ignored %v", tc.raw, got.Link, tc.password)
		}
		if got.Preset.ProfileID != "" || got.Preset.WorkingDirectory != "" || got.Preset.RunAsAdministrator {
			t.Fatalf("%s selected a profile or local shell option", tc.raw)
		}
	}
}

func TestTerminalLinkNeverCarriesPasswords(t *testing.T) {
	for _, raw := range []string{"ssh://operator:Secr3t-Value@192.0.2.10", "ssh://operator:Secr3t-Value;fingerprint=ssh-ed25519-00@192.0.2.10", "telnet://operator:Secr3t-Value@192.0.2.10"} {
		got, err := parseTerminalLink(raw, "windows")
		if err != nil {
			t.Fatal(err)
		}
		if !got.Link.PasswordIgnored {
			t.Fatalf("%s: the ignored password was not reported", raw)
		}
		encoded, _ := terminalChildArguments(t.TempDir(), &got.Preset, false)
		for _, part := range append(encoded, got.Preset.Host, got.Preset.Username) {
			if strings.Contains(part, "Secr3t") {
				t.Fatalf("%s: password reached the preset: %q", raw, part)
			}
		}
	}
}

func TestTerminalLinkRejectsEverythingElse(t *testing.T) {
	rejected := []struct{ raw, goos string }{
		{"", "windows"},
		{"ssh:", "windows"},
		{"ssh:router", "windows"},
		{"ssh:/router", "windows"},
		{"ssh://", "windows"},
		{"ssh://user@", "windows"},
		{"ssh://:22", "windows"},
		{"http://router.example.com", "windows"},
		{"https://router.example.com", "windows"},
		{"file:///C:/Windows/System32/cmd.exe", "windows"},
		{"rdp://192.0.2.1", "windows"},
		{"sftp://192.0.2.1", "windows"},
		{"javascript:alert(1)", "windows"},
		{"ssh2://192.0.2.1", "windows"},
		{"ssh://192.0.2.1/show running-config", "windows"},
		{"ssh://192.0.2.1/reload", "windows"},
		{"ssh://192.0.2.1//", "windows"},
		{"ssh://192.0.2.1/%2e%2e", "windows"},
		{"ssh://192.0.2.1?command=reload", "windows"},
		{"ssh://192.0.2.1/?x=1", "windows"},
		{"telnet://192.0.2.1/?x=1", "windows"},
		{"ssh://192.0.2.1#frag", "windows"},
		{"ssh://192.0.2.1 --data-dir x", "windows"},
		{"ssh://192.0.2.1\" --data-dir \"C:\\x", "windows"},
		{"ssh://192.0.2.1\n", "windows"},
		{"ssh://192.0.2.1\r\nshow", "windows"},
		{"ssh://192.0.2.1\t", "windows"},
		{" ssh://192.0.2.1", "windows"},
		{"ssh://192.0.2.1\x00", "windows"},
		{"ssh://rou\u00e9ter.example", "windows"},
		{"ssh://192.0.2.1\\x", "windows"},
		{"ssh://<host>", "windows"},
		{"ssh://ho%20st", "windows"},
		{"ssh://ho%0ast", "windows"},
		{"ssh://ho%00st", "windows"},
		{"ssh://ho%zzst", "windows"},
		{"ssh://ho%2fst", "windows"},
		{"ssh://host%3A22", "windows"},
		{"ssh://us%0Aer@192.0.2.1", "windows"},
		{"ssh://us%20er@192.0.2.1", "windows"},
		{"ssh://us%00er@192.0.2.1", "windows"},
		{"ssh://us%e9er@192.0.2.1", "windows"},
		{"ssh://-oProxyCommand=calc@192.0.2.1", "windows"},
		{"ssh://" + strings.Repeat("u", 257) + "@192.0.2.1", "windows"},
		{"ssh://-host.example", "windows"},
		{"ssh://host-.example", "windows"},
		{"ssh://host..example", "windows"},
		{"ssh://" + strings.Repeat("a", 64) + ".example", "windows"},
		{"ssh://" + strings.Repeat("abcdefgh.", 29) + "com", "windows"},
		{"ssh://host_name.example", "windows"},
		{"ssh://192.0.2.1:0", "windows"},
		{"ssh://192.0.2.1:65536", "windows"},
		{"ssh://192.0.2.1:99999", "windows"},
		{"ssh://192.0.2.1:123456", "windows"},
		{"ssh://192.0.2.1:-22", "windows"},
		{"ssh://192.0.2.1:+22", "windows"},
		{"ssh://192.0.2.1:2a", "windows"},
		{"ssh://192.0.2.1:22:22", "windows"},
		{"ssh://192.0.2.1:", "windows"},
		{"ssh://2001:db8::1", "windows"},
		{"ssh://[2001:db8::1", "windows"},
		{"ssh://[2001:db8::1]x", "windows"},
		{"ssh://[2001:db8::1]:", "windows"},
		{"ssh://[fe80::1%25eth0]", "windows"},
		{"ssh://[192.0.2.1]", "windows"},
		{"ssh://[v1.fe80::a+en1]", "windows"},
		{"ssh://0.0.0.0", "windows"},
		{"ssh://[::]", "windows"},
		{"ssh://224.0.0.1", "windows"},
		{"ssh://255.255.255.255", "windows"},
		{"ssh://3232235777", "windows"},
		{"ssh://0x7f000001", "windows"},
		{"ssh://192.168.1", "windows"},
		{"telnet://user;opt@192.0.2.1", "windows"},
		{"telnet://192.0.2.1:0", "windows"},
		{"telnet:192.0.2.1", "windows"},
		{"serial:", "windows"},
		{"serial://", "windows"},
		{"serial:COM", "windows"},
		{"serial:COM0", "windows"},
		{"serial:COM03", "windows"},
		{"serial:COM65536", "windows"},
		{"serial:COM3x", "windows"},
		{"serial:LPT1", "windows"},
		{"serial:\\\\.\\COM3", "windows"},
		{"serial:%5C%5C.%5CCOM3", "windows"},
		{"serial:NUL", "windows"},
		{"serial:COM3/extra", "windows"},
		{"serial:COM3?baud=", "windows"},
		{"serial:COM3?baud", "windows"},
		{"serial:COM3?baud=49", "windows"},
		{"serial:COM3?baud=4000001", "windows"},
		{"serial:COM3?baud=9600&baud=115200", "windows"},
		{"serial:COM3?baud=9600&&data=8", "windows"},
		{"serial:COM3?data=4", "windows"},
		{"serial:COM3?data=9", "windows"},
		{"serial:COM3?parity=x", "windows"},
		{"serial:COM3?stop=3", "windows"},
		{"serial:COM3?stop=1.5", "windows"},
		{"serial:COM3?stop=2&data=5", "windows"},
		{"serial:COM3?flow=dsrdtr", "windows"},
		{"serial:COM3?command=reload", "windows"},
		{"serial:COM3?baud=%39600", "windows"},
		{"serial:/dev/ttyUSB0", "windows"},
		{"serial:COM3", "linux"},
		{"serial:/dev/pts/3", "linux"},
		{"serial:/dev/tty", "linux"},
		{"serial:/dev/ttyUSB", "linux"},
		{"serial:/dev/ttyUSB0/", "linux"},
		{"serial:/dev/ttyUSB0/../sda", "linux"},
		{"serial:/dev/sda", "linux"},
		{"serial:/etc/passwd", "linux"},
		{"serial://dev/ttyUSB0", "linux"},
		{"serial:/dev/ttyUSB12345", "linux"},
		{"serial:COM3", "darwin"},
		{"serial:/dev/ttyUSB0", "darwin"},
		{"ssh://" + strings.Repeat("a", terminalLinkMaxLength), "windows"},
	}
	for _, tc := range rejected {
		if got, err := parseTerminalLink(tc.raw, tc.goos); err == nil {
			t.Fatalf("%q (%s) was accepted: %+v", tc.raw, tc.goos, got)
		}
	}
}

// Fuzz-style coverage: every one-byte mutation of a set of valid links either
// fails or yields a normalized destination without control characters,
// secrets, paths or local-shell options.
func TestTerminalLinkMutationsStayWithinPolicy(t *testing.T) {
	seeds := []string{"ssh://operator:Zq9@192.0.2.10:22/", "telnet://[2001:db8::1]:23", "serial:COM3?baud=9600&parity=n", "ssh://op;fingerprint=x@host.example"}
	alphabet := []byte{0, '\n', '\r', ' ', '"', '\\', '/', '?', '#', '%', '@', ':', ';', '[', ']', '&', '=', '-', 'a', '0', 0x80, 0xff}
	checked := 0
	for _, seed := range seeds {
		for i := 0; i <= len(seed); i++ {
			for _, c := range alphabet {
				for _, mutated := range []string{seed[:i] + string([]byte{c}) + seed[i:], seed[:min(i, len(seed)-1)] + string([]byte{c}) + seed[min(i+1, len(seed)):]} {
					for _, goos := range []string{"windows", "linux"} {
						got, err := parseTerminalLink(mutated, goos)
						checked++
						if err != nil {
							continue
						}
						p := got.Preset
						for _, field := range []string{p.Host, p.Username, p.SerialPort} {
							if strings.ContainsAny(field, "\x00\r\n\t \"\\/?#") && !(p.Type == "serial" && goos == "linux" && strings.HasPrefix(field, "/dev/tty")) {
								t.Fatalf("%q produced unsafe field %q", mutated, field)
							}
							// While the link still marks Zq9 as a password, it must not
							// reach any field.
							if strings.Contains(mutated, ":Zq9@") && strings.Contains(field, "Zq9") {
								t.Fatalf("%q leaked the password into %q", mutated, field)
							}
						}
						if p.ProfileID != "" || p.WorkingDirectory != "" || p.RunAsAdministrator || p.Port < 1 || p.Port > 65535 {
							t.Fatalf("%q produced an unsafe preset %+v", mutated, p)
						}
						if p.Type != "ssh" && p.Type != "telnet" && p.Type != "serial" {
							t.Fatalf("%q produced type %q", mutated, p.Type)
						}
					}
				}
			}
		}
	}
	if checked < 1000 {
		t.Fatalf("Only %d mutations were checked", checked)
	}
}

func TestTerminalLinkLaunchAllowsOnlyTheRegisteredCommand(t *testing.T) {
	parse := func(args ...string) (*terminalLink, error) {
		flags := flag.NewFlagSet("test", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		flags.Bool("terminal", false, "")
		flags.String("terminal-preset", "", "")
		flags.String("data-dir", "", "")
		flags.Bool("no-browser", false, "")
		flags.Int("port", 0, "")
		flags.String(terminalLinkFlag, "", "")
		if err := flags.Parse(args); err != nil {
			return nil, err
		}
		return terminalLinkLaunch(flags, "windows")
	}
	if link, err := parse("--terminal"); err != nil || link != nil {
		t.Fatalf("Plain Terminal launch changed: %v %v", link, err)
	}
	link, err := parse("--terminal", "--open-url", "ssh://operator@192.0.2.10:2222")
	if err != nil || link == nil || link.Preset.Host != "192.0.2.10" || link.Preset.Port != 2222 || link.Preset.Username != "operator" {
		t.Fatalf("Registered link command failed: %+v %v", link, err)
	}
	if link, err := parse("--terminal", "--open-url=telnet://192.0.2.20"); err != nil || link.Preset.Type != "telnet" {
		t.Fatalf("Inline link value failed: %+v %v", link, err)
	}
	for _, args := range [][]string{
		{"--open-url", "ssh://192.0.2.10"},
		{"--terminal=false", "--open-url", "ssh://192.0.2.10"},
		// What a link with embedded quotes turns into on the Windows command line.
		{"--terminal", "--open-url", "ssh://192.0.2.10", "--data-dir", "C:\\Users\\Public"},
		{"--terminal", "--open-url", "ssh://192.0.2.10", "--no-browser"},
		{"--terminal", "--open-url", "ssh://192.0.2.10", "--port", "8080"},
		{"--terminal", "--open-url", "ssh://192.0.2.10", "--terminal-preset", "e30"},
		{"--terminal", "--open-url", "ssh://192.0.2.10", "extra"},
		{"--terminal", "--open-url", "ssh://192.0.2.10", "--", "--data-dir"},
		{"--terminal", "--open-url", ""},
		{"--terminal", "--open-url", "http://192.0.2.10"},
	} {
		if link, err := parse(args...); err == nil {
			t.Fatalf("%q was accepted: %+v", args, link)
		}
	}
}
