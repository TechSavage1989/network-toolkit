package terminal

// sshBannerNormalizer is scoped to one SSH authentication exchange. Some
// devices send bare LF in their pre-PTY login banner. xterm correctly preserves
// the current column on LF, so those banners otherwise form a staircase.
//
// Only a missing CR is supplied. Existing CRLF, lone CR, control sequences and
// all other bytes remain unchanged. Remember the last byte across banner
// callbacks so a fragmented CRLF is not doubled. No PTY output uses this helper.
type sshBannerNormalizer struct{ previousCR bool }

func (n *sshBannerNormalizer) normalize(message string) []byte {
	output := make([]byte, 0, len(message))
	for i := 0; i < len(message); i++ {
		b := message[i]
		if b == '\n' && !n.previousCR {
			output = append(output, '\r')
		}
		output = append(output, b)
		n.previousCR = b == '\r'
	}
	return output
}
