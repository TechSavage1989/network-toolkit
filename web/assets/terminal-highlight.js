/* Network Toolkit terminal format highlighting. No runtime dependencies.
 * Reads xterm's parsed buffer and adds semantic foreground colors and
 * decoration borders. Default or neutral text with a console-colored background,
 * no inverse attribute and no hidden attribute receives a foreground color.
 * It never writes terminal data, rewrites ANSI attributes, observes keyboard
 * input, creates links, or persists/caches terminal text.
 *
 * Public APIs: https://xtermjs.org/docs/api/terminal/classes/terminal/
 * https://xtermjs.org/docs/api/terminal/interfaces/idecorationoptions/
 * Format references: RFC 3986 section 3.2.2; RFC 4291 sections 2.2/2.3;
 * RFC 4007 section 11; RFC 5424 section 6.2.1; RFC 9110 section 15.
 * Interface names and plain-language status labels are conservative display
 * conventions, not universal standards or health/security conclusions. The
 * palette is an application choice; ANSI/SGR and the format standards do not
 * prescribe these semantic colors. Optional border patterns add a second cue.
 */
(function (global) {
  'use strict';

  const LIMITS = Object.freeze({ text: 4096, token: 128, matches: 256,
    decorations: 8192, rows: 300, columns: 1000, wrapRows: 64, frameRows: 40, frameMs: 6, delay: 40 });
  const LABELS = Object.freeze({ address: 'IP / CIDR', mac: 'MAC address',
    interface: 'Interface label', command: 'Command / config verb', field: 'Setting / field name',
    protocol: 'Protocol / service', value: 'Mode / named value', number: 'Number / unit', comment: 'Comment / description',
    warning: 'Warning / pending', error: 'Error / down', success: 'Up / success' });
  const MARKS = Object.freeze({ address: ['solid', 2], mac: ['dotted', 2],
    interface: ['dashed', 1], command: ['solid', 1], field: ['dotted', 1], protocol: ['dashed', 1],
    value: ['solid', 1], number: ['dotted', 1], comment: ['dotted', 1],
    warning: ['dashed', 2], error: ['double', 3], success: ['solid', 2] });
  const COLORS = Object.freeze({
    dark: Object.freeze({ address: '#69c7ff', mac: '#7ed4d8', interface: '#c6a6ff', command: '#f0a3d0', field: '#a6b8f8', protocol: '#e9bd81', value: '#d0cb93', number: '#bddf86', comment: '#9ba7b8', warning: '#f4c365', error: '#ff9b9b', success: '#91d8ae' }),
    light: Object.freeze({ address: '#165aa0', mac: '#007a79', interface: '#7748a9', command: '#a02678', field: '#3f55ab', protocol: '#99501a', value: '#626300', number: '#386712', comment: '#596579', warning: '#8b5b00', error: '#a82033', success: '#177344' })
  });

  // Syntax roles are based on the published FortiOS, IOS/IOS XE, AOS-CX,
  // AOS-S, Junos, PAN-OS and SonicOS configuration forms. Command position
  // distinguishes settings from operational prose; colors are app choices.
  const COMMANDS = new Set(('config configure conf edit set unset append select unselect next end exit quit show sh get diagnose diag execute exec commit rollback delete remove purge move rename copy replace insert activate deactivate annotate wildcard display save write no default').split(' '));
  const FIELDS = new Set(('system global vdom interface interfaces ethernet loopback lag vlan vlans trunk tagged untagged switchport mode access native allowed address addresses ip ipv4 ipv6 netmask gateway subnet network networks route routes router routing routing-options routing-instances protocols protocol family inet inet6 unit description comment comments remark remarks hostname host-name name alias status type allowaccess management mtu speed duplex auto autonegotiation auto-negotiate flow-control vrf vrf-forwarding zone zones firewall policy policies security security-policy security-mode srcintf dstintf srcaddr dstaddr src-zone dst-zone source destination source-address destination-address source-zone destination-zone service services action schedule nat ippool pool poolname logtraffic log-setting log-profile members member group groups port ports port-range src-port dst-port destination-port source-port metric distance priority weight preference nexthop next-hop enable disable shutdown spanning-tree lacp lldp cdp authentication authorization accounting access-list access-group address-object address-group service-object service-group ip-assignment portshield layer2 layer3 link-state link-duplex virtual-router network-object object object-group neighbor peer remote-as local-as autonomous-system as-path route-map prefix-list community advertise redistribute import export import-policy export-policy filter filters term terms from to then vlan-id vlan-trunk vlan-access routing-interface switch-options ethernet-switching encapsulation flexible-vlan-tagging vlan-tagging voice qos rate-limit bandwidth poe vsf vsx stacking stack member-id chassis serial-number location option options dns-server domain-name ntp-server server server-name server-ip username user password secret private-key certificate inspection-profile ssl-ssh-profile utm-status webfilter-profile av-profile ips-sensor application-list internet-service tunnel-interface tunnel-source tunnel-destination aggregate sub-interface parent-interface virtual-wire-pair wildcard-vlan vlan-filter acl rule rules sequence seq ipsec phase1-interface phase2-interface ike-version proposal dhgrp psksecret remote-gw auto-negotiate monitor interval timeout keepalive retries version firmware release build revision enabled disabled state operstate carrier admin-state admin-status link protocol-state table mask ipmask permitted up down static dhcp pppoe physical tunnel aggregate redundant software-switch hard-switch virtual-switch l2bridge l3 ip-forwarding').split(' '));
  const PROTOCOLS = new Set(('ip ipv4 ipv6 tcp udp icmp icmpv6 icmp6 sctp arp rarp ndp ospf ospfv3 ospf6 bgp rip ripng isis is-is stp rstp mstp pvst rapid-pvst lacp lldp cdp vrrp hsrp dhcp dhcpv6 dns ntp sntp snmp snmpv3 syslog ssh ssh2 telnet http https ftp ftps tftp sftp scp smtp smtps imap imaps pop3 pop3s ldap ldaps radius radius-acct tacacs tacacs+ dot1x 802.1x eap peap tls ssl ike ikev1 ikev2 ipsec esp ah gre gretap vxlan geneve mpls ppp pppoe pppoa pptp l2tp bfd pim igmp mld ripng fgfm fabric ftm ping probe-response netconf restconf'.split(' ')));
  const VALUES = new Set(('static dynamic manual auto automatic enable disable enabled disabled yes no true false on off physical aggregate redundant tunnel vlan vdom-link loopback virtual-switch hard-switch soft-switch software-switch switch access trunk tagged untagged native-untagged routed transparent wire-mode tap-mode l2bridge l3 full half full-duplex half-duplex up down connected disconnected active inactive accept allow permit deny reject drop all any none always default unicast multicast broadcast passthrough proxy flow flow-based proxy-based server client root lan wan dmz mgmt internal external trusted untrusted local remote standalone primary secondary master backup slave passive').split(' '));

  function themeName(theme) { return theme === 'light' ? 'light' : 'dark'; }
  function customColors(value) {
    const result = {};
    for (const kind of Object.keys(LABELS)) if (typeof value?.[kind] === 'string' && /^#[0-9a-f]{6}$/i.test(value[kind])) result[kind] = value[kind].toLowerCase();
    return result;
  }
  function palette(theme, value) { return { ...COLORS[themeName(theme)], ...customColors(value) }; }
  function colorNumber(value) {
    if (typeof value !== 'string') return null;
    if (/^#[0-9a-f]{6}$/i.test(value)) return parseInt(value.slice(1), 16);
    if (/^#[0-9a-f]{3}$/i.test(value)) return parseInt(value.slice(1).replace(/./g, digit => digit + digit), 16);
    return null;
  }
  function foregroundPolicy(terminalTheme) {
    const normal = colorNumber(terminalTheme.foreground), neutralSlots = new Set();
    const neutral = rgb => {
      if (rgb === null) return false;
      if (normal !== null && rgb === normal) return true;
      const red = rgb >>> 16 & 255, green = rgb >>> 8 & 255, blue = rgb & 255;
      return Math.max(red, green, blue) - Math.min(red, green, blue) <= 32;
    };
    // White/bright-white and black/bright-black are normal text roles in
    // many device pagers. Honor a deliberately chromatic custom palette.
    for (const [index, key] of [[0, 'black'], [7, 'white'], [8, 'brightBlack'], [15, 'brightWhite']]) {
      const rgb = colorNumber(terminalTheme[key]);
      if (rgb === null || neutral(rgb)) neutralSlots.add(index);
    }
    return cell => {
      if (typeof cell.isFgDefault !== 'function') return false;
      if (cell.isFgDefault()) return true;
      if (typeof cell.getFgColor !== 'function') return false;
      const value = cell.getFgColor();
      if (typeof cell.isFgRGB === 'function' && cell.isFgRGB()) return neutral(value);
      if (typeof cell.isFgPalette !== 'function' || !cell.isFgPalette()) return false;
      if (value < 16) return neutralSlots.has(value);
      if (value >= 232 && value <= 255) return true;
      if (value < 16 || value > 231) return false;
      const cube = value - 16;
      return Math.floor(cube / 36) === Math.floor(cube / 6) % 6 && Math.floor(cube / 6) % 6 === cube % 6;
    };
  }
  function backgroundPolicy(terminalTheme) {
    const normal = colorNumber(terminalTheme.background);
    const slots = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
      'brightBlack', 'brightRed', 'brightGreen', 'brightYellow', 'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite'];
    const resolved = new Map(), levels = [0, 95, 135, 175, 215, 255];
    return cell => {
      if (typeof cell.isBgDefault !== 'function') return false;
      if (cell.isBgDefault()) return true;
      if (normal === null || typeof cell.getBgColor !== 'function') return false;
      const value = cell.getBgColor();
      let rgb = null;
      if (typeof cell.isBgRGB === 'function' && cell.isBgRGB()) rgb = value;
      else if (typeof cell.isBgPalette === 'function' && cell.isBgPalette()) {
        if (!resolved.has(value)) {
          let color = null;
          if (value < 16) color = colorNumber(terminalTheme[slots[value]]);
          else if (value <= 231) {
            const cube = value - 16;
            color = levels[Math.floor(cube / 36)] << 16 | levels[Math.floor(cube / 6) % 6] << 8 | levels[cube % 6];
          } else if (value <= 255) color = (8 + 10 * (value - 232)) * 0x010101;
          resolved.set(value, color);
        }
        rgb = resolved.get(value);
      }
      if (rgb === null) return false;
      return Math.abs((rgb >>> 16 & 255) - (normal >>> 16 & 255)) <= 32 &&
        Math.abs((rgb >>> 8 & 255) - (normal >>> 8 & 255)) <= 32 && Math.abs((rgb & 255) - (normal & 255)) <= 32;
    };
  }
  function legend(theme, options = {}) {
    const colors = palette(theme, options.colors);
    return Object.keys(LABELS).map(kind => ({ kind, label: LABELS[kind], color: colors[kind],
      borderStyle: options.underlines === false ? 'none' : MARKS[kind][0], borderWidth: options.underlines === false ? 0 : MARKS[kind][1] }));
  }
  function ipv4(value) {
    const parts = value.split('.');
    return parts.length === 4 && parts.every(part => /^(?:0|[1-9][0-9]{0,2})$/.test(part) && Number(part) <= 255);
  }
  function ipv6(value) {
    let address = value;
    const zone = address.indexOf('%');
    if (zone !== -1) {
      if (!/^[A-Za-z0-9_.~-]{1,32}$/.test(address.slice(zone + 1))) return false;
      address = address.slice(0, zone);
    }
    if (!address.includes(':') || address.includes(':::')) return false;
    if (address.includes('.')) {
      const end = address.lastIndexOf(':');
      if (!ipv4(address.slice(end + 1))) return false;
      address = address.slice(0, end + 1) + '0:0';
    }
    if (!/^[0-9a-f:]+$/i.test(address)) return false;
    const halves = address.split('::');
    if (halves.length > 2) return false;
    const groups = [];
    for (const half of halves) {
      if (half) groups.push(...half.split(':'));
    }
    if (groups.some(group => !/^[0-9a-f]{1,4}$/i.test(group))) return false;
    return halves.length === 2 ? groups.length < 8 : groups.length === 8;
  }
  function addressKind(value) {
    const pieces = value.split('/');
    if (pieces.length > 2) return false;
    const version = ipv4(pieces[0]) ? 4 : ipv6(pieces[0]) ? 6 : 0;
    if (!version) return false;
    if (pieces.length === 2 && (!/^(?:0|[1-9][0-9]{0,2})$/.test(pieces[1]) || Number(pieces[1]) > (version === 4 ? 32 : 128))) return false;
    return true;
  }
  function mac(value) {
    return /^(?:[0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(value) ||
      /^(?:[0-9a-f]{2}-){5}[0-9a-f]{2}$/i.test(value) ||
      /^(?:[0-9a-f]{4}\.){2}[0-9a-f]{4}$/i.test(value);
  }
  function interfaceLabel(value, contextual = false) {
    if (/^(?:(?:en|wl|ww)(?:o\d+(?:n\d+)?|s\d+(?:f\d+)?(?:d\d+)?|p\d+s\d+(?:f\d+)?(?:d\d+)?|x[0-9a-f]{12})|(?:eth|wlan|wwan|tun|tap|wg|utun|bond|br|virbr|docker|vnet|vmnet|vxlan)\d+|veth[0-9a-f]{4,16})(?:\.\d{1,4})?$/i.test(value)) return true;
    if (/^(?:GigabitEthernet|TenGigabitEthernet|TwentyFiveGigE|FortyGigabitEthernet|HundredGigE|FastEthernet|Ethernet|Eth|Gi|Te|Fa|Fo|Hu)\d+(?:\/\d+){1,3}(?:\.\d{1,4})?$/i.test(value)) return true;
    if (/^(?:(?:ge|xe|et|fe|at|gr|ip|lt|mt|ms)-\d+\/\d+\/\d+(?::\d+)?(?:\.\d+)?|(?:ae|reth|irb|lo|st|em|fxp|vme)(?:\d+)(?:\.\d+)?|(?:Vlan|Loopback|Port-channel|Bundle-Ether|Tunnel|port|lag|trk|ether|bridge)\d+(?:\.\d+)?|X\d+(?::V\d+)?|Port[A-Z](?:\.\d+)?)$/i.test(value)) return true;
    return contextual && /^(?:\d{1,2}\/\d{1,2}\/\d{1,3}(?:[.:]\d+)?|[A-P]\d{1,2}(?:-[A-P]?\d{1,2})?(?:-Trk\d+)?|wan\d*|lan\d*|internal\d*|dmz|mgmt|fortilink|(?:ssl|naf|l2)\.root)$/i.test(value);
  }
  function syntaxContext(text) {
    const prompt = /^\s*(?:[A-Za-z0-9_.:@/~()\[\]-]+(?:\s+\([^()]{1,64}\))?\s*[#>$%]|<[^<>]{1,120}>)\s+/.exec(text);
    const offset = prompt ? prompt[0].length : 0, tokens = [];
    const pattern = /"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s{}\[\];=]+/g;
    let match;
    while ((match = pattern.exec(text))) {
      if (match.index < offset) continue;
      const raw = match[0], quoted = /^["']/.test(raw), value = quoted ? raw.slice(1, -1) : raw.replace(/[,;:]$/, '');
      if (value) tokens.push({ start: match.index, end: match.index + raw.length, value, word: value.toLowerCase(), quoted });
    }
    const first = tokens[0], verb = !first?.quoted && COMMANDS.has(first?.word) && (first?.word !== 'no' || offset > 0 || FIELDS.has(tokens[1]?.word)), field = !first?.quoted && FIELDS.has(first?.word);
    const hasAddress = /(?:^|\s)(?:\d{1,3}\.){3}\d{1,3}(?=\s|up\b|down\b|$)/i.test(text.slice(offset));
    const hasState = /(?:\b|\d)(?:up|down|connected|notconnect|disabled|active|inactive)(?=\s|$)/i.test(text.slice(offset));
    const namedTable = tokens.length >= 4 && /^(?:static|dhcp|pppoe|manual|auto)$/i.test(tokens[1]?.word) && hasAddress && hasState;
    const interfaceTable = !!first && hasState && (interfaceLabel(first.value, true) || namedTable);
    const isConfig = !!(verb || field && !/^(?:is|are|was|were)$/i.test(tokens[1]?.word || '') || /\{\s*$/.test(text));
    const comments = [];
    if (/^\s*[!#]/.test(text.slice(offset))) comments.push({ start: offset, end: text.length });
    else {
      const index = verb && /^(?:set|append)$/.test(first.word) ? 1 : 0;
      if (/^(?:description|comment|comments|remark|remarks)$/.test(tokens[index]?.word || '') && tokens[index + 1]) comments.push({ start: tokens[index + 1].start, end: text.length });
    }
    return { tokens, verb, isConfig, namedTable, interfaceTable, comments,
      quoted: (start, end) => tokens.some(token => token.quoted && start >= token.start && end <= token.end) };
  }
  function versionContext(text, start) {
    return /\b(?:version|firmware|release|build|revision)\s*[:=]?\s*$/i.test(text.slice(Math.max(0, start - 40), start));
  }
  function negatedOrZero(text, start, end, table = false) {
    const before = text.slice(Math.max(0, start - 48), start);
    const after = text.slice(end, end + 28);
    return /\b(?:no|not|without|zero)\s+$/i.test(before) || !table && (/(?:^|\s)0\s+(?:(?:input|output|crc|frame|packet)\s+)?$/i.test(before) ||
      /^\s*[:=]\s*0(?=$|[\s,;)}\]])/.test(after) || /^\s+0(?=$|[\s,;)}\]])/.test(after));
  }
  function statusContext(text, start, end, syntax) {
    const before = text.slice(Math.max(0, start - 100), start);
    const after = text.slice(end, end + 50);
    if (!before.trim() && !after.trim()) return true;
    if (syntax?.interfaceTable && /\s$/.test(before) && /^(?:\s|$)/.test(after)) return true;
    if (/(?:\b(?:state|status|link|protocol|operstate|carrier)\s*(?::|=|is)?\s*|\bis\s+(?:administratively\s+)?)$/i.test(before)) return true;
    if (/(?:^|[\s])(?:administratively\s+)$/.test(before)) return true;
    if ((/\s{2,}$/.test(before) || !before.trim()) && (/^\s{2,}/.test(after) || !after.trim())) return true;
    if (/\b(?:up|down)\/$/i.test(before) || /^\/(?:up|down)\b/i.test(after)) return true;
    return /[<,]$/.test(before) && /^[,>]/.test(after);
  }
  function stateBoundary(text, start, end) {
    // An exact up/down pair is a status convention, not a path fragment.
    let first = start, last = end;
    if (/^(?:up|down)$/i.test(text.slice(start, end))) {
      const left = /\b(?:up|down)\/$/i.exec(text.slice(0, start));
      const right = /^\/(?:up|down)\b/i.exec(text.slice(end));
      if (left) first = start - left[0].length;
      if (right) last = end + right[0].length;
    }
    return identifierBoundary(text, first, last);
  }
  function identifierBoundary(text, start, end) {
    // Prevent labels inside hostnames, config identifiers, URLs and paths.
    return !/[A-Za-z0-9_./-]/.test(text[start - 1] || '') && !/[A-Za-z0-9_./-]/.test(text[end] || '');
  }

  function analyze(input) {
    const text = typeof input === 'string' ? input : '';
    if (!text || text.length > LIMITS.text) return [];
    const found = [];
    const add = (start, end, kind) => {
      if (found.length >= LIMITS.matches || end <= start || found.some(hit => start < hit.end && end > hit.start)) return;
      found.push({ start, end, kind });
    };
    const syntax = syntaxContext(text);
    for (const comment of syntax.comments) add(comment.start, comment.end, 'comment');
    // Take complete candidates instead of accepting a valid suffix from an
    // invalid IP, version, identifier or longer hardware address.
    const candidates = /[A-Za-z0-9_.:%/~\-]+/g;
    let match;
    while ((match = candidates.exec(text))) {
      let value = match[0];
      if (value.length > LIMITS.token) continue;
      const start = match.index;
      value = value.replace(/\.+$/, ''); // Sentence punctuation is not an octet.
      if (!value || versionContext(text, start)) continue;
      if (mac(value)) add(start, start + value.length, 'mac');
      else if (addressKind(value)) add(start, start + value.length, 'address');
      else {
        const endpoint = /^(\d+\.\d+\.\d+\.\d+):(\d{1,5})?$/.exec(value);
        if (endpoint && ipv4(endpoint[1]) && (endpoint[2] === undefined || Number(endpoint[2]) <= 65535)) add(start, start + value.length, 'address');
        else {
          const label = value.endsWith(':') ? value.slice(0, -1) : value;
          const tokenIndex = syntax.tokens.findIndex(token => start >= token.start && start < token.end), previous = syntax.tokens[tokenIndex - 1];
          const contextual = syntax.interfaceTable && tokenIndex === 0 || syntax.isConfig && /^(?:interface|interfaces|srcintf|dstintf|members|member|tagged|untagged|trunk|source-interface|parent-interface|tunnel-interface)$/.test(previous?.word || '');
          if (interfaceLabel(label, contextual)) add(start, start + label.length, 'interface');
        }
      }
    }
    const phrases = /\b(?:permission denied|access denied|authentication (?:failed|failure)|login failed|connection (?:refused|reset|failed|timed out)|request timed out|destination (?:host |network |port )?unreachable|no route to host|host unreachable|network unreachable)\b/gi;
    while ((match = phrases.exec(text))) {
      if (!syntax.quoted(match.index, match.index + match[0].length) && identifierBoundary(text, match.index, match.index + match[0].length) && !negatedOrZero(text, match.index, match.index + match[0].length)) add(match.index, match.index + match[0].length, 'error');
    }
    const labels = /\b(?:fatal|critical|errors?|warning|warn|failed|failure|success|successful|passed)\b/gi;
    while ((match = labels.exec(text))) {
      const start = match.index, end = start + match[0].length;
      if (syntax.quoted(start, end) || !identifierBoundary(text, start, end) || negatedOrZero(text, start, end)) continue;
      const before = text.slice(Math.max(0, start - 60), start), after = text.slice(end, end + 24);
      if (!(!before.trim() || /[\[(:]\s*$/.test(before) || /^\s*[:\])!]/.test(after) || match[0] === match[0].toUpperCase())) continue;
      const word = match[0].toLowerCase();
      add(start, end, /^(?:warn|warning)$/.test(word) ? 'warning' : /^(?:success|successful|passed)$/.test(word) ? 'success' : 'error');
    }
    const states = /\b(?:administratively down|lower_up|no-carrier|established|listen(?:ing)?|connected|disconnected|unreachable|incomplete|filtered|pending|degraded|dormant|notconnect|active|inactive|up|down|open|closed|ok)\b/gi;
    while ((match = states.exec(text))) {
      const start = match.index, end = start + match[0].length;
      if (syntax.quoted(start, end) || !stateBoundary(text, start, end) || negatedOrZero(text, start, end, syntax.interfaceTable) || !statusContext(text, start, end, syntax)) continue;
      const word = match[0].toLowerCase();
      add(start, end, /^(?:up|lower_up|established|listen|listening|connected|active|open|ok)$/.test(word) ? 'success' : /^(?:pending|filtered|incomplete|degraded|dormant|inactive)$/.test(word) ? 'warning' : 'error');
    }
    // Fixed-width interface tables can omit the gap between an address and
    // its state. Recognize the compact state only inside an identified table.
    if (syntax.interfaceTable) {
      const compact = /((?:\d{1,3}\.){3}\d{1,3})(up|down)(?=\s|$)/gi;
      while ((match = compact.exec(text))) if (ipv4(match[1])) {
        add(match.index, match.index + match[1].length, 'address');
        add(match.index + match[1].length, match.index + match[0].length, match[2].toLowerCase() === 'up' ? 'success' : 'error');
      }
    }
    // A protocol-labelled status is meaningful; an arbitrary number is not.
    const http = /\bHTTP\/(?:1\.[01]|2(?:\.0)?|3(?:\.0)?)\s+([1-5][0-9]{2})(?=\s|$)/gi;
    while ((match = http.exec(text))) {
      const status = Number(match[1]);
      if (status >= 200 && status < 300) add(match.index, match.index + match[0].length, 'success');
      else if (status >= 400) add(match.index, match.index + match[0].length, status >= 500 ? 'error' : 'warning');
    }
    // Only the RFC 5424 PRI+version prefix gets severity classification.
    const syslog = /^(\s*)(<(?:0|[1-9][0-9]{0,2})>1)(?=\s+(?:-|\d{4}-\d{2}-\d{2}T))/i.exec(text);
    if (syslog) {
      const priority = Number(syslog[2].slice(1, syslog[2].indexOf('>')));
      if (priority <= 191 && priority % 8 <= 4) add(syslog[1].length, syslog[1].length + syslog[2].length, priority % 8 < 4 ? 'error' : 'warning');
    }
    syntaxHighlights(text, syntax, found, add);
    return found.sort((a, b) => a.start - b.start || a.end - b.end);
  }

  function syntaxHighlights(text, syntax, found, add) {
    // Fill around higher-priority addresses, interfaces and operational
    // states rather than replacing their categories inside quoted values.
    const fill = (start, end, kind) => {
      const occupied = found.filter(hit => hit.start < end && hit.end > start).sort((a, b) => a.start - b.start);
      let cursor = start;
      for (const hit of occupied) {
        if (cursor < hit.start && /\S/.test(text.slice(cursor, Math.min(hit.start, end)))) add(cursor, Math.min(hit.start, end), kind);
        cursor = Math.max(cursor, hit.end);
      }
      if (cursor < end && /\S/.test(text.slice(cursor, end))) add(cursor, end, kind);
    };
    const tokens = syntax.tokens, first = tokens[0];
    if (!first || syntax.comments.some(comment => comment.start <= first.start)) return;
    if (syntax.interfaceTable) add(first.start, first.end, 'interface');
    if (syntax.verb) add(first.start, first.end, 'command');
    const namedInterfaceFields = /^(?:interface|interfaces|srcintf|dstintf|source-interface|parent-interface|tunnel-interface|tunnel-source|tunnel-destination|aggregate|members|member|tagged|untagged|trunk)$/;
    for (let i = 0; i < tokens.length; i++) {
      const token = tokens[i], previous = tokens[i - 1], next = tokens[i + 1], word = token.word;
      if (syntax.comments.some(comment => token.start >= comment.start)) continue;
      const parameter = syntax.isConfig && i > 0;
      if (parameter && previous && namedInterfaceFields.test(previous.word) && !FIELDS.has(word) && !PROTOCOLS.has(word)) add(token.start, token.end, 'interface');
      const pathField = /^(?:vlan|access|trunk|native|tagged|untagged|aggregate)$/.test(word) && !/^(?:mode|type)$/.test(previous?.word || '') && next && (FIELDS.has(next.word) || /^\d/.test(next.value));
      if (syntax.isConfig && !token.quoted && (i === 0 && !syntax.verb || pathField || FIELDS.has(word) && !VALUES.has(word) && !PROTOCOLS.has(word) || i === 1 && /^(?:set|unset|append|select|unselect|config|configure|conf)$/.test(first.word))) add(token.start, token.start + token.value.length, 'field');
      if (!token.quoted && PROTOCOLS.has(word)) add(token.start, token.start + token.value.length, 'protocol');
      if (!token.quoted && (syntax.isConfig || syntax.interfaceTable) && /^(?:[+-]?\d+(?:\.\d+)?(?:[-,:]\d+)*)(?:ms|s|sec|kbps|mbps|gbps|[kmgt]?(?:bit|bits|bps|byte|bytes)|[kmgt]|%)?$/i.test(word)) add(token.start, token.end, 'number');
      if (token.quoted && syntax.isConfig || parameter && (!FIELDS.has(word) || VALUES.has(word)) || syntax.interfaceTable && VALUES.has(word)) fill(token.start, token.end, 'value');
    }
    // Colon/equal-labelled operational output uses the same field palette.
    const properties = /\b([A-Za-z][A-Za-z0-9_-]*)\s*[:=]\s*/g;
    let property;
    while ((property = properties.exec(text))) if (FIELDS.has(property[1].toLowerCase()) && !syntax.quoted(property.index, property.index + property[1].length)) add(property.index, property.index + property[1].length, 'field');
  }

  function attach(term, options) {
    const opts = options || {};
    let enabled = opts.enabled !== false, theme = themeName(opts.theme), disposed = false;
    let requestedColors = customColors(opts.colors), colors = palette(theme, requestedColors), underlines = opts.underlines !== false;
    let timer = null, active = false, suspended = '', limited = false, count = 0;
    let lastState = '', geometry = '', unsupported = false, scrollView = null;
    let records = new Map();
    const dirty = new Set(), repaint = new Set(), subscriptions = [];
    const clock = () => global.performance && typeof global.performance.now === 'function' ? global.performance.now() : Date.now();
    const getState = () => ({ enabled, active, suspended, limited, count });
    const announce = () => {
      const state = getState(), key = JSON.stringify(state);
      if (key === lastState) return;
      lastState = key;
      if (typeof opts.onState === 'function') { try { opts.onState(state); } catch (_) {} }
    };
    function remove(record) {
      if (!record.marker.isDisposed) repaint.add(record.marker.line);
      for (const item of record.items) {
        if (item.listener) item.listener.dispose();
        item.decoration.dispose();
      }
      record.marker.dispose();
      count = Math.max(0, count - record.items.length);
    }
    function clear() {
      for (const record of records.values()) remove(record);
      records.clear();
      count = 0;
    }
    function visible() {
      const element = term.element;
      return !!element && (typeof element.getClientRects !== 'function' || element.getClientRects().length > 0);
    }
    function view() {
      const buffer = term.buffer.active;
      return { buffer, first: buffer.viewportY, last: Math.min(buffer.length - 1, buffer.viewportY + Math.min(term.rows, LIMITS.rows) - 1) };
    }
    function paint() {
      if (!repaint.size) return;
      // Decorations are computed after parsing, which can follow the text's
      // first paint. Explicitly invalidate the changed visible rows instead
      // of relying on another write, scroll or incidental renderer refresh.
      // Only mutations enter this set, so our onRender listener settles after
      // its verification pass rather than keeping the renderer in a loop.
      const current = view();
      let first = current.last + 1, last = current.first - 1;
      for (const y of repaint) if (y >= current.first && y <= current.last) {
        first = Math.min(first, y); last = Math.max(last, y);
      }
      repaint.clear();
      if (first <= last && typeof term.refresh === 'function') term.refresh(first - current.first, last - current.first);
    }
    function style(element, kind) {
      const color = colors[kind], mark = MARKS[kind];
      element.setAttribute('aria-hidden', 'true');
      element.setAttribute('data-nt-highlight', kind);
      element.style.pointerEvents = 'none';
      element.style.userSelect = 'none';
      element.style.boxSizing = 'border-box';
      element.style.borderBottom = underlines ? mark[1] + 'px ' + mark[0] + ' ' + color : 'none';
      // Underlines provide the non-color cue. A vertical stroke beside a
      // short status such as "up" looks like terminal text or a cursor.
      element.style.borderLeft = 'none';
    }
    function schedule() {
      if (disposed || !enabled || unsupported || timer !== null) return;
      timer = global.setTimeout(flush, LIMITS.delay);
    }
    function enqueue(start, end) {
      if (disposed || !enabled || unsupported) return;
      try {
        const current = view();
        const first = Math.max(current.first, start === undefined ? current.first : start);
        const last = Math.min(current.last, end === undefined ? current.last : end);
        for (let y = first; y <= last; y++) dirty.add(y);
        schedule();
      } catch (_) { unsupported = true; active = false; suspended = 'unsupported'; clear(); announce(); }
    }
    function refresh() {
      if (disposed) return;
      limited = false;
      enqueue();
    }
    function prune(current) {
      const next = new Map();
      for (const record of records.values()) {
        const y = record.marker.line;
        if (record.marker.isDisposed || y < current.first || y > current.last || next.has(y)) remove(record);
        else next.set(y, record);
      }
      records = next;
      for (const y of dirty) if (y < current.first || y > current.last) dirty.delete(y);
    }
    function collect(buffer, y) {
      let first = y, last = y, steps = 0;
      while (first > 0) {
        const line = buffer.getLine(first);
        if (!line || !line.isWrapped) break;
        if (++steps >= LIMITS.wrapRows) return null;
        first--;
      }
      while (last + 1 < buffer.length) {
        const line = buffer.getLine(last + 1);
        if (!line || !line.isWrapped) break;
        if (++steps >= LIMITS.wrapRows) return null;
        last++;
      }
      let text = '';
      const terminalTheme = term.options?.theme || {};
      const rows = [], cell = buffer.getNullCell(), neutralForeground = foregroundPolicy(terminalTheme), normalBackground = backgroundPolicy(terminalTheme);
      for (let row = first; row <= last; row++) {
        const line = buffer.getLine(row);
        if (!line) return null;
        const entry = { y: row, start: text.length, end: text.length, starts: [], ends: [], hidden: [], colorable: [] };
        let rowText = '';
        if (term.cols > LIMITS.columns) return null;
        let columns = Math.min(term.cols, line.length);
        // A wider terminal can retain a wrapped cursor line and append null
        // cells to its old physical row. They are layout padding, not spaces
        // in the logical text. Preserve actual space characters the peer sent.
        // This also removes padding before a wrapped wide glyph.
        if (row < last) while (columns > 0) {
          const tail = line.getCell(columns - 1, cell);
          if (!tail || tail.getChars()) break;
          columns--;
        }
        for (let column = 0; column < columns; column++) {
          const value = line.getCell(column, cell);
          if (!value || value.getWidth() === 0) continue;
          const width = value.getWidth(), chars = value.getChars();
          const visibleChars = chars || ' ';
          // A single terminal cell can contain many combining characters.
          // Reject it before allocating per-character cell mappings.
          if (text.length + rowText.length + visibleChars.length > LIMITS.text) return null;
          const hidden = typeof value.isInvisible === 'function' && !!value.isInvisible();
          // A pager can leave SGR white active until its next reset. Treat
          // neutral/default-equivalent foregrounds as ordinary text so that
          // a remote reset cannot create a permanent plain/colored boundary.
          // Backgrounds matching the console are ordinary text too. Distinct
          // colors/backgrounds, inverse and hidden text keep their original
          // presentation; the parsed cells are never changed.
          const colorable = neutralForeground(value) &&
            normalBackground(value) &&
            typeof value.isInverse === 'function' && !value.isInverse() &&
            typeof value.isInvisible === 'function' && !hidden;
          rowText += visibleChars;
          for (let i = 0; i < visibleChars.length; i++) {
            entry.starts.push(column);
            entry.ends.push(Math.min(term.cols, column + width));
            entry.hidden.push(hidden);
            entry.colorable.push(!!colorable);
          }
        }
        text += rowText;
        entry.end = text.length;
        rows.push(entry);
      }
      return { text, rows };
    }
    function updateRow(y, segments, current) {
      const key = segments.map(part => part.x + ':' + part.width + ':' + part.kind + ':' + Number(part.colorable)).join('|');
      const previous = records.get(y);
      if (previous && previous.key === key && !previous.marker.isDisposed) return;
      if (previous) { remove(previous); records.delete(y); }
      if (!segments.length) return;
      if (count + segments.length > LIMITS.decorations) { limited = true; return; }
      const marker = term.registerMarker(y - (current.buffer.baseY + current.buffer.cursorY));
      if (!marker || marker.isDisposed) return;
      const items = [];
      try {
        for (const part of segments) {
          const settings = { marker, x: part.x, width: part.width, height: 1, layer: 'bottom' };
          if (part.colorable) settings.foregroundColor = colors[part.kind];
          const decoration = term.registerDecoration(settings);
          if (!decoration) continue;
          const item = { decoration, listener: null, kind: part.kind, element: decoration.element };
          item.listener = decoration.onRender(element => { item.element = element; style(element, part.kind); });
          if (item.element) style(item.element, part.kind);
          items.push(item);
        }
      } catch (error) {
        for (const item of items) { item.listener.dispose(); item.decoration.dispose(); }
        marker.dispose();
        throw error;
      }
      if (!items.length) { marker.dispose(); return; }
      records.set(y, { marker, items, key, segments });
      count += items.length;
      repaint.add(y);
    }
    function scan(y, current) {
      const group = collect(current.buffer, y);
      if (!group) {
        limited = true;
        dirty.delete(y);
        updateRow(y, [], current);
        return 1;
      }
      const matches = analyze(group.text);
      if (matches.length === LIMITS.matches) limited = true;
      // Never decorate invisible SGR 8 cells or disclose their token shape.
      const safe = matches.filter(hit => !group.rows.some(row => {
        const start = Math.max(hit.start, row.start), end = Math.min(hit.end, row.end);
        for (let i = start; i < end; i++) if (row.hidden[i - row.start]) return true;
        return false;
      }));
      let scanned = 0;
      for (const row of group.rows) {
        if (row.y < current.first || row.y > current.last) continue;
        const segments = [];
        for (const hit of safe) {
          const first = Math.max(hit.start, row.start), last = Math.min(hit.end, row.end);
          if (first >= last) continue;
          // A peer may change ANSI colors in the middle of an address or
          // label. Split its visual span at those boundaries without changing
          // the recognized token, original cells or any remote attributes.
          for (let start = first; start < last;) {
            const colorable = row.colorable[start - row.start];
            let finish = start + 1;
            while (finish < last && row.colorable[finish - row.start] === colorable) finish++;
            const x = row.starts[start - row.start], end = row.ends[finish - row.start - 1];
            if (Number.isInteger(x) && end > x) segments.push({ x, width: end - x, kind: hit.kind, colorable });
            start = finish;
          }
        }
        updateRow(row.y, segments, current);
        dirty.delete(row.y);
        scanned++;
      }
      return Math.max(1, scanned);
    }
    function flush(completeViewport = false) {
      if (timer !== null) global.clearTimeout(timer);
      timer = null;
      if (disposed || !enabled || unsupported) return;
      try {
        const current = view();
        suspended = current.buffer.type === 'alternate' ? 'alternate' : !visible() ? 'hidden' : '';
        active = !suspended;
        if (!active) { dirty.clear(); clear(); paint(); announce(); return; }
        const size = term.cols + ':' + term.rows;
        if (size !== geometry) { clear(); geometry = size; }
        prune(current);
        const began = clock();
        let scanned = 0;
        // Always finish at least one logical line. A delayed/slow clock check
        // must not indefinitely defer the entire queue on a busy UI thread.
        while (dirty.size && (completeViewport || (scanned < LIMITS.frameRows && (scanned === 0 || clock() - began < LIMITS.frameMs)))) {
          const y = dirty.values().next().value;
          scanned += scan(y, current);
        }
        paint();
        announce();
        if (dirty.size) schedule();
      } catch (_) {
        unsupported = true; active = false; suspended = 'unsupported'; dirty.clear(); clear(); paint(); announce();
      }
    }
    function subscribe(event, callback) {
      if (typeof event === 'function') subscriptions.push(event(callback));
    }
    function tokensAtRow(y) {
      if (disposed || !enabled || !active || suspended || !Number.isInteger(y)) return [];
      try {
        const current = view();
        if (y < current.first || y > current.last) return [];
        const group = collect(current.buffer, y);
        if (!group) return [];
        const matches = analyze(group.text), result = [];
        for (const hit of matches) {
          let hidden = false;
          const spans = [];
          for (const row of group.rows) {
            const first = Math.max(hit.start, row.start), last = Math.min(hit.end, row.end);
            if (first >= last) continue;
            for (let i = first; i < last; i++) if (row.hidden[i - row.start]) hidden = true;
            const x = row.starts[first - row.start], end = row.ends[last - row.start - 1];
            if (Number.isInteger(x) && end > x) spans.push({ row: row.y, x, end });
          }
          // Expose only marks that currently exist, not merely recognizable
          // text that was skipped by the bounded highlighter or hidden by SGR.
          const marked = spans.some(span => [...records.values()].some(record =>
            !record.marker.isDisposed && record.marker.line === span.row &&
            record.segments.some(part => part.kind === hit.kind && part.x < span.end && part.x + part.width > span.x)));
          if (!hidden && marked) result.push({ text: group.text.slice(hit.start, hit.end), kind: hit.kind, spans });
        }
        return result;
      } catch (_) { return []; }
    }
    try {
      if (!term || typeof term.registerMarker !== 'function' || typeof term.registerDecoration !== 'function' || !term.buffer || !term.buffer.active) throw new Error('Unsupported terminal API');
      scrollView = { type: term.buffer.active.type, base: term.buffer.active.baseY, first: term.buffer.active.viewportY };
      subscribe(term.onWriteParsed, () => {
        // Keep the existing immediate reconciliation of old marks: changed
        // ANSI/invisible attributes must take precedence before the next paint,
        // including marked rows beyond this turn's new-row scanning budget.
        try { const current = view(); prune(current); for (const y of [...records.keys()]) scan(y, current); }
        catch (_) { clear(); }
        // Start new and rewritten rows in this parsing turn, before the next
        // terminal paint. Large views continue in bounded batches. Scanning
        // only existing marks left new rows dependent on a later timer/render.
        enqueue();
        flush();
      });
      subscribe(term.onScroll, () => {
        const current = view(), previous = scrollView;
        scrollView = { type: current.buffer.type, base: current.buffer.baseY, first: current.first };
        // A user/programmatic viewport move must finish before xterm's next
        // repaint, including rows above the previous view. Waiting 40 ms lets
        // a plain block appear above the already-colored overlapping rows.
        // Output growth and scrollback trimming also emit onScroll. Keep those
        // on the parsed-write path instead of scanning per incoming newline.
        const moved = previous && previous.type === 'normal' && scrollView.type === 'normal' &&
          previous.base === scrollView.base && previous.first !== scrollView.first &&
          ![...records].some(([y, record]) => record.marker.isDisposed || record.marker.line !== y);
        enqueue();
        if (moved) flush(true);
      });
      subscribe(term.onResize, () => {
        scrollView = { type: term.buffer.active.type, base: term.buffer.active.baseY, first: term.buffer.active.viewportY };
        clear(); dirty.clear(); refresh(); announce();
      });
      subscribe(term.onRender, range => {
        try { const first = term.buffer.active.viewportY; enqueue(first + range.start, first + range.end); } catch (_) {}
      });
      subscribe(term.buffer.onBufferChange, () => {
        // Clear normal-buffer marks before the alternate screen renders.
        clear(); dirty.clear(); active = false;
        scrollView = { type: term.buffer.active.type, base: term.buffer.active.baseY, first: term.buffer.active.viewportY };
        suspended = enabled && term.buffer.active.type === 'alternate' ? 'alternate' : '';
        refresh(); announce();
      });
      if (typeof global.ResizeObserver === 'function' && term.element) {
        const observer = new global.ResizeObserver(refresh);
        observer.observe(term.element);
        subscriptions.push({ dispose: () => observer.disconnect() });
      }
    } catch (_) { unsupported = true; suspended = 'unsupported'; }
    function setAppearance(value = {}) {
      if (disposed) return;
      const nextTheme = themeName(value.theme), nextRequested = customColors(value.colors), nextColors = palette(nextTheme, nextRequested);
      const changed = Object.keys(LABELS).some(kind => colors[kind] !== nextColors[kind]);
      theme = nextTheme; requestedColors = nextRequested; colors = nextColors; underlines = value.underlines !== false;
      // Foreground options are immutable in xterm's public API. Underline-only
      // edits restyle existing elements without replacing their decorations.
      if (changed) clear();
      else for (const record of records.values()) for (const item of record.items) if (item.element) style(item.element, item.kind);
      enqueue();
    }
    const controller = {
      getState,
      tokensAtRow,
      tokenAt(column, row) {
        if (!Number.isInteger(column) || column < 0 || column >= term.cols) return null;
        return tokensAtRow(row).find(token => token.spans.some(span => span.row === row && column >= span.x && column < span.end)) || null;
      },
      setEnabled(value) {
        if (disposed) return;
        enabled = !!value;
        if (!enabled) {
          if (timer !== null) global.clearTimeout(timer);
          timer = null; dirty.clear(); clear(); paint(); active = false; suspended = ''; limited = false; announce();
        } else if (unsupported) { suspended = 'unsupported'; announce(); }
        else refresh();
      },
      setTheme(value) {
        setAppearance({ theme: value, colors: requestedColors, underlines });
      },
      setAppearance,
      refresh,
      dispose() {
        if (disposed) return;
        disposed = true;
        if (timer !== null) global.clearTimeout(timer);
        timer = null; dirty.clear();
        for (const subscription of subscriptions) if (subscription && typeof subscription.dispose === 'function') subscription.dispose();
        clear(); paint(); active = false;
      }
    };
    announce();
    if (enabled && !unsupported) refresh();
    return controller;
  }

  global.NTTerminalHighlight = Object.freeze({ attach, analyze, legend, limits: LIMITS });
})(typeof globalThis !== 'undefined' ? globalThis : window);
