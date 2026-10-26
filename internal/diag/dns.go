package diag

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/model"
	"networktoolkit/internal/platform"
)

var dnsTypes = map[string]uint16{"A": 1, "NS": 2, "CNAME": 5, "SOA": 6, "PTR": 12, "MX": 15, "TXT": 16, "AAAA": 28, "SRV": 33, "CAA": 257, "DS": 43, "DNSKEY": 48, "TLSA": 52, "HTTPS": 65}
var dnsNames = map[uint16]string{1: "A", 2: "NS", 5: "CNAME", 6: "SOA", 12: "PTR", 15: "MX", 16: "TXT", 28: "AAAA", 33: "SRV", 41: "OPT", 257: "CAA", 43: "DS", 48: "DNSKEY", 52: "TLSA", 65: "HTTPS"}

type DNSRecord struct {
	Time    string         `json:"time,omitempty"`
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Value   string         `json:"value"`
	TTL     *uint32        `json:"ttl,omitempty"`
	Section string         `json:"section,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}
type DNSQueryResult struct {
	Time       string  `json:"time,omitempty"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	DurationMs float64 `json:"durationMs"`
	Server     string  `json:"server"`
	Error      string  `json:"error,omitempty"`
}

func dns(ctx context.Context, req model.Request, emit model.Emit) (model.Result, error) {
	query := strings.TrimSpace(req.Target)
	if query == "" {
		return model.Result{}, errors.New("enter a DNS name or an IP address for a PTR lookup")
	}
	typ := strings.ToUpper(stringOption(req.Options, "recordType", "A"))
	if _, ok := dnsTypes[typ]; !ok && typ != "ALL" {
		return model.Result{}, errors.New("unsupported DNS record type")
	}
	if net.ParseIP(query) != nil && typ != "PTR" && typ != "ALL" {
		return model.Result{}, errors.New("select PTR for a reverse DNS lookup of an IP address")
	}
	server := stringOption(req.Options, "server", "")
	if binding, selected := platform.SourceBindingFromContext(ctx); selected {
		var err error
		if server == "" {
			server, err = selectedDNSServer(ctx)
		} else {
			server, err = boundServerEndpoint(binding, server)
		}
		if err != nil {
			return model.Result{}, err
		}
	} else if server != "" {
		if _, err := serverEndpoint(server); err != nil {
			return model.Result{}, err
		}
	}
	timeout, err := durationOption(req.Options, "timeoutMs", 3000, 100, 15000)
	if err != nil {
		return model.Result{}, err
	}
	types := []string{typ}
	if typ == "ALL" {
		types = []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT", "SOA", "CAA", "DS", "DNSKEY", "HTTPS"}
		if net.ParseIP(query) != nil {
			types = []string{"PTR"}
		}
	}
	records := make([]DNSRecord, 0)
	queries := make([]DNSQueryResult, 0, len(types))
	start := time.Now()
	serverLabel := server
	if serverLabel == "" {
		serverLabel = "System resolver"
	}
	result := func() model.Result {
		return model.Result{Summary: fmt.Sprintf("%d record(s) from %d query type(s)", len(records), len(queries)), Data: map[string]any{"query": query, "server": serverLabel, "records": records, "queries": queries, "elapsedMs": ms(time.Since(start)), "note": "TTL values come from the Windows DNS client where supported or a direct DNS query. A native fallback may not provide TTL. Cached TTL values decrease over time. ALL queries common record types separately rather than using the DNS ANY query."}}
	}
	for _, recordType := range types {
		if err = ctx.Err(); err != nil {
			return result(), err
		}
		q := query
		if recordType == "PTR" {
			if ip := net.ParseIP(query); ip != nil {
				q = reverseName(ip)
			}
		}
		if _, err = dnsQuestion(q, dnsTypes[recordType], 0); err != nil {
			return result(), err
		}
		queryStart := time.Now()
		qr := DNSQueryResult{Type: recordType, Server: serverLabel}
		var rows []DNSRecord
		var lookupErr error
		useServer := server
		// On Windows, the system DNS client can provide per-record TTLs and
		// respects interface selection, NRPT, VPN routing and resolver fallback.
		// Do not confuse a native IP lookup (which drops TTL) with DnsQuery_W.
		if server == "" {
			if nativeRows, nativeErr := systemDNSWithTTL(ctx, q, recordType, timeout); nativeErr == nil {
				rows, qr.Status, qr.Server = nativeRows, "NOERROR", "Windows system DNS (TTL available)"
				useServer = ""
				lookupErr = nil
			} else {
				lookupErr = nativeErr
			}
		}
		// Prefer the configured resolver's DNS wire response for every type:
		// Go's native lookup APIs discard per-record TTL and DNS response codes.
		// A native lookup is retained as a fallback for networks blocking direct
		// DNS, but its response does not claim to contain TTL information.
		if server == "" && qr.Status != "NOERROR" {
			if configured, configuredErr := configuredDNSServer(ctx); configuredErr == nil {
				useServer = configured
				qr.Server = configured + " (configured resolver)"
			}
		}
		if qr.Status != "NOERROR" {
			lookupErr = nil // The platform TTL API is optional, not a terminal failure.
			if useServer != "" {
				var code string
				rows, code, lookupErr = wireLookup(ctx, q, dnsTypes[recordType], useServer, timeout)
				qr.Status = code
				if lookupErr != nil && server == "" && recordType != "SOA" && recordType != "CAA" && recordType != "DS" && recordType != "DNSKEY" && recordType != "TLSA" && recordType != "HTTPS" {
					if nativeRows, nativeErr := nativeLookup(ctx, query, recordType, timeout); nativeErr == nil {
						rows, lookupErr, qr.Status = nativeRows, nil, ""
						qr.Server = "System resolver (native fallback, TTL unavailable)"
					}
				}
			} else {
				rows, lookupErr = nativeLookup(ctx, query, recordType, timeout)
			}
		}
		qr.DurationMs = ms(time.Since(queryStart))
		qr.Time = measurementTime()
		if lookupErr != nil {
			qr.Error = lookupErr.Error()
			qr.Status = "error"
			var de *net.DNSError
			if errors.As(lookupErr, &de) && de.IsNotFound {
				qr.Status = "no_records"
			}
		}
		if qr.Status == "" || qr.Status == "NOERROR" {
			answers := 0
			for _, row := range rows {
				if row.Section == "" || row.Section == "answer" {
					answers++
				}
			}
			if answers > 0 {
				qr.Status = "success"
			} else {
				qr.Status = "no_records"
			}
		}
		for i := range rows {
			if rows[i].Time == "" {
				rows[i].Time = qr.Time
			}
			if rows[i].Details == nil {
				rows[i].Details = dnsRecordDetails(rows[i])
			}
		}
		queries = append(queries, qr)
		records = append(records, rows...)
		level := "success"
		if qr.Status != "success" {
			level = "warning"
		}
		event(emit, "dns", level, fmt.Sprintf("%s: %s (%d records, %.1f ms)", recordType, qr.Status, len(rows), qr.DurationMs), map[string]any{"query": qr, "records": rows})
	}
	if len(queries) == 1 && queries[0].Status == "error" {
		return result(), errors.New(queries[0].Error)
	}
	return result(), ctx.Err()
}

