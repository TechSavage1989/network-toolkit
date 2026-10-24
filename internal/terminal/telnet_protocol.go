package terminal

import (
	"errors"
	"fmt"
	"strings"
)

// Telnet command and option values are from RFC 854, 856, 857, 858, 1073 and
// 1091. Each direction of each option has its own RFC 1143 negotiation state.
const (
	tnSE           = 240
	tnNOP          = 241
	tnDM           = 242
	tnBRK          = 243
	tnIP           = 244
	tnAO           = 245
	tnAYT          = 246
	tnEC           = 247
	tnEL           = 248
	tnGA           = 249
	tnSB           = 250
	tnWILL         = 251
	tnWONT         = 252
	tnDO           = 253
	tnDONT         = 254
	tnIAC          = 255
	tnBinary       = 0
	tnEcho         = 1
	tnSGA          = 3
	tnTerminalType = 24
	tnNAWS         = 31
)

type telnetQState uint8

const (
	qNo telnetQState = iota
	qYes
	qWantNo
	qWantNoOpposite
	qWantYes
	qWantYesOpposite
)

// receive and request return 1 for a positive negotiation, -1 for a negative
// negotiation, and 0 when no response is needed. The caller chooses DO/DONT or
// WILL/WONT according to the direction. Unknown options are always refused.
func (q *telnetQState) receive(positive, allow bool) int {
	if positive {
		switch *q {
		case qNo:
			if allow {
				*q = qYes
				return 1
			}
			return -1
		case qYes:
		case qWantNo:
			*q = qNo
		case qWantNoOpposite:
			*q = qYes
		case qWantYes:
			*q = qYes
		case qWantYesOpposite:
			*q = qWantNo
			return -1
		}
	} else {
		switch *q {
		case qNo:
		case qYes:
			*q = qNo
			return -1
		case qWantNo:
			*q = qNo
		case qWantNoOpposite:
			*q = qWantYes
			return 1
		case qWantYes, qWantYesOpposite:
			*q = qNo
		}
	}
	return 0
}

func (q *telnetQState) request(enable bool) int {
	if enable {
		switch *q {
		case qNo:
			*q = qWantYes
			return 1
		case qWantNo:
			*q = qWantNoOpposite
		case qWantYesOpposite:
			*q = qWantYes
		}
	} else {
		switch *q {
		case qYes:
			*q = qWantNo
			return -1
		case qWantNoOpposite:
			*q = qWantNo
		case qWantYes:
			*q = qWantYesOpposite
		}
	}
	return 0
}

type telnetParserState uint8

const (
	tnData telnetParserState = iota
	tnCommand
	tnOption
	tnSubOption
	tnSubData
	tnSubCommand
)

type telnetProtocol struct {
	local, remote      [256]telnetQState
	state              telnetParserState
	command, subOption byte
	sub                []byte
	pendingCR          bool
	cols, rows         int
	terminal           string
}

func newTelnetProtocol(cols, rows int, terminal string) *telnetProtocol {
	if cols < 1 || cols > 65535 {
		cols = 80
	}
	if rows < 1 || rows > 65535 {
		rows = 24
	}
	if terminal == "" || len(terminal) > 64 {
		terminal = "xterm-256color"
	}
	for _, c := range terminal {
		if c < 33 || c > 126 || strings.ContainsRune("\"'\\", c) {
			terminal = "xterm-256color"
			break
		}
	}
	return &telnetProtocol{cols: cols, rows: rows, terminal: terminal}
}

func (p *telnetProtocol) initial() []byte {
	var wire []byte
	for _, option := range []byte{tnBinary, tnSGA, tnTerminalType, tnNAWS} {
		wire = appendNegotiation(wire, option, p.local[option].request(true), true)
	}
	for _, option := range []byte{tnBinary, tnEcho, tnSGA} {
		wire = appendNegotiation(wire, option, p.remote[option].request(true), false)
	}
	return wire
}

func appendNegotiation(wire []byte, option byte, direction int, local bool) []byte {
	if direction == 0 {
		return wire
	}
	command := byte(tnDO)
	if direction < 0 {
		command = tnDONT
	}
	if local {
		command = tnWILL
		if direction < 0 {
			command = tnWONT
		}
	}
	return append(wire, tnIAC, command, option)
}

func (p *telnetProtocol) negotiate(command, option byte) []byte {
	var wire []byte
	if command == tnDO || command == tnDONT {
		allow := option == tnBinary || option == tnSGA || option == tnTerminalType || option == tnNAWS
		was := p.local[option]
		wire = appendNegotiation(wire, option, p.local[option].receive(command == tnDO, allow), true)
		if option == tnNAWS && was != qYes && p.local[option] == qYes {
			wire = append(wire, p.windowSize()...)
		}
	} else {
		allow := option == tnBinary || option == tnEcho || option == tnSGA
		was := p.remote[option]
		wire = appendNegotiation(wire, option, p.remote[option].receive(command == tnWILL, allow), false)
		if option == tnBinary && was != p.remote[option] {
			p.pendingCR = false
		}
	}
	return wire
}

