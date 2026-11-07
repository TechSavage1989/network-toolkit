package capture

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Everyday wording for the troubleshooting view. The technical wording stays
// in Observation and Technical so nothing is lost for experienced users.

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

func dnsCodeName(code uint16) string {
	switch code {
	case 1:
		return "FORMERR"
	case 2:
		return "SERVFAIL"
	case 3:
		return "NXDOMAIN"
	case 4:
		return "NOTIMP"
	case 5:
		return "REFUSED"
	}
	return "error"
}

func dnsErrorHeadline(code uint16, name string) string {
	switch code {
	case 3:
		return "The name " + name + " does not exist"
	case 2:
		return "The DNS server could not look up " + name
	case 5:
		return "The DNS server refused to look up " + name
	}
	return "The DNS server returned an error for " + name
}

func icmpUnreachableReason(v6 bool, code byte) string {
	if v6 {
		switch code {
		case 0:
			return "there is no route to it"
		case 1:
			return "it is blocked by a firewall or policy"
		case 3:
			return "the device did not respond"
		case 4:
			return "nothing is listening on that port"
		}
		return "delivery failed"
	}
	switch code {
	case 0, 6:
		return "there is no route to that network"
	case 1, 7:
		return "the device did not respond"
	case 2:
		return "the protocol is not supported there"
	case 3:
		return "nothing is listening on that port"
	case 9, 10, 13:
		return "it is blocked by a firewall or policy"
	}
	return "delivery failed"
}

func httpStatusText(status int) string {
	text := map[int]string{
		400: "bad request", 401: "sign-in required", 403: "access denied", 404: "page not found",
		405: "method not allowed", 407: "proxy sign-in required", 408: "request timed out",
		429: "too many requests", 500: "server error", 501: "not supported", 502: "bad gateway",
		503: "service unavailable", 504: "gateway timed out",
	}[status]
	if text == "" {
		if status >= 500 {
			text = "server error"
		} else {
			text = "request error"
		}
	}
	return fmt.Sprintf("%s (%d)", text, status)
}

// A capture with fewer packets or seconds than this is too small to judge.
const (
	verdictMinimumPackets = 10
	verdictMinimumSeconds = 3
)

func troubleVerdict(r Troubleshooting) TroubleshootVerdict {
	attention, notes := 0, 0
	for _, f := range r.Findings {
		if f.Severity == "attention" {
			attention++
		} else {
			notes++
		}
	}
	small := r.PacketCount < verdictMinimumPackets || r.Duration < verdictMinimumSeconds
	switch {
	case attention > 0:
		headline := "1 problem found"
		if attention > 1 {
			headline = fmt.Sprintf("%d problems found", attention)
		}
		detail := "Start with the first one below. Each explains what happened, what it usually means and what to check next."
		if notes > 0 {
			detail += fmt.Sprintf(" %d more %s worth knowing follow.", notes, plural(notes, "thing is", "things are"))
		}
		return TroubleshootVerdict{State: "problems", Headline: headline, Detail: detail}
	case small:
		return TroubleshootVerdict{State: "little", Headline: "Very little traffic was recorded", Detail: fmt.Sprintf("%d %s over %.1f seconds is too little to judge. Start the capture first, then try the failing website, app or device while it records, and stop afterwards.", r.PacketCount, plural(r.PacketCount, "packet", "packets"), r.Duration)}
	case notes > 0:
		return TroubleshootVerdict{State: "notes", Headline: "No clear problems found", Detail: fmt.Sprintf("%d %s worth knowing %s below. If the problem happened during this recording, the cause may be in the application or outside what this computer could see.", notes, plural(notes, "thing", "things"), plural(notes, "is", "are"))}
	}
	return TroubleshootVerdict{State: "clear", Headline: "No problems found in this capture", Detail: "Name lookups and connections seen here looked normal. A recording cannot prove everything is healthy: check that the problem happened while it was recording."}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// cleanHost normalizes a host name or address typed by the user.
func cleanHost(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
}