func nativeLookup(parent context.Context, query, typ string, timeout time.Duration) ([]DNSRecord, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	resolver := net.DefaultResolver
	if _, selected := platform.SourceBindingFromContext(ctx); selected {
		var err error
		resolver, err = boundResolver(ctx)
		if err != nil {
			return nil, err
		}
	}
	rows := make([]DNSRecord, 0)
	add := func(value string) { rows = append(rows, DNSRecord{Name: query, Type: typ, Value: value}) }
	switch typ {
	case "A", "AAAA":
		network := "ip4"
		if typ == "AAAA" {
			network = "ip6"
		}
		ips, err := resolver.LookupIP(ctx, network, query)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			add(ip.String())
		}
	case "CNAME":
		name, err := resolver.LookupCNAME(ctx, query)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(strings.TrimSuffix(query, "."), strings.TrimSuffix(name, ".")) {
			add(name)
		}
	case "MX":
		mxs, err := resolver.LookupMX(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, mx := range mxs {
			add(fmt.Sprintf("%d %s", mx.Pref, mx.Host))
		}
	case "NS":
		nss, err := resolver.LookupNS(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, ns := range nss {
			add(ns.Host)
		}
	case "TXT":
		txts, err := resolver.LookupTXT(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, txt := range txts {
			add(txt)
		}
	case "PTR":
		names, err := resolver.LookupAddr(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			add(name)
		}
	case "SRV":
		_, srvs, err := resolver.LookupSRV(ctx, "", "", query)
		if err != nil {
			return nil, err
		}
		for _, srv := range srvs {
			add(fmt.Sprintf("%d %d %d %s", srv.Priority, srv.Weight, srv.Port, srv.Target))
		}
	default:
		return nil, errors.New("record type requires a direct DNS query")
	}
	return rows, nil
}

func configuredDNSServer(ctx context.Context) (string, error) {
	if _, selected := platform.SourceBindingFromContext(ctx); selected {
		return selectedDNSServer(ctx)
	}
	system, err := platform.System(ctx)
	if err != nil {
		return "", fmt.Errorf("read configured DNS servers: %w", err)
	}
	var values []string
	switch list := system["dnsServers"].(type) {
	case []string:
		values = list
	case []any:
		for _, v := range list {
			if s, ok := v.(string); ok {
				values = append(values, s)
			}
		}
	}
	for _, server := range values {
		addr, err := netip.ParseAddr(strings.TrimSpace(server))
		if err == nil && (!addr.IsValid() || addr.IsUnspecified() || addr.IsMulticast()) {
			continue
		}
		if _, err := serverEndpoint(server); err == nil {
			return server, nil
		}
	}
	return "", errors.New("no configured DNS server was available for this record type; specify a DNS server")
}

func reverseName(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa.", v4[3], v4[2], v4[1], v4[0])
	}
	v6 := ip.To16()
	var b strings.Builder
	for i := len(v6) - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "%x.%x.", v6[i]&15, v6[i]>>4)
	}
	b.WriteString("ip6.arpa.")
	return b.String()
}

