/* Terminal syntax highlighting rule sets in the style of MobaXterm (owner
 * request, v1.21.6). A rule set has up to six text colors plus underline; each
 * is a regular expression or a comma-separated word list. Rule sets can be
 * imported from and exported to MobaXterm's .msyn files.
 *
 * MobaXterm patterns use POSIX bracket expressions ("[]x]" and "[^\]" are a
 * literal "]" and "not a backslash") and mark the start and end of a line with
 * "¨" (U+00A8). They are converted to JavaScript regular expressions here. The
 * first capture group is colored when it matched, otherwise the whole match.
 *
 * Built-in rule sets are written for Network Toolkit; they cover the same
 * kinds of words as MobaXterm's presets. Colors come from the terminal's
 * color scheme. Nothing here writes to the terminal or keeps terminal text.
 */
(function (global) {
  'use strict';

  const COLORS = Object.freeze(['red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'underline']);
  const LABELS = Object.freeze({ red: 'Red', green: 'Green', yellow: 'Yellow', blue: 'Blue', magenta: 'Magenta', cyan: 'Cyan', underline: 'Underline' });
  const LIMITS = Object.freeze({ pattern: 8000, name: 80, text: 4096, hitsPerColor: 96, custom: 50, words: 2000, probeMs: 25 });
  const LINE = '¨';

  // Shared pieces for the built-in rule sets (lookarounds keep spans tight).
  const B = '(?<=[^A-Za-z0-9_&-])', E = '(?=[^A-Za-z0-9_-])';
  const URL = B + '((?:https?|ftps?|sftp|ssh|telnet)://[A-Za-z0-9_.&?=%~#{}()@+:/!$*,;\'-]*[A-Za-z0-9_/=#~)-])';
  const IPV4 = '(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])(?:\\.(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])){3}(?:/[0-9]{1,2})?';
  const MAC = '[0-9A-Fa-f]{2}(?:[:-][0-9A-Fa-f]{2}){5}|[0-9A-Fa-f]{4}\\.[0-9A-Fa-f]{4}\\.[0-9A-Fa-f]{4}';
  const words = list => list.split(/\s*\|\s*/).filter(Boolean).join('|');

  const BUILT_IN = Object.freeze([
    { id: 'standard', name: 'Standard keywords (OK/warning/error/…)', regex: true, caseSensitive: false,
      notes: { red: 'Errors and failures', green: 'Success', yellow: 'Warnings', magenta: 'Addresses and empty values', cyan: 'Information', underline: 'Web addresses' },
      rules: {
        red: B + '((?:access |permission |authentication |connection |operation )?(?:denied|refused|rejected|failed|failure)|' + words('fatal|critical|errors?|crash(?:ed)?|core dumped|segmentation fault|seg ?fault|panic|abort(?:ed)?|invalid|unsupported|not supported|not permitted|not allowed|not implemented|no such (?:file|directory|host|device|user)|unreachable|corrupt(?:ed|ion)?|overflow|bad (?:file|address|argument|parameter|request|gateway|password)|permission problem|not ok') + ')' + E + '|[=:>"\'(\\[] *(false|no|ko|fail)(?= *[]=:<"\')},;. ]|' + LINE + ')',
        green: B + '(' + words('success(?:ful(?:ly)?)?|succeeded|accepted|allowed|enabled|connected|established|authenticated|completed?|passed|ready|online') + ')' + E + '|[=:>"\'(\\[] *(true|yes|ok)(?= *[]=:<"\')},;. ]|' + LINE + ')',
        yellow: B + '(' + words('warn(?:ing)?s?|deprecated|caution|attention|alert|unknown|unexpected|retry(?:ing)?|cannot|could not|unable to|not found|no more|disabled|disconnected|closed|terminated|stopped|exited|timed out|time ?out|not responding|out of (?:memory|space)|low (?:memory|disk)') + ')' + E,
        blue: '',
        magenta: '(?<=[^0-9A-Za-z_&.:-])(' + IPV4 + '|' + MAC + '|localhost|null|none)(?=[^0-9A-Za-z_-])',
        cyan: B + '(' + words('info(?:rmation(?:al)?)?|notice|note|debug|hint|starting|started|loading|checking|creating|building|booting|launching|connecting|last (?:failed )?login:?') + ')(?=[^A-Za-z0-9_-]|:)',
        underline: URL
      } },
    { id: 'shell', name: 'Unix shell script', regex: true, caseSensitive: true,
      notes: { red: 'Command substitution', green: 'Comments', yellow: 'Variables', blue: 'Redirections, operators and color codes', magenta: 'Keywords', cyan: 'Built-in commands', underline: 'Web addresses' },
      rules: {
        red: '(\\$\\([^()' + LINE + ']*\\)|`[^`' + LINE + ']+`)',
        green: '(?<=[' + LINE + ' \\t;])(#[^' + LINE + ']*)',
        yellow: '(\\$(?:[A-Za-z_][A-Za-z0-9_]*|\\{[^}' + LINE + ']+\\}|[0-9?@#$!*-]))',
        blue: '(\\\\(?:033|e|x1[bB])\\[[0-9;]*m|/dev/null|&&|\\|\\||[0-9]?>>?|<<-?)',
        magenta: '(?<=[' + LINE + '\\s;(){}&|])(' + words('if|then|else|elif|fi|for|foreach|while|until|do|done|case|esac|in|select|function|return|break|continue|time') + ')(?=[' + LINE + '\\s;(){}&|])',
        cyan: '(?<=[' + LINE + '\\s;(|&`])(' + words('echo|printf|export|unset|setenv|local|readonly|declare|typeset|alias|unalias|source|set|shift|read|exit|cd|pwd|test|trap|eval|exec|ulimit|umask|shopt|builtin|command|type|which|wait|kill|sudo') + ')(?=[' + LINE + '\\s;)|&`])',
        underline: URL
      } },
    { id: 'network', name: 'Network configuration (Cisco and others)', regex: true, caseSensitive: false,
      notes: { red: 'Down, disabled, denied and "no" commands', green: 'Up, permitted, names and descriptions', yellow: 'Addresses, VLANs and interfaces', blue: 'Comment lines', magenta: 'Configuration sections', cyan: 'Commands and settings', underline: 'Web addresses' },
      rules: {
        red: '(?<=' + LINE + ' *)(no)(?= )|' + B + '(' + words('administratively down|down|err-?disabled|disabled|shutdown|notconnect|not ?connected|inactive|blocking|discarding|deny|denied|refused|failed|failure|errors?|unreachable|fault') + ')' + E,
        green: '(?<=' + LINE + ' *)((?:description|hostname|host-name|name|alias|version|set alias|set description) [^' + LINE + ']+)|' + B + '(' + words('up|connected|enabled|active|established|forwarding|full|permit|yes|ok|success(?:ful)?') + ')' + E,
        yellow: '(?<=[^0-9A-Za-z_&.:/-])(' + IPV4 + '|' + MAC + '|vlan ?[0-9]+|(?:gigabitethernet|tengigabitethernet|fastethernet|ethernet|hundredgige|fortygigabitethernet|twentyfivegige|port-channel|loopback|tunnel|gi|te|fa|eth|et|po|lo|tu)[0-9]+(?:/[0-9]+)*(?:\\.[0-9]+)?|(?:ge|xe|et|fe)-[0-9]+/[0-9]+/[0-9]+(?:\\.[0-9]+)?|ae[0-9]+|wan[0-9]+|port[0-9]+)(?=[^0-9A-Za-z_-])',
        blue: '(?<=' + LINE + ' *)([!#][^' + LINE + ']*)',
        magenta: '(?<=' + LINE + ' *)(' + words('interface|router|vlan|line|ip access-list|ipv6 access-list|access-list|policy-map|class-map|class|route-map|crypto map|crypto isakmp|aaa|vrf definition|vrf|object-group|object|config|edit|next|end|zone|security-zone|firewall') + ')(?=[\\s' + LINE + '])',
        cyan: B + '(' + words('show|ip address|ipv6 address|ip route|ipv6 route|switchport|spanning-tree|snmp-server|logging|ntp server|ip helper-address|access-group|access-class|mtu|speed|duplex|encapsulation|channel-group|standby|vrrp|monitor session|media-type|negotiation|transport input|username|password|secret|set|unset|commit|write memory|copy|reload|ping|traceroute') + ')' + E,
        underline: URL
      } },
    { id: 'perl', name: 'Perl syntax', regex: true, caseSensitive: true,
      notes: { red: 'Exits, file changes and modules', green: 'Comments', yellow: 'Operators and functions', blue: 'Pragmas', magenta: 'Control flow', cyan: 'Variables', underline: 'Web addresses' },
      rules: {
        red: B + '(' + words('die|exit|unlink|warn|croak|confess|rmdir|use [A-Za-z0-9_]+(?:::[A-Za-z0-9_]+)*|require') + ')' + E,
        green: '(?<=[' + LINE + ' \\t;])(#[^' + LINE + ']*)',
        yellow: '(=>|' + B + '(?:' + words('eq|ne|lt|gt|le|ge|cmp|not|and|or|xor|defined|undef|ref|bless|my|our|local|print|printf|sprintf|open|close|binmode|push|pop|shift|unshift|splice|split|join|grep|map|sort|keys|values|each|exists|delete|chomp|chop|lc|uc|lcfirst|ucfirst|length|substr|index|rindex|reverse|wantarray') + ')' + E + ')',
        blue: B + '(' + words('strict|warnings|utf8|vars|constant|lib|base|parent|fields|integer|overload') + ')' + E,
        magenta: B + '(' + words('if|elsif|else|unless|while|until|for|foreach|do|last|next|redo|return|goto|sub|package|BEGIN|END') + ')' + E,
        cyan: '([$@%](?:[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z0-9_]+)*|\\{[^}' + LINE + ']+\\})|\\$[0-9_&!@/\\\\])',
        underline: URL
      } },
    { id: 'sql', name: 'SQL syntax', regex: false, caseSensitive: false,
      notes: { red: 'Destructive commands and errors', green: 'Success and database names', yellow: 'Settings and constraints', magenta: 'Database objects', cyan: 'Query keywords' },
      rules: {
        red: 'drop,delete,truncate,abort,rollback,cascade,revoke,kill,shutdown,error,warning,fail,failed,denied',
        green: 'ok,success,successful,mysql,mariadb,postgresql,postgres,sqlite,oracle,sql server',
        yellow: 'lock,unlock,show,describe,desc,explain,use,set,grant,help,null,not null,default,auto_increment,unique,check,timeout,quit,exit',
        blue: '',
        magenta: 'table,tables,database,databases,schema,index,indexes,view,trigger,procedure,function,sequence,column,engine,charset,collation,rowid',
        cyan: 'select,from,where,insert,into,values,update,create,alter,join,inner join,left join,right join,full join,outer,cross join,on,group by,order by,having,limit,offset,union,distinct,as,and,or,not,in,between,like,is,case,when,then,else,end,begin,commit,transaction,primary key,foreign key,references,exists,with,returning',
        underline: ''
      } }
  ]);

  // POSIX character classes inside bracket expressions.
  const POSIX = { alpha: 'A-Za-z', digit: '0-9', alnum: 'A-Za-z0-9', upper: 'A-Z', lower: 'a-z', space: ' \\t\\r\\n\\f\\v', blank: ' \\t', punct: '!-\\/:-@\\[-`{-~', xdigit: '0-9A-Fa-f', word: 'A-Za-z0-9_', cntrl: '\\x00-\\x1f\\x7f', print: ' -~', graph: '!-~' };
  const CLASS_ESCAPES = 'dDsSwWtnrfv';

  // Convert a MobaXterm (POSIX bracket) pattern to JavaScript source.
  function convert(pattern) {
    let source = String(pattern), caseless = false;
    const inline = source.match(/^\(\?i\)/);
    if (inline) { caseless = true; source = source.slice(inline[0].length); }
    let out = '';
    for (let i = 0; i < source.length;) {
      const c = source[i];
      if (c === '\\') {
        const next = source[i + 1];
        if (next === undefined) { out += '\\\\'; i++; continue; }
        if (next === '<' || next === '>') { out += '\\b'; i += 2; continue; }
        out += c + next; i += 2; continue;
      }
      if (c === '[') {
        let j = i + 1, negate = false, items = '';
        if (source[j] === '^') { negate = true; j++; }
        if (source[j] === ']') { items += '\\]'; j++; }
        let closed = false;
        while (j < source.length) {
          const d = source[j];
          if (d === ']') { closed = true; break; }
          if (d === '[' && source[j + 1] === ':') {
            const end = source.indexOf(':]', j + 2), name = end > 0 ? source.slice(j + 2, end) : '';
            if (Object.hasOwn(POSIX, name)) { items += POSIX[name]; j = end + 2; continue; }
          }
          if (d === '\\') {
            const next = source[j + 1];
            if (next && CLASS_ESCAPES.includes(next)) { items += '\\' + next; j += 2; continue; }
            items += '\\\\'; j++; continue;
          }
          items += d === '[' || d === '/' ? '\\' + d : d;
          j++;
        }
        if (!closed) { out += '\\['; i++; continue; }
        out += '[' + (negate ? '^' : '') + items + ']';
        i = j + 1;
        continue;
      }
      out += c; i++;
    }
    return { source: out, caseless };
  }

  function escapeWord(word) { return word.replace(/[\\^$.*+?()[\]{}|/]/g, '\\$&'); }
  function wordSource(list) {
    const terms = [...new Set(String(list).split(',').map(word => word.trim()).filter(Boolean))].slice(0, LIMITS.words)
      .sort((a, b) => b.length - a.length).map(word => escapeWord(word).replace(/ +/g, '\\s+'));
    return terms.length ? '(?<![A-Za-z0-9_])(' + terms.join('|') + ')(?![A-Za-z0-9_])' : '';
  }

  function cleanText(value, max) { return typeof value === 'string' ? value.replace(/[\u0000-\u0008\u000b-\u001f\u007f]/g, '').slice(0, max) : ''; }

  // A safe, complete rule set object (custom rule sets from saved state or import).
  function normalize(value) {
    const v = value && typeof value === 'object' && !Array.isArray(value) ? value : {};
    const id = typeof v.id === 'string' && /^[a-z0-9][a-z0-9-]{0,39}$/.test(v.id) ? v.id : '';
    const rules = {};
    for (const color of COLORS) rules[color] = cleanText(v.rules?.[color], LIMITS.pattern).replace(/[\r\n]+/g, ' ');
    return { id, name: cleanText(v.name, LIMITS.name).trim() || 'Custom syntax', regex: v.regex !== false, caseSensitive: v.caseSensitive === true, rules };
  }

  // Compile one color's pattern; returns {regex} or {error}.
  function compileColor(text, regexMode, caseSensitive) {
    const value = String(text || '').trim();
    if (!value) return { regex: null };
    if (value.length > LIMITS.pattern) return { error: 'This pattern is longer than ' + LIMITS.pattern + ' characters.' };
    let source = '', flags = 'gd' + (caseSensitive ? '' : 'i');
    if (regexMode) { const converted = convert(value); source = converted.source; if (converted.caseless && !flags.includes('i')) flags += 'i'; }
    else source = wordSource(value);
    if (!source) return { regex: null };
    let regex;
    try { regex = new RegExp(source, flags); } catch (error) { return { error: 'This pattern is not a valid regular expression: ' + error.message }; }
    const slow = probe(regex, value);
    if (slow) return { error: slow };
    return { regex };
  }

  // Reject patterns whose running time grows explosively; they would freeze
  // the terminal while it repaints. Short probes keep the check itself quick.
  function probe(regex, pattern) {
    const now = () => global.performance?.now ? global.performance.now() : Date.now();
    const samples = [n => 'a'.repeat(n) + '!', n => ' '.repeat(n) + '!', n => 'a1.-_/: '.repeat(Math.ceil(n / 8)), n => '#'.repeat(n), n => '10.0.0.1 '.repeat(Math.ceil(n / 9)), n => '$('.repeat(Math.ceil(n / 2))];
    // Runs of the pattern's own letters and digits expose nested repetition
    // such as (x+x+)+y that generic text does not.
    for (const ch of [...new Set(String(pattern).replace(/\\./g, '').match(/[A-Za-z0-9]/g) || [])].slice(0, 16)) samples.push(n => ch.repeat(n) + '!');
    for (const sample of samples) {
      // Short runs first: exponential patterns show up within a few dozen
      // characters, and the probe stops at the first slow run.
      for (const length of [6, 10, 14, 17, 20, 23, 26, 32, 48, 96, 192, 384]) {
        const text = LINE + sample(length) + LINE, start = now();
        regex.lastIndex = 0; let count = 0, match;
        while ((match = regex.exec(text)) && count++ < 512) { if (match[0] === '') regex.lastIndex++; }
        regex.lastIndex = 0;
        if (now() - start > LIMITS.probeMs) return 'This pattern is too slow to run on every line. Simplify repeated groups such as (a+)+.';
      }
    }
    return '';
  }

  const cache = new Map();
  // compile(ruleSet) -> {match(text) -> hits, errors, id, key}
  function compile(value) {
    const set = normalize(value), key = JSON.stringify(set);
    if (cache.has(key)) return cache.get(key);
    const compiled = {}, errors = {};
    for (const color of COLORS) {
      const result = compileColor(set.rules[color], set.regex, set.caseSensitive);
      if (result.error) errors[color] = result.error; else if (result.regex) compiled[color] = result.regex;
    }
    const matcher = { key, id: set.id, name: set.name, errors, colors: Object.keys(compiled), match: text => match(compiled, text) };
    if (cache.size > 64) cache.clear();
    cache.set(key, matcher);
    return matcher;
  }

  function match(compiled, input) {
    const text = typeof input === 'string' ? input : '';
    if (!text || text.length > LIMITS.text) return [];
    const framed = LINE + text + LINE, taken = [], hits = [];
    for (const color of COLORS) {
      const regex = compiled[color];
      if (!regex) continue;
      regex.lastIndex = 0;
      let found = 0, m;
      while ((m = regex.exec(framed)) && found < LIMITS.hitsPerColor) {
        if (m[0] === '') { regex.lastIndex++; continue; }
        // The first capture group that matched when the rest of the match is
        // only delimiters (MobaXterm's "[^A-Za-z](word)[^A-Za-z]" style);
        // otherwise the whole match, so "fail(ed)?" colors all of "failed".
        let span = m.indices[0];
        for (let g = 1; g < m.indices.length; g++) {
          if (!m.indices[g]) continue;
          const outside = framed.slice(m.indices[0][0], m.indices[g][0]) + framed.slice(m.indices[g][1], m.indices[0][1]);
          if (!/[\p{L}\p{N}]/u.test(outside)) span = m.indices[g];
          break;
        }
        let start = span[0] - 1, end = span[1] - 1;
        // The line markers and surrounding blanks are never colored.
        start = Math.max(0, start); end = Math.min(text.length, end);
        while (start < end && /\s/.test(text[start])) start++;
        while (end > start && /\s/.test(text[end - 1])) end--;
        if (end <= start) continue;
        found++;
        if (color !== 'underline') {
          if (taken.some(hit => start < hit.end && end > hit.start)) continue;
          taken.push({ start, end });
        }
        hits.push({ start, end, kind: 'syn-' + color });
      }
      regex.lastIndex = 0;
    }
    return hits.sort((a, b) => a.start - b.start || a.end - b.end);
  }

  // ----- .msyn files -------------------------------------------------------
  // Text decoding: UTF-8 when valid, otherwise Windows-1252 (MobaXterm's).
  function decode(bytes) {
    const data = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes || []);
    try { return new TextDecoder('utf-8', { fatal: true }).decode(data).replace(/^﻿/, ''); } catch (_) {}
    try { return new TextDecoder('windows-1252').decode(data); } catch (_) {}
    return Array.from(data, byte => String.fromCharCode(byte)).join('');
  }
  const CP1252 = { 0x20ac: 0x80, 0x201a: 0x82, 0x0192: 0x83, 0x201e: 0x84, 0x2026: 0x85, 0x2020: 0x86, 0x2021: 0x87, 0x02c6: 0x88, 0x2030: 0x89, 0x0160: 0x8a, 0x2039: 0x8b, 0x0152: 0x8c, 0x017d: 0x8e, 0x2018: 0x91, 0x2019: 0x92, 0x201c: 0x93, 0x201d: 0x94, 0x2022: 0x95, 0x2013: 0x96, 0x2014: 0x97, 0x02dc: 0x98, 0x2122: 0x99, 0x0161: 0x9a, 0x203a: 0x9b, 0x0153: 0x9c, 0x017e: 0x9e, 0x0178: 0x9f };
  function encode(text) {
    return Uint8Array.from(Array.from(String(text)), ch => { const code = ch.codePointAt(0); return code < 0x80 || code >= 0xa0 && code <= 0xff ? code : CP1252[code] || 0x3f; });
  }

  function parseMsyn(input) {
    const text = typeof input === 'string' ? input : decode(input), fields = {};
    let section = false;
    for (const raw of text.replace(/\r\n?/g, '\n').split('\n')) {
      const line = raw.trim();
      if (!line) continue;
      if (/^\[.*\]$/.test(line)) { section = /^\[customsyntax\]$/i.test(line); continue; }
      if (!section) continue;
      const at = raw.indexOf('=');
      if (at > 0) fields[raw.slice(0, at).trim().toLowerCase()] = raw.slice(at + 1).replace(/[\r\n]+$/, '');
    }
    if (!Object.keys(fields).length) throw new Error('This is not a MobaXterm syntax file ([CustomSyntax] section not found).');
    const rules = {};
    for (const color of COLORS) rules[color] = fields[color] || '';
    return normalize({ name: fields.name || 'Imported syntax', regex: fields.useregex !== '0', caseSensitive: fields.casesensitive === '1', rules });
  }

  function toMsyn(value) {
    const set = normalize(value), line = (key, v) => key + '=' + v;
    return ['[CustomSyntax]', line('Name', set.name), line('UseRegex', set.regex ? 1 : 0), line('Underline', set.rules.underline), line('Red', set.rules.red), line('Green', set.rules.green), line('Yellow', set.rules.yellow), line('Blue', set.rules.blue), line('Magenta', set.rules.magenta), line('Cyan', set.rules.cyan), line('Blinking', ''), line('CaseSensitive', set.caseSensitive ? 1 : 0), '', ''].join('\r\n');
  }

  // ----- Saved preferences ---------------------------------------------------
  function builtIn(id) { return BUILT_IN.find(set => set.id === id) || null; }
  function preferences(value) {
    const v = value && typeof value === 'object' && !Array.isArray(value) ? value : {};
    const custom = [], seen = new Set();
    for (const item of Array.isArray(v.custom) ? v.custom : []) {
      const set = normalize(item);
      if (!set.id || seen.has(set.id) || builtIn(set.id) || custom.length >= LIMITS.custom) continue;
      seen.add(set.id); custom.push(set);
    }
    const active = typeof v.active === 'string' && (v.active === 'smart' || builtIn(v.active) || custom.some(set => set.id === v.active)) ? v.active : 'smart';
    // Per-profile color overrides (v1.21.12): {profile id: {red: '#rrggbb', ...}}.
    const colors = {}, requested = v.colors && typeof v.colors === 'object' && !Array.isArray(v.colors) ? v.colors : {};
    for (const [id, slots] of Object.entries(requested)) {
      if (!(builtIn(id) || custom.some(set => set.id === id)) || !slots || typeof slots !== 'object') continue;
      const clean = {};
      for (const color of COLORS) if (typeof slots[color] === 'string' && /^#[0-9a-f]{6}$/i.test(slots[color])) clean[color] = slots[color].toLowerCase();
      if (Object.keys(clean).length) colors[id] = clean;
    }
    return { active, custom, colors };
  }
  function find(prefs, id) { const clean = preferences(prefs); return builtIn(id) || clean.custom.find(set => set.id === id) || null; }
  function newId(prefs, name) {
    const clean = preferences(prefs), base = String(name || 'custom').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 30) || 'custom';
    let id = 'custom-' + base, n = 2;
    while (builtIn(id) || clean.custom.some(set => set.id === id)) id = 'custom-' + base.slice(0, 26) + '-' + n++;
    return id.slice(0, 40);
  }
  // Built-in color notes, used by the legend; custom sets list their colors.
  function legend(set) {
    const value = builtIn(set?.id) || set;
    return COLORS.filter(color => value?.rules?.[color]).map(color => ({ color, label: LABELS[color], note: value.notes?.[color] || '' }));
  }

  const api = { COLORS, LABELS, LIMITS, BUILT_IN, convert, compile, compileColor, match, parseMsyn, toMsyn, decode, encode, normalize, preferences, find, builtIn, newId, legend };
  if (typeof module === 'object' && module.exports) module.exports = api;
  global.NTKTerminalSyntaxRules = Object.freeze(api);
})(typeof window === 'object' ? window : globalThis);