func telnetSubnegotiation(option byte, data []byte) []byte {
	wire := []byte{tnIAC, tnSB, option}
	for _, b := range data {
		wire = append(wire, b)
		if b == tnIAC {
			wire = append(wire, b)
		}
	}
	return append(wire, tnIAC, tnSE)
}

func (p *telnetProtocol) windowSize() []byte {
	return telnetSubnegotiation(tnNAWS, []byte{byte(p.cols >> 8), byte(p.cols), byte(p.rows >> 8), byte(p.rows)})
}

func (p *telnetProtocol) resize(cols, rows int) ([]byte, error) {
	if cols < 1 || cols > 65535 || rows < 1 || rows > 65535 {
		return nil, errors.New("terminal dimensions must be between 1 and 65535")
	}
	if p.cols == cols && p.rows == rows {
		return nil, nil
	}
	p.cols, p.rows = cols, rows
	if p.local[tnNAWS] == qYes {
		return p.windowSize(), nil
	}
	return nil, nil
}

func (p *telnetProtocol) dataByte(output []byte, b byte) []byte {
	if p.remote[tnBinary] != qYes {
		if p.pendingCR && b == 0 {
			p.pendingCR = false
			return output
		}
		p.pendingCR = b == '\r'
	}
	return append(output, b)
}

// feed may receive any fragment, including fragments ending in IAC or in a
// subnegotiation. Protocol bytes never reach the terminal renderer. CR NUL is
// decoded without delaying a visible carriage return; CR LF remains CR LF.
func (p *telnetProtocol) feed(data []byte) (output, reply []byte, err error) {
	for _, b := range data {
		switch p.state {
		case tnData:
			if b == tnIAC {
				p.state = tnCommand
			} else {
				output = p.dataByte(output, b)
			}
		case tnCommand:
			p.state = tnData
			switch b {
			case tnIAC:
				output = p.dataByte(output, b)
			case tnWILL, tnWONT, tnDO, tnDONT:
				p.command, p.state = b, tnOption
			case tnSB:
				p.state = tnSubOption
			case tnAYT:
				reply = append(reply, []byte("\r\n[Network Toolkit is here]\r\n")...)
			case tnEC:
				output = append(output, '\b', ' ', '\b')
			case tnEL:
				output = append(output, []byte("\r\x1b[2K")...)
			}
		case tnOption:
			reply = append(reply, p.negotiate(p.command, b)...)
			p.state = tnData
		case tnSubOption:
			p.subOption, p.sub, p.state = b, p.sub[:0], tnSubData
		case tnSubData:
			if b == tnIAC {
				p.state = tnSubCommand
			} else {
				p.sub = append(p.sub, b)
			}
		case tnSubCommand:
			switch b {
			case tnIAC:
				p.sub = append(p.sub, b)
				p.state = tnSubData
			case tnSE:
				if p.subOption == tnTerminalType && p.local[tnTerminalType] == qYes && len(p.sub) == 1 && p.sub[0] == 1 {
					reply = append(reply, telnetSubnegotiation(tnTerminalType, append([]byte{0}, []byte(p.terminal)...))...)
				}
				p.sub, p.state = p.sub[:0], tnData
			default:
				return output, reply, fmt.Errorf("malformed Telnet subnegotiation: IAC %d", b)
			}
		}
		if len(p.sub) > 4096 {
			return output, reply, errors.New("Telnet subnegotiation exceeds 4096 bytes")
		}
	}
	return output, reply, nil
}

// telnetEncode converts terminal input to the active wire representation. In
// NVT mode a literal CR is CR NUL, a newline is CR LF, and an existing CR LF is
// retained. Binary mode preserves all data bytes. IAC is escaped in both modes.
// ends maps completed wire offsets back to input byte counts for short writes.
func telnetEncode(data []byte, binary bool) (wire []byte, ends []int) {
	wire = make([]byte, 0, len(data)+16)
	ends = make([]int, 0, len(data))
	for i, b := range data {
		if !binary {
			switch b {
			case '\r':
				wire = append(wire, b)
				if i+1 == len(data) || data[i+1] != '\n' && data[i+1] != 0 {
					wire = append(wire, 0)
				}
				ends = append(ends, len(wire))
				continue
			case '\n':
				if i == 0 || data[i-1] != '\r' {
					wire = append(wire, '\r')
				}
			}
		}
		wire = append(wire, b)
		if b == tnIAC {
			wire = append(wire, b)
		}
		ends = append(ends, len(wire))
	}
	return wire, ends
}

func telnetLocalEcho(data []byte) []byte {
	var output []byte
	for i, b := range data {
		switch b {
		case '\r':
			output = append(output, '\r', '\n')
		case '\n':
			if i == 0 || data[i-1] != '\r' {
				output = append(output, '\r', '\n')
			}
		case 8, 127:
			output = append(output, '\b', ' ', '\b')
		default:
			output = append(output, b)
		}
	}
	return output
}