func serverEndpoint(server string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", errors.New("DNS server is empty")
	}
	if a, err := netip.ParseAddr(strings.Trim(server, "[]")); err == nil {
		return net.JoinHostPort(a.String(), "53"), nil
	}
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		if strings.Contains(server, ":") {
			return "", errors.New("DNS server must be an IP or hostname, optionally with :port; use [IPv6]:port")
		}
		host, port = server, "53"
	}
	if _, err = validateHost(host); err != nil {
		return "", fmt.Errorf("DNS server: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("DNS server port must be between 1 and 65535")
	}
	return net.JoinHostPort(host, port), nil
}

func dnsQuestion(name string, typ, id uint16) ([]byte, error) {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == "" || len(name) > 253 {
		return nil, errors.New("DNS name must contain 1 to 253 characters")
	}
	msg := make([]byte, 12)
	binary.BigEndian.PutUint16(msg, id)
	binary.BigEndian.PutUint16(msg[2:], 0x0100)
	binary.BigEndian.PutUint16(msg[4:], 1)
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, errors.New("DNS label must contain 1 to 63 characters")
		}
		for _, c := range []byte(label) {
			if c < 33 || c > 126 || c == '/' || c == '\\' {
				return nil, errors.New("DNS name contains an invalid character; use ASCII or punycode")
			}
		}
		msg = append(msg, byte(len(label)))
		msg = append(msg, label...)
	}
	msg = append(msg, 0, byte(typ>>8), byte(typ), 0, 1)
	return msg, nil
}

