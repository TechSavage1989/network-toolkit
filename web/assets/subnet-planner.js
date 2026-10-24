/* Network Toolkit IPv4 subnet arithmetic. No I/O, dependencies or scan actions.
 * The rows are mathematical partitions, not assigned or discovered networks.
 *
 * Standards reviewed 2026-10-21:
 * https://www.rfc-editor.org/rfc/rfc791.html   section 3.2: historical A/B/C
 * https://www.rfc-editor.org/rfc/rfc1112.html  section 4: multicast D/reserved E
 * https://www.rfc-editor.org/rfc/rfc4632.html  section 3.1: CIDR, /0, /31, /32
 * https://www.rfc-editor.org/rfc/rfc3021.html  sections 2.1-2.2: /31 endpoints
 * https://www.iana.org/assignments/iana-ipv4-special-registry/
 *
 * All addresses/counts use exact Number arithmetic (at most 2^32). In
 * particular, counts and block sizes never use signed 32-bit bitwise coercion.
 * page() performs constant-time interval intersection and builds <=128 rows;
 * it never enumerates or caches an entire parent network.
 */
(function (global, factory) {
  'use strict';
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  else global.NetworkToolkitSubnetPlanner = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  'use strict';

  const SPACE = 4294967296;
  const MAX_IP = SPACE - 1;
  const MAX_PAGE_SIZE = 128;
  const MATHEMATICAL_NOTE = 'Address counts and ranges are mathematical; they do not establish assignment, availability or reachability.';

  function fail(code, message) {
    const error = new Error(message);
    error.code = 'subnet_' + code;
    throw error;
  }

  function parseIPv4(value) {
    if (typeof value === 'number') {
      if (Number.isInteger(value) && value >= 0 && value <= MAX_IP) return value;
      fail('address', 'Enter a valid IPv4 address.');
    }
    if (typeof value !== 'string' || value.length > 64) fail('address', 'Enter a valid IPv4 address.');
    const parts = value.trim().split('.');
    if (parts.length !== 4 || parts.some(part => !/^\d{1,3}$/.test(part) || Number(part) > 255)) {
      fail('address', 'Enter a valid IPv4 address with four decimal octets.');
    }
    // Leading zeros are decimal, matching the calculator's existing parser.
    return parts.reduce((address, part) => address * 256 + Number(part), 0);
  }

  function formatIPv4(value) {
    const address = parseIPv4(value);
    return [Math.floor(address / 16777216), Math.floor(address / 65536) % 256,
      Math.floor(address / 256) % 256, address % 256].join('.');
  }

  function parsePrefix(value) {
    if (typeof value === 'number') {
      if (Number.isInteger(value) && value >= 0 && value <= 32) return value;
      fail('prefix', 'Choose an IPv4 prefix from 0 to 32.');
    }
    if (typeof value !== 'string' || value.length > 64) {
      fail('prefix', 'Enter a prefix from 0 to 32 or a contiguous IPv4 subnet mask.');
    }
    const text = value.trim();
    if (/^\/?\d{1,2}$/.test(text)) return parsePrefix(Number(text.replace('/', '')));
    let mask;
    try { mask = parseIPv4(text); }
    catch (_) { fail('prefix', 'Enter a prefix from 0 to 32 or a contiguous IPv4 subnet mask.'); }
    // 33 comparisons are bounded and avoid floating-point log rounding for
    // invalid masks close to a power of two.
    for (let prefix = 0; prefix <= 32; prefix++) {
      if (mask === SPACE - 2 ** (32 - prefix)) return prefix;
    }
    fail('mask', 'The subnet mask must have consecutive 1 bits followed by 0 bits.');
  }

  function parseInput(input, fallbackPrefix) {
    if (typeof input === 'number') return { ip: parseIPv4(input), prefix: parsePrefix(fallbackPrefix) };
    if (typeof input !== 'string' || input.length > 96) {
      fail('input', 'Enter an IPv4 address, optionally followed by a CIDR prefix or subnet mask.');
    }
    const match = /^([^\s/]+)(?:\s*\/\s*([^\s/]+)|[ \t]+([^\s/]+))?$/.exec(input.trim());
    if (!match) fail('input', 'Enter an IPv4 address, optionally followed by one CIDR prefix or subnet mask.');
    return { ip: parseIPv4(match[1]), prefix: parsePrefix(match[2] ?? match[3] ?? fallbackPrefix) };
  }

  function subnet(ip, prefix) {
    const total = 2 ** (32 - prefix);
    const network = Math.floor(ip / total) * total;
    const upper = network + total - 1;
    const wildcard = total - 1;
    const mask = MAX_IP - wildcard;
    const endpoints = prefix >= 31;
    const first = endpoints ? network : network + 1;
    const last = endpoints ? upper : upper - 1;
    let note = MATHEMATICAL_NOTE;
    if (prefix === 0) {
      note = 'A /0 covers the entire IPv4 address space and is the default route. Its upper address, 255.255.255.255, is limited broadcast. The conventional host count does not imply that all addresses can be assigned.';
    } else if (prefix === 31) {
      note = 'For an RFC 3021 point-to-point link, both addresses are endpoints and there is no separate directed broadcast. ' + MATHEMATICAL_NOTE;
    } else if (prefix === 32) {
      note = 'A /32 is one host route with no separate broadcast address. ' + MATHEMATICAL_NOTE;
    }
    return {
      ip, address: formatIPv4(ip), prefix, mask, maskAddress: formatIPv4(mask),
      wildcard, wildcardAddress: formatIPv4(wildcard), network, networkAddress: formatIPv4(network),
      cidr: formatIPv4(network) + '/' + prefix, upper, upperAddress: formatIPv4(upper),
      broadcast: endpoints ? null : upper, broadcastAddress: endpoints ? null : formatIPv4(upper),
      total, usable: endpoints ? total : total - 2, first, last,
      firstAddress: formatIPv4(first), lastAddress: formatIPv4(last),
      kind: prefix === 32 ? 'host' : prefix === 31 ? 'point-to-point' : 'subnet', note
    };
  }

  function calculate(input, fallbackPrefix = 24) {
    const parsed = parseInput(input, fallbackPrefix);
    return subnet(parsed.ip, parsed.prefix);
  }

  function classify(address) {
    const ip = parseIPv4(address);
    const first = Math.floor(ip / 16777216);
    let letter, defaultPrefix, range, purpose, exception = null;
    if (first < 128) {
      letter = 'A'; defaultPrefix = 8; range = '0.0.0.0 - 127.255.255.255'; purpose = 'Historical unicast format';
    } else if (first < 192) {
      letter = 'B'; defaultPrefix = 16; range = '128.0.0.0 - 191.255.255.255'; purpose = 'Historical unicast format';
    } else if (first < 224) {
      letter = 'C'; defaultPrefix = 24; range = '192.0.0.0 - 223.255.255.255'; purpose = 'Historical unicast format';
    } else if (first < 240) {
      letter = 'D'; defaultPrefix = null; range = '224.0.0.0 - 239.255.255.255'; purpose = 'Multicast';
    } else {
      letter = 'E'; defaultPrefix = null; range = '240.0.0.0 - 255.255.255.255'; purpose = 'Reserved';
    }
    let note = defaultPrefix === null
      ? (letter === 'D' ? 'Class D addresses identify IPv4 multicast groups.' : 'Class E address space is reserved by protocol.') + ' There is no historical unicast default subnet mask.'
      : 'Historical Class ' + letter + ' has a /' + defaultPrefix + ' default mask. CIDR determines the chosen subnet boundaries independently of this class.';
    if (first === 0) {
      exception = ip === 0 ? 'This host on this network' : 'This network';
      purpose = 'Special use';
      note += ' The 0.0.0.0/8 range has special this-network semantics; it is not an ordinary assignable Class A network.';
    } else if (first === 127) {
      exception = 'Loopback'; purpose = 'Special use';
      note += ' The entire 127.0.0.0/8 range is loopback, not an ordinary assignable Class A network.';
    } else if (ip === MAX_IP) {
      exception = 'Limited broadcast'; purpose = 'Limited broadcast';
      note += ' 255.255.255.255 is the limited-broadcast address.';
    }
    return { class: letter, label: 'Class ' + letter + ' (historical)', defaultPrefix,
      defaultMask: defaultPrefix === null ? null : formatIPv4(SPACE - 2 ** (32 - defaultPrefix)),
      range, purpose, exception, note };
  }

  function defaultParentPrefix(address, prefix) {
    parseIPv4(address);
    const selectedPrefix = parsePrefix(prefix);
    // Keep only the complete octets fixed by the selected mask. This display
    // scope is independent of historical address classes. Aligned prefixes
    // (including /0 and /32) have one possible network; the maximum is 128.
    return 8 * Math.floor(selectedPrefix / 8);
  }

  function parentPattern(parent) {
    // Explicit non-octet scopes remain supported by the low-level arithmetic
    // API. A CIDR describes them exactly; wildcard octets would overstate them.
    if (parent.prefix % 8 !== 0) return parent.cidr;
    const fixedOctets = parent.prefix / 8;
    return parent.networkAddress.split('.').map((octet, index) => index < fixedOctets ? octet : '*').join('.');
  }

  function pageInteger(value, fallback, name) {
    if (value === undefined) return fallback;
    if (typeof value === 'string' && /^\d{1,16}$/.test(value.trim())) value = Number(value.trim());
    if (!Number.isSafeInteger(value)) fail('page', name + ' must be a whole number.');
    return value;
  }

  /* page({address,prefix,parentPrefix?,filter?,sort?,page?,pageSize?})
   * Omit parentPrefix for an automatic scope based on the mask's complete
   * octets. basePattern labels that scope with wildcard octets, or with its
   * CIDR when a non-octet parentPrefix is explicitly supplied.
   * page numbers/result positions are one-based; row.index is the zero-based
   * ascending index in the complete parent. Empty results have page=1,
   * pageCount=0 and firstResult=lastResult=0. currentSubnetPage is null when
   * filtered out. Filtering accepts one IP or CIDR and includes any intersecting
   * child; a narrower filter therefore selects its containing child subnet.
   */
  function page(options) {
    if (!options || typeof options !== 'object' || Array.isArray(options)) {
      fail('options', 'Enter subnet planner options.');
    }
    const selected = calculate(options.address, options.prefix === undefined ? 24 : options.prefix);
    const classInfo = classify(selected.ip);
    const parentPrefix = options.parentPrefix === undefined
      ? defaultParentPrefix(selected.ip, selected.prefix) : parsePrefix(options.parentPrefix);
    if (parentPrefix > selected.prefix) fail('parent', 'The containing parent prefix must be from /0 through /' + selected.prefix + '.');
    const parent = subnet(selected.ip, parentPrefix);
    const networkCount = 2 ** (selected.prefix - parentPrefix);
    const sort = options.sort === undefined ? 'asc' : options.sort;
    if (sort !== 'asc' && sort !== 'desc') fail('sort', 'Choose ascending or descending network order.');
    const pageSize = Math.max(1, Math.min(MAX_PAGE_SIZE, pageInteger(options.pageSize, 50, 'Page size')));
    const requestedPage = Math.max(1, pageInteger(options.page, 1, 'Page number'));
    if (options.filter !== undefined && (typeof options.filter !== 'string' || options.filter.length > 96)) {
      fail('filter', 'Filter by one IPv4 address or CIDR network.');
    }
    const filter = options.filter === undefined ? '' : options.filter.trim();
    let firstIndex = 0, lastIndex = networkCount - 1;
    if (filter) {
      const match = calculate(filter, 32);
      const lower = Math.max(parent.network, match.network);
      const upper = Math.min(parent.upper, match.upper);
      if (lower > upper) { firstIndex = 0; lastIndex = -1; }
      else {
        firstIndex = Math.floor((lower - parent.network) / selected.total);
        lastIndex = Math.floor((upper - parent.network) / selected.total);
      }
    }
    const total = lastIndex - firstIndex + 1;
    const pageCount = Math.ceil(total / pageSize);
    const currentPage = Math.min(requestedPage, Math.max(1, pageCount));
    const offset = (currentPage - 1) * pageSize;
    const count = Math.min(pageSize, total - offset);
    const selectedIndex = (selected.network - parent.network) / selected.total;
    const selectedPosition = sort === 'desc' ? lastIndex - selectedIndex : selectedIndex - firstIndex;
    const currentSubnetPage = selectedIndex >= firstIndex && selectedIndex <= lastIndex
      ? Math.floor(selectedPosition / pageSize) + 1 : null;
    const rows = [];
    for (let i = 0; i < count; i++) {
      const index = sort === 'desc' ? lastIndex - offset - i : firstIndex + offset + i;
      rows.push(Object.assign(subnet(parent.network + index * selected.total, selected.prefix),
        { index, ordinal: index + 1, selected: index === selectedIndex }));
    }
    return { classInfo, selected, parent, parentPrefix, basePattern: parentPattern(parent), networkCount, total,
      page: currentPage, pageSize, pageCount, firstResult: total ? offset + 1 : 0,
      lastResult: total ? offset + count : 0, currentSubnetPage, filter, sort, rows };
  }

  return Object.freeze({ parseIPv4, formatIPv4, parsePrefix, calculate, classify, defaultParentPrefix, page });
});
