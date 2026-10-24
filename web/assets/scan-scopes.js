/* Offline IPv4 scope and TCP list planning. Keep grammar aligned with internal/diag/scan_scopes.go. */
(function (root) {
  'use strict';
  const MAX_INPUT_BYTES = 16384;
  const SPACE = /[,;\x20\t\r\n\f\v]+/;
  const trimSpace = value => String(value).replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '');
  const normalise = value => trimSpace(String(value == null ? '' : value)).replace(/[\t ]*([-\/])[\t ]*/g, '$1');
  const toAddress = value => [24, 16, 8, 0].map(shift => (value >>> shift) & 255).join('.');
  function toNumber(value) {
    if (!/^(?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}$/.test(value)) throw new Error('Enter a valid IPv4 address.');
    const parts = value.split('.').map(Number);
    if (parts.some(part => part > 255)) throw new Error('IPv4 octets must be between 0 and 255.');
    return parts.reduce((total, part) => total * 256 + part, 0);
  }
  function maskPrefix(mask) {
    const number = toNumber(mask);
    let prefix = 0, zeroSeen = false;
    for (let bit = 31; bit >= 0; bit--) {
      if ((number >>> bit) & 1) {
        if (zeroSeen) throw new Error('Subnet masks must contain consecutive one bits followed by zero bits.');
        prefix++;
      } else zeroSeen = true;
    }
    return prefix;
  }
  function network(address, prefix) {
    const size = Math.pow(2, 32 - prefix), start = Math.floor(address / size) * size;
    return {canonical: toAddress(start) + '/' + prefix, first: start + (prefix < 31 ? 1 : 0), last: start + size - (prefix < 31 ? 2 : 1), prefix};
  }
  function parseEntry(raw) {
    if (raw.endsWith('.*')) {
      const base = raw.slice(0, -1) + '0';
      return {...network(toNumber(base), 24), raw};
    }
    if (raw.includes('/')) {
      const parts = raw.split('/');
      if (parts.length !== 2 || !parts[1]) throw new Error('Use address/CIDR or address/subnet-mask.');
      const address = toNumber(parts[0]);
      const prefix = parts[1].includes('.') ? maskPrefix(parts[1]) : /^\d{1,2}$/.test(parts[1]) ? Number(parts[1]) : -1;
      if (prefix < 0 || prefix > 32) throw new Error('IPv4 CIDR prefixes must be between 0 and 32.');
      return {...network(address, prefix), raw};
    }
    if (raw.includes('-')) {
      const parts = raw.split('-');
      if (parts.length !== 2 || !parts[0] || !parts[1]) throw new Error('Use an ascending IPv4 range such as 192.168.1.1-254.');
      const first = toNumber(parts[0]);
      const end = /^\d{1,3}$/.test(parts[1]) ? parts[0].split('.').slice(0, 3).join('.') + '.' + Number(parts[1]) : parts[1];
      const last = toNumber(end);
      if (last < first) throw new Error('IPv4 range end must be at or after its start.');
      return {raw, canonical: toAddress(first) + '-' + toAddress(last), first, last};
    }
    const address = toNumber(raw);
    return {raw, canonical: toAddress(address), first: address, last: address};
  }
  function tokens(value) { return normalise(value).split(SPACE).filter(Boolean); }
  function parse(value) {
    const text = String(value == null ? '' : value);
    if (new TextEncoder().encode(text).length > MAX_INPUT_BYTES) throw new Error('The scan scope list is too long. Use no more than 16,384 bytes.');
    const entries = tokens(text);
    if (!entries.length) throw new Error('Add a local network or enter at least one IPv4 network, host or range.');
    const scopes = [], parsed = [], seen = new Set();
    for (const entry of entries) {
      let scope;
      try { scope = parseEntry(entry); }
      catch (error) { throw new Error('Invalid scope "' + entry.slice(0, 100) + '": ' + error.message); }
      if (!seen.has(scope.canonical)) { seen.add(scope.canonical); scopes.push(scope.canonical); parsed.push(scope); }
    }
    const intervals = [];
    for (const scope of parsed.slice().sort((a, b) => a.first - b.first || a.last - b.last)) {
      const previous = intervals[intervals.length - 1];
      if (previous && scope.first <= previous.last + 1) previous.last = Math.max(previous.last, scope.last);
      else intervals.push({first: scope.first, last: scope.last});
    }
    const hosts = intervals.reduce((count, interval) => count + interval.last - interval.first + 1, 0);
    return {scopes, entries: parsed, intervals, hosts, first: intervals[0].first, last: intervals[intervals.length - 1].last,
      overlaps: parsed.reduce((count, entry) => count + entry.last - entry.first + 1, 0) - hosts};
  }
  function parsePorts(value, limit) {
    if (new TextEncoder().encode(String(value == null ? '' : value)).length > MAX_INPUT_BYTES) throw new Error('The TCP port list is too long. Use no more than 16,384 bytes.');
    const text = trimSpace(String(value == null ? '' : value)).replace(/[\t ]*-[\t ]*/g, '-'), ports = new Set();
    const groups = text.split(',');
    if (groups.some(group => !trimSpace(group))) throw new Error('Enter TCP ports or ranges. Separate entries with commas, spaces or new lines.');
    for (const group of groups) {
      const parts = group.split(/[;\x20\t\r\n\f\v]+/).filter(Boolean);
      if (!parts.length) throw new Error('Enter at least one TCP port or range.');
      for (const raw of parts) {
        const part = trimSpace(raw);
        const match = /^(\d+)(?:-(\d+))?$/.exec(part);
        if (!match) throw new Error('Invalid TCP entry "' + part.slice(0, 80) + '". Use port numbers or ranges such as 8000-8010.');
        const first = Number(match[1]), last = match[2] ? Number(match[2]) : first;
        if (first < 1 || last > 65535 || last < first || !Number.isSafeInteger(first) || !Number.isSafeInteger(last)) throw new Error('TCP ports must be from 1 to 65,535 with ascending ranges.');
        if (last - first + 1 > limit) throw new Error('Choose no more than ' + limit.toLocaleString() + ' TCP ports for this tool.');
        for (let port = first; port <= last; port++) {
          ports.add(port);
          if (ports.size > limit) throw new Error('Choose no more than ' + limit.toLocaleString() + ' TCP ports for this tool.');
        }
      }
    }
    return [...ports].sort((a, b) => a - b);
  }
  const api = Object.freeze({parse, parseEntry, parsePorts, tokens, toNumber, toAddress, MAX_INPUT_BYTES});
  root.NetworkToolkitScanScopes = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof globalThis !== 'undefined' ? globalThis : this);