func wireLookup(parent context.Context, name string, typ uint16, server string, timeout time.Duration) ([]DNSRecord, string, error) {
	endpoint, err := serverEndpoint(server)
	if err != nil {
		return nil, "", err
	}
	var idBytes [2]byte
	if _, err = rand.Read(idBytes[:]); err != nil {
		return nil, "", err
	}
	id := binary.BigEndian.Uint16(idBytes[:])
	question, err := dnsQuestion(name, typ, id)
	if err != nil {
		return nil, "", err
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	response, err := dnsExchange(ctx, "udp", endpoint, question)
	if err != nil {
		return nil, "", fmt.Errorf("DNS query to %s: %w", endpoint, err)
	}
	if len(response) < 12 || binary.BigEndian.Uint16(response) != id {
		return nil, "", errors.New("DNS response ID did not match the query")
	}
	if binary.BigEndian.Uint16(response[2:])&0x0200 != 0 {
		response, err = dnsExchange(ctx, "tcp", endpoint, question)
		if err != nil {
			return nil, "", fmt.Errorf("DNS TCP retry to %s: %w", endpoint, err)
		}
	}
	return parseDNS(response, id, name, typ)
}

func dnsExchange(ctx context.Context, network, endpoint string, question []byte) ([]byte, error) {
	conn, err := platform.DialContext(ctx, network, endpoint, 0)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	if network == "tcp" {
		packet := make([]byte, 2+len(question))
		binary.BigEndian.PutUint16(packet, uint16(len(question)))
		copy(packet[2:], question)
		if _, err = conn.Write(packet); err != nil {
			return nil, err
		}
		var size [2]byte
		if _, err = io.ReadFull(conn, size[:]); err != nil {
			return nil, err
		}
		n := int(binary.BigEndian.Uint16(size[:]))
		if n < 12 {
			return nil, errors.New("DNS TCP response is too short")
		}
		data := make([]byte, n)
		_, err = io.ReadFull(conn, data)
		return data, err
	}
	if _, err = conn.Write(question); err != nil {
		return nil, err
	}
	data := make([]byte, 65535)
	n, err := conn.Read(data)
	return data[:n], err
}

func dnsName(msg []byte, offset int) (string, int, error) {
	labels := make([]string, 0)
	visited := map[int]bool{}
	next := -1
	length := 0
	for steps := 0; steps < 128; steps++ {
		if offset < 0 || offset >= len(msg) {
			return "", 0, errors.New("DNS name extends beyond the response")
		}
		if visited[offset] {
			return "", 0, errors.New("DNS name compression loop")
		}
		visited[offset] = true
		n := int(msg[offset])
		offset++
		if n == 0 {
			if next < 0 {
				next = offset
			}
			return strings.Join(labels, ".") + ".", next, nil
		}
		if n&0xc0 == 0xc0 {
			if offset >= len(msg) {
				return "", 0, errors.New("truncated DNS compression pointer")
			}
			pointer := (n&0x3f)<<8 | int(msg[offset])
			offset++
			if next < 0 {
				next = offset
			}
			offset = pointer
			continue
		}
		if n&0xc0 != 0 || n > 63 || offset+n > len(msg) {
			return "", 0, errors.New("invalid DNS label length")
		}
		length += n + 1
		if length > 255 {
			return "", 0, errors.New("decoded DNS name is too long")
		}
		var label strings.Builder
		for _, c := range msg[offset : offset+n] {
			if c < 33 || c > 126 || c == '.' || c == '\\' {
				fmt.Fprintf(&label, "\\%03d", c)
			} else {
				label.WriteByte(c)
			}
		}
		labels = append(labels, label.String())
		offset += n
	}
	return "", 0, errors.New("DNS name exceeds the compression limit")
}

func parseDNS(msg []byte, id uint16, name string, typ uint16) ([]DNSRecord, string, error) {
	if len(msg) < 12 {
		return nil, "", errors.New("DNS response is shorter than its header")
	}
	if binary.BigEndian.Uint16(msg) != id {
		return nil, "", errors.New("DNS response ID mismatch")
	}
	flags := binary.BigEndian.Uint16(msg[2:])
	if flags&0x8000 == 0 {
		return nil, "", errors.New("DNS packet is not a response")
	}
	if flags&0x7800 != 0 {
		return nil, "", errors.New("DNS response has an unexpected opcode")
	}
	if flags&0x0200 != 0 {
		return nil, "", errors.New("DNS response is still truncated after TCP retry")
	}
	rcode := int(flags & 15)
	codes := []string{"NOERROR", "FORMERR", "SERVFAIL", "NXDOMAIN", "NOTIMP", "REFUSED"}
	code := fmt.Sprintf("RCODE_%d", rcode)
	if rcode < len(codes) {
		code = codes[rcode]
	}
	qd := int(binary.BigEndian.Uint16(msg[4:]))
	if qd != 1 {
		return nil, code, errors.New("DNS response must echo exactly one question")
	}
	qname, offset, err := dnsName(msg, 12)
	if err != nil {
		return nil, code, err
	}
	if offset+4 > len(msg) {
		return nil, code, errors.New("truncated DNS question")
	}
	if !strings.EqualFold(strings.TrimSuffix(qname, "."), strings.TrimSuffix(name, ".")) || binary.BigEndian.Uint16(msg[offset:]) != typ || binary.BigEndian.Uint16(msg[offset+2:]) != 1 {
		return nil, code, errors.New("DNS response question does not match the request")
	}
	offset += 4
	counts := []int{int(binary.BigEndian.Uint16(msg[6:])), int(binary.BigEndian.Uint16(msg[8:])), int(binary.BigEndian.Uint16(msg[10:]))}
	if counts[0]+counts[1]+counts[2] > 4096 {
		return nil, code, errors.New("DNS response contains too many records")
	}
	records := make([]DNSRecord, 0, counts[0]+counts[1]+counts[2])
	for section, count := range counts {
		for i := 0; i < count; i++ {
			owner, next, e := dnsName(msg, offset)
			if e != nil {
				return nil, code, e
			}
			offset = next
			if offset+10 > len(msg) {
				return nil, code, errors.New("truncated DNS resource record")
			}
			rtype := binary.BigEndian.Uint16(msg[offset:])
			class := binary.BigEndian.Uint16(msg[offset+2:])
			ttl := binary.BigEndian.Uint32(msg[offset+4:])
			size := int(binary.BigEndian.Uint16(msg[offset+8:]))
			offset += 10
			end := offset + size
			if end > len(msg) {
				return nil, code, errors.New("DNS record data exceeds the response")
			}
			if class == 1 && rtype != 41 {
				value, e := dnsValue(msg, offset, end, rtype)
				if e != nil {
					return nil, code, e
				}
				kind := dnsNames[rtype]
				if kind == "" {
					kind = fmt.Sprintf("TYPE%d", rtype)
				}
				records = append(records, DNSRecord{Name: owner, Type: kind, Value: value, TTL: &ttl, Section: []string{"answer", "authority", "additional"}[section]})
			}
			offset = end
		}
	}
	return records, code, nil
}

func dnsValue(msg []byte, start, end int, typ uint16) (string, error) {
	r := msg[start:end]
	readName := func(offset int) (string, int, error) {
		if offset >= end {
			return "", 0, errors.New("DNS name missing from record")
		}
		name, next, err := dnsName(msg, offset)
		if err == nil && next > end {
			err = errors.New("DNS name extends beyond its record")
		}
		return name, next, err
	}
	switch typ {
	case 1:
		if len(r) != 4 {
			return "", errors.New("invalid A record length")
		}
		return net.IP(r).String(), nil
	case 28:
		if len(r) != 16 {
			return "", errors.New("invalid AAAA record length")
		}
		return net.IP(r).String(), nil
	case 2, 5, 12:
		name, _, err := readName(start)
		return name, err
	case 15:
		if len(r) < 3 {
			return "", errors.New("invalid MX record length")
		}
		name, _, err := readName(start + 2)
		return fmt.Sprintf("%d %s", binary.BigEndian.Uint16(r), name), err
	case 16:
		var values []string
		for offset := start; offset < end; {
			n := int(msg[offset])
			offset++
			if offset+n > end {
				return "", errors.New("truncated TXT record")
			}
			values = append(values, string(msg[offset:offset+n]))
			offset += n
		}
		return strings.Join(values, ""), nil
	case 257: // CAA: flags, tag and value
		if len(r) < 2 || 2+int(r[1]) > len(r) {
			return "", errors.New("invalid CAA record")
		}
		return fmt.Sprintf("%d %s %q", r[0], string(r[2:2+int(r[1])]), string(r[2+int(r[1]):])), nil
	case 43: // DS: key tag, algorithm, digest type, digest
		if len(r) < 4 {
			return "", errors.New("invalid DS record")
		}
		return fmt.Sprintf("%d %d %d %s", binary.BigEndian.Uint16(r), r[2], r[3], strings.ToUpper(hex.EncodeToString(r[4:]))), nil
	case 48: // DNSKEY: flags, protocol, algorithm, base64 public key
		if len(r) < 4 {
			return "", errors.New("invalid DNSKEY record")
		}
		return fmt.Sprintf("flags=%d protocol=%d algorithm=%d key=%s", binary.BigEndian.Uint16(r), r[2], r[3], hex.EncodeToString(r[4:])), nil
	case 33:
		if len(r) < 7 {
			return "", errors.New("invalid SRV record length")
		}
		name, _, err := readName(start + 6)
		return fmt.Sprintf("%d %d %d %s", binary.BigEndian.Uint16(r), binary.BigEndian.Uint16(r[2:]), binary.BigEndian.Uint16(r[4:]), name), err
	case 6:
		primary, next, err := readName(start)
		if err != nil {
			return "", err
		}
		mailbox, next, err := readName(next)
		if err != nil {
			return "", err
		}
		if next+20 != end {
			return "", errors.New("invalid SOA record length")
		}
		return fmt.Sprintf("%s %s %d %d %d %d %d", primary, mailbox, binary.BigEndian.Uint32(msg[next:]), binary.BigEndian.Uint32(msg[next+4:]), binary.BigEndian.Uint32(msg[next+8:]), binary.BigEndian.Uint32(msg[next+12:]), binary.BigEndian.Uint32(msg[next+16:])), nil
	default:
		return hex.EncodeToString(r), nil
	}
}

// DNSChecker-style protocol fields are returned independently from the
// summary text so they are available in result details and exports.
func dnsRecordDetails(record DNSRecord) map[string]any {
	values := strings.Fields(record.Value)
	result := map[string]any{}
	switch record.Type {
	case "MX":
		if len(values) >= 2 {
			result["preference"] = values[0]
			result["mailExchanger"] = strings.Join(values[1:], " ")
		}
	case "SOA":
		if len(values) >= 7 {
			for i, name := range []string{"primaryNameServer", "responsibleMailbox", "serial", "refreshSeconds", "retrySeconds", "expireSeconds", "minimumTTLSeconds"} {
				result[name] = values[i]
			}
		}
	case "SRV":
		if len(values) >= 4 {
			for i, name := range []string{"priority", "weight", "port"} {
				result[name] = values[i]
			}
			result["target"] = strings.Join(values[3:], " ")
		}
	case "CAA":
		if len(values) >= 3 {
			result["flags"] = values[0]
			result["tag"] = values[1]
			result["issuer"] = strings.Join(values[2:], " ")
		}
	case "TXT":
		if strings.HasPrefix(strings.ToLower(record.Value), "v=spf1") {
			result["purpose"] = "SPF"
		}
		if strings.HasPrefix(strings.ToLower(record.Value), "v=dmarc1") {
			result["purpose"] = "DMARC"
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
