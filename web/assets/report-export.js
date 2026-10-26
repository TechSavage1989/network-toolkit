/* Network Toolkit report rendering. Original project code, no runtime dependencies. */
(function (root, factory) {
  'use strict';
  const api = factory(typeof module === 'object' && module.exports ? require('./source-interface.js') : root.NetworkToolkitSource, typeof module === 'object' && module.exports ? require('./result-time.js') : root.NetworkToolkitResultTime);
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.NetworkToolkitReportExport = api;
})(typeof globalThis === 'object' ? globalThis : this, function (sourceInfo, resultTime) {
  'use strict';

  const toolNames = Object.freeze({
    quick: 'Device check', ping: 'Ping', trace: 'Trace route', dns: 'DNS lookup',
    tcp: 'TCP ports', http: 'HTTP check', tls: 'TLS certificate',
    discover: 'Network discovery', monitor: 'Monitor', identify: 'Device details',
    routes: 'Routes', arp: 'ARP / neighbors', connections: 'Connections',
    wifi: 'Wi-Fi', ipconfig: 'IP configuration', wol: 'Wake-on-LAN'
  });
  const labels = Object.freeze({
    time: 'Recorded time (local)', timestamp: 'Recorded time (local)', captured: 'Snapshot recorded (local)', address: 'IP address', hostname: 'Hostname', hostnames: 'Hostname aliases',
    hostnameSource: 'Hostname source', hostnameStatus: 'Hostname lookup status',
    hostnameNote: 'Hostname lookup note', mac: 'MAC address', vendor: 'Vendor',
    rttMs: 'RTT (ms)', latencyMs: 'Latency (ms)', elapsedMs: 'Elapsed (ms)',
    avgMs: 'Average RTT (ms)', minMs: 'Minimum RTT (ms)', maxMs: 'Maximum RTT (ms)',
    jitterMs: 'Jitter (ms)', lossPercent: 'Packet loss (%)', ttl: 'TTL',
    icmpError: 'ICMP error', identityResponded: 'Device identity response',
    cidr: 'CIDR', mtu: 'MTU', pid: 'Process ID', url: 'URL',
    dnsServers: 'DNS servers', interfaceIndex: 'Interface index',
    completedProbes: 'Completed probes', requestedProbes: 'Requested probes',
    notStarted: 'Not started', openPorts: 'Open TCP ports',
    portsRequested: 'Requested TCP ports', scopes: 'Scan scopes', localContext: 'Local network context',
    hosts: 'Device results', samples: 'Probe samples', statistics: 'Statistics',
    records: 'DNS records', hops: 'Route hops', checks: 'Checks',
    descriptionUrl: 'Device description URL', modelSource: 'Model source',
    source: 'Recorded diagnostic source', sourceInterfaceIndex: 'Requested source interface index',
    sourceInterfaceName: 'Requested source interface name', sourceAddress: 'Requested source IP',
    interfaceDescription: 'Adapter description', interfaceName: 'Adapter name',
    nameResolution: 'Name resolution', dnsServer: 'DNS server', resolveHostnames: 'Resolve hostnames'
  });
  const preferredColumns = [
    'time', 'timestamp', 'address', 'hostname', 'name', 'hop', 'sequence', 'port', 'type',
    'status', 'state', 'success', 'value', 'detail', 'rttMs', 'latencyMs',
    'ttl', 'service', 'mac', 'vendor', 'destination', 'gateway', 'interface'
  ];
  const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[c]));
  const record = value => value !== null && typeof value === 'object' && !Array.isArray(value);
  const primitive = value => value === null || typeof value !== 'object';
  const simple = value => primitive(value) || Array.isArray(value) && value.every(primitive);
  const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
  const title = key => (resultTime?.isKey(key) ? (key==='captured'?'Snapshot recorded':'Recorded time')+' ('+(resultTime.zoneLabel?.()||'local')+')' : labels[key]) || String(key).replace(/([a-z\d])([A-Z])/g, '$1 $2')
    .replace(/[_-]+/g, ' ').replace(/^./, c => c.toUpperCase())
    .replace(/\bDns\b/g, 'DNS').replace(/\bTcp\b/g, 'TCP').replace(/\bIp\b/g, 'IP');
  const statusText = value => ({
    no_response: 'No response', completed: 'Completed', cancelled: 'Cancelled',
    running: 'Running', queued: 'Queued', failed: 'Failed', error: 'Error',
    success: 'Success', responded: 'Responded', timed_out: 'Timed out',
    timeout: 'Timeout', open: 'Open', closed: 'Closed', refused: 'Refused'
  })[value] || title(value || 'Not reported');

  function scalar(value, key) {
    if (resultTime?.isKey(key)) return timestamp(value);
    if (value === null || value === undefined) return 'Not reported';
    if (typeof value === 'boolean') return value ? 'Yes' : 'No';
    if (typeof value === 'number') {
      if (!Number.isFinite(value)) return 'Not reported';
      // Human-readable measurements must not expose binary floating-point
      // tails. Keep small nonzero measurements visible and leave counts intact.
      if (!Number.isInteger(value) && /(?:Ms|Percent|Pct)$/i.test(key || '')) {
        return String(Number(Math.abs(value) < 0.001 ? value.toPrecision(3) : value.toFixed(3)));
      }
      return String(value);
    }
    if (Array.isArray(value)) return value.length ? value.map(v => scalar(v, key)).join(', ') : 'None recorded';
    if (key === 'status' || key === 'state') return statusText(String(value));
    return String(value) || 'Not reported';
  }

  function timestamp(value) {
    if (resultTime) { const local=resultTime.format(value);return Number.isFinite(resultTime.epoch(value))?local+' ('+String(value)+')':local; }
    if (!value) return 'Not recorded';
    const date = new Date(value);
    return Number.isFinite(date.getTime()) ? date.toISOString().replace('T', ' ').replace('Z', ' UTC') : String(value);
  }

  function duration(job, exported) {
    const start = Date.parse(job.started), end = Date.parse(job.finished || exported);
    if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return 'Not reported';
    const seconds = (end - start) / 1000;
    return seconds < 60 ? seconds.toFixed(1) + ' s' : Math.floor(seconds / 60) + ' min ' + Math.floor(seconds % 60) + ' s';
  }

  function reportValue(input) {
    const value = record(input) ? input : {};
    return { ...value, jobs: Array.isArray(value.jobs) ? value.jobs.filter(record) : [],
      system: record(value.system) ? value.system : {} };
  }

  function networkScope(job) {
    return job.tool === 'discover' || job.result?.data?.scope === 'subnet' || job.options?.scope === 'subnet' ||
      (Array.isArray(job.events) && job.events.some(event => event?.data?.type === 'subnet-host'));
  }

  function jobName(job) {
    if (job.tool === 'quick' && networkScope(job)) return 'Network check (previous version)';
    return toolNames[job.tool] || title(job.tool || 'Diagnostic');
  }

  function json(value) {
    try { return JSON.stringify(value, null, 2); } catch { return 'This value cannot be represented as JSON.'; }
  }

  function badge(value) {
    const status = String(value || 'unknown').toLowerCase();
    const tone = ['completed', 'success', 'responded', 'open', 'ok'].includes(status) ? 'good'
      : ['failed', 'error'].includes(status) ? 'bad'
      : ['running', 'queued', 'cancelled', 'warning', 'timeout', 'no_response'].includes(status) ? 'warn' : 'neutral';
    return '<span class="status ' + tone + '">' + esc(statusText(status)) + '</span>';
  }

  function fields(entries) {
    return '<dl class="fields">' + entries.map(([key, value]) => '<div class="field"><dt>' + esc(key) +
      '</dt><dd>' + esc(value) + '</dd></div>').join('') + '</dl>';
  }

  function table(headers, rows, classes) {
    return tableBody(headers, rows.map(row => '<tr>' + row.map(cell => '<td>' + cell + '</td>').join('') + '</tr>').join(''), classes);
  }

  function tableBody(headers, body, classes) {
    return '<div class="table-scroll" tabindex="0" role="region" aria-label="' + esc(headers[0] + ' table') +
      '"><table' + (classes ? ' class="' + esc(classes) + '"' : '') + '><thead><tr>' +
      headers.map(h => '<th scope="col">' + esc(h) + '</th>').join('') + '</tr></thead><tbody>' +
      body +
      '</tbody></table></div>';
  }

  function renderValue(value, depth) {
    depth = depth || 0;
    if (value === undefined || value === null) return '<p class="muted">Not reported.</p>';
    if (depth > 10) return '<pre class="raw">' + esc(json(value)) + '</pre>';
    if (primitive(value)) return '<p class="value">' + esc(scalar(value)) + '</p>';
    if (Array.isArray(value)) {
      if (!value.length) return '<p class="muted">No entries recorded.</p>';
      if (value.every(primitive)) return '<ul class="value-list">' + value.map(v => '<li>' + esc(scalar(v)) + '</li>').join('') + '</ul>';
      if (!value.every(record)) return '<ol class="record-list">' + value.map(v => '<li>' + renderValue(v, depth + 1) + '</li>').join('') + '</ol>';
      const keys = [...new Set(value.flatMap(row => Object.keys(row)))];
      const simpleKeys = keys.filter(k => value.every(row => !own(row, k) || simple(row[k])));
      const ordered = [...preferredColumns.filter(k => simpleKeys.includes(k)), ...simpleKeys.filter(k => !preferredColumns.includes(k))];
      const columns = ordered.slice(0, 6);
      if (!columns.length) return '<ol class="record-list">' + value.map(row => '<li>' + renderValue(row, depth + 1) + '</li>').join('') + '</ol>';
      const rows = value.map((row, index) => {
        const cells = columns.map(key => key === 'status' || key === 'state' ? badge(row[key]) : esc(scalar(row[key], key)));
        const extra = Object.fromEntries(Object.entries(row).filter(([key]) => !columns.includes(key)));
        return '<tr>' + cells.map(cell => '<td>' + cell + '</td>').join('') + '</tr>' +
          (Object.keys(extra).length ? '<tr class="expanded-record"><td colspan="' + columns.length +
            '"><details class="row-details"><summary>Additional details for entry ' + (index + 1) +
            '</summary><div class="record-detail-body">' + renderValue(extra, depth + 1) + '</div></details></td></tr>' : '');
      });
      return tableBody(columns.map(title), rows.join(''));
    }
    const entries = Object.entries(value);
    if (!entries.length) return '<p class="muted">No fields recorded.</p>';
    const ordinary = entries.filter(([, v]) => primitive(v) && !(typeof v === 'string' && (v.includes('\n') || v.length > 360)));
    const nested = entries.filter(([key]) => !ordinary.some(([k]) => k === key));
    return (ordinary.length ? fields(ordinary.map(([key, v]) => [title(key), scalar(v, key)])) : '') +
      nested.map(([key, v]) => '<section class="data-group"><h4>' + esc(title(key)) + '</h4>' +
        (typeof v === 'string' ? '<pre class="raw">' + esc(v) + '</pre>' : renderValue(v, depth + 1)) + '</section>').join('');
  }

  function networkData(job) {
    const data = record(job.result?.data) ? job.result.data : {};
    const events = Array.isArray(job.events) ? job.events.filter(record) : [];
    if (!networkScope(job)) return null;
    let hosts;
    if (Array.isArray(data.hosts)) hosts = data.hosts.filter(record);
    else {
      const byAddress = new Map();
      // Each update is a complete host snapshot. Omitted identity fields may
      // have been deliberately cleared after conflicting neighbor observations.
      const add = host => { if (record(host) && typeof host.address === 'string') byAddress.set(host.address, { ...host }); };
      for (const event of events) if (['host', 'subnet-host'].includes(event.data?.type)) add(event.data.value);
      if (Array.isArray(job.liveHosts)) job.liveHosts.forEach(add);
      else if (record(job.liveHosts)) Object.values(job.liveHosts).forEach(add);
      hosts = [...byAddress.values()];
    }
    const ipNumber = value => {
      const parts = String(value || '').split('.');
      return parts.length === 4 && parts.every(p => /^\d{1,3}$/.test(p) && Number(p) <= 255)
        ? parts.reduce((n, p) => n * 256 + Number(p), 0) : null;
    };
    hosts = hosts.map((row, index) => ({ row, index })).sort((a, b) => {
      const x = ipNumber(a.row.address), y = ipNumber(b.row.address);
      return x !== null && y !== null ? x - y || a.index - b.index
        : String(a.row.address || '').localeCompare(String(b.row.address || ''), undefined, { numeric: true });
    }).map(item => item.row);
    const lastProgress = events.filter(event => ['progress', 'subnet-progress'].includes(event.data?.type) && record(event.data.value)).at(-1)?.data.value || {};
    return { data: { ...lastProgress, ...data }, hosts };
  }

  function hostName(host) {
    return host.hostname || (Array.isArray(host.hostnames) ? host.hostnames[0] : '') || 'Not resolved';
  }

  function hostOutcome(host) {
    if (host.responded === true) return 'Responded';
    if (host.status) return statusText(host.status);
    if (host.responded === false) return 'No response';
    return Array.isArray(host.evidence) && host.evidence.length ? 'Response evidence recorded' : 'Not reported';
  }

  function renderNetwork(snapshot) {
    const hosts = snapshot.hosts, data = snapshot.data;
    const rows = hosts.map(host => {
      const cells = [
      '<strong class="mono">' + esc(host.address || 'Not reported') + '</strong>' +
        '<span class="cell-sub">' + esc(hostName(host)) + '</span>' +
        '<span class="cell-sub">' + esc(resultTime?.columnLabel?.()||'Recorded (local)') + ': ' + esc(timestamp(host.time)) + '</span>',
      esc(hostOutcome(host)) + (host.status === 'cancelled' && host.responded ? '<span class="cell-sub">Cancelled after response</span>' : ''),
      esc(scalar(host.mac)) + '<span class="cell-sub">' + esc(scalar(host.vendor)) + '</span>',
      esc(host.latencyMs == null ? 'Not measured' : scalar(host.latencyMs, 'latencyMs') + ' ms'),
      esc(scalar(host.openPorts))
      ];
      return '<tr data-device-row><td>' + cells.join('</td><td>') + '</td></tr>' +
        '<tr class="expanded-record"><td colspan="5"><details class="row-details"><summary>Device details for ' +
        esc(host.address || 'this entry') + '</summary><div class="record-detail-body">' + renderValue(host, 1) + '</div></details></td></tr>';
    });
    const meta = Object.fromEntries(Object.entries(data).filter(([key]) => key !== 'hosts'));
    return '<h3>Device results <span class="count">' + hosts.length.toLocaleString('en-US') + ' recorded</span></h3>' +
      '<p class="section-note">All recorded device rows are included, regardless of the app list filters or page. An unresolved hostname does not establish whether a device responded.</p>' +
      (hosts.length ? tableBody(['IP address / hostname', 'Outcome', 'MAC address / vendor', 'Latency', 'Open TCP ports'], rows.join(''), 'network-table')
        : '<p class="muted">No device rows were recorded at export.</p>') +
      (Object.keys(meta).length ? '<details class="supplement"><summary>Scan settings, counts and context</summary>' + renderValue(meta, 0) + '</details>' : '');
  }

  function jobSummary(job) {
    return job.error || job.result?.summary || (['running', 'queued'].includes(job.status)
      ? 'This diagnostic was still running when the report was exported.' : 'No summary was recorded.');
  }

  function recordedData(job) {
    if (job.result?.data !== undefined) return job.result.data;
    if (!['ping','monitor'].includes(job.tool) || !record(job.sampleStatistics)) return undefined;
    const samples=(Array.isArray(job.events)?job.events:[]).filter(event=>event?.data?.type==='sample').map(event=>event.data.value);
    return {statistics:job.sampleStatistics,samples,note:'Statistics cover the complete run. This live snapshot contains the latest '+samples.length+' retained sample rows.'};
  }

  function activityHTML(job) {
    const events = Array.isArray(job.events) ? job.events.filter(record) : [];
    if (!events.length) return '';
    return '<details class="supplement activity"><summary>Recorded activity (' + events.length.toLocaleString('en-US') + ' events)</summary>' +
      '<ol class="activity-list">' + events.map(event => '<li><div class="event-heading"><time>' + esc(timestamp(event.time)) +
        '</time>' + badge(event.level || 'info') + '</div><p>' + esc(event.message || '') + '</p>' +
        (event.data !== undefined ? '<details class="event-data"><summary>Event details</summary>' + renderValue(event.data, 0) + '</details>' : '') +
        '</li>').join('') + '</ol></details>';
  }

  const stylesheet = `
    :root{color-scheme:light;font-family:"Segoe UI",Arial,sans-serif;color:#182b43;background:#eef2f6;font-size:14px;line-height:1.55}
    *{box-sizing:border-box}body{margin:0}main{width:min(1280px,100%);margin:auto;padding:32px 28px 48px}
    h1,h2,h3,h4,p{margin:0}h1{font-size:30px;line-height:1.2;letter-spacing:-.5px;margin:8px 0 12px}h2{font-size:21px;line-height:1.3}h3{font-size:17px;margin:22px 0 10px}h4{font-size:14px;margin:14px 0 7px}
    a{color:#1558a4;text-decoration-thickness:1px;text-underline-offset:3px}a:focus-visible,summary:focus-visible,.table-scroll:focus-visible{outline:3px solid #559be7;outline-offset:3px}
    .report-header{border-top:6px solid #2268ae;background:#fff;padding:28px;border-radius:9px;margin-bottom:22px;box-shadow:0 2px 7px #17304d09}
    .eyebrow{text-transform:uppercase;letter-spacing:1.5px;font-size:11px;font-weight:700;color:#28669c}.meta,.section-note,.muted{color:#566a7f}.meta{font-size:13px}.meta p+p{margin-top:3px}
    .summary-cards{display:flex;flex-wrap:wrap;gap:12px;margin-top:20px}.summary-card{min-width:120px;padding:12px 16px;background:#f0f5fa;border:1px solid #d7e3f0;border-radius:7px}.summary-card strong{display:block;font-size:25px;line-height:1.2;color:#174f83}.summary-card span{display:block;font-size:12px;margin-top:4px}
    .report-notes{margin-top:20px;padding:14px 16px;border-left:4px solid #6594bf;background:#f5f8fb;white-space:pre-wrap;overflow-wrap:anywhere}.report-notes h2{font-size:14px;margin-bottom:6px}
    .section,.diagnostic{background:#fff;border:1px solid #d5dfe9;border-radius:9px;padding:23px;margin-top:20px;min-width:0}.section>h2{margin-bottom:14px}.diagnostic{scroll-margin-top:16px}
    .diagnostic-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:14px;border-bottom:1px solid #dce4ed;padding-bottom:15px;margin-bottom:15px}.diagnostic-heading h2{margin:3px 0 5px}.target{font-family:Consolas,"Courier New",monospace;font-size:14px;overflow-wrap:anywhere}.number{font-size:11px;font-weight:700;letter-spacing:.7px;text-transform:uppercase;color:#64778b}
    .result-summary{border-left:4px solid #87a9c9;padding:11px 14px;margin:15px 0;background:#f4f7fb;white-space:pre-wrap;overflow-wrap:anywhere}.section-note{font-size:12px;margin:8px 0 12px}.count{font-size:12px;color:#526a7e;font-weight:400;margin-left:6px}
    .fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));margin:0;border:1px solid #dbe3ec;border-radius:6px;overflow:hidden}.field{padding:10px 13px;min-width:0;border-bottom:1px solid #e4eaf1}.field:nth-child(odd){border-right:1px solid #e4eaf1}.field dt{font-size:11px;font-weight:600;color:#566d84;margin-bottom:4px}.field dd{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font-size:13px}.data-group{margin-top:15px}.value{white-space:pre-wrap;overflow-wrap:anywhere}
    .table-scroll{width:100%;overflow-x:auto;margin:9px 0 14px;border:1px solid #d6e0ea;border-radius:6px}table{border-collapse:collapse;width:100%;table-layout:fixed;font-size:12px;text-align:left}th,td{padding:10px 11px;vertical-align:top;border-bottom:1px solid #dce5ee;overflow-wrap:anywhere;word-break:normal}th{font-size:11px;font-weight:650;letter-spacing:.1px;background:#eaf0f6;color:#324e6c}tbody tr:nth-child(even){background:#f7f9fc}tbody tr:last-child td{border-bottom:0}td p{margin:0}td .fields{display:block}td .field{border-right:0!important;padding:7px}td .field dd{font-size:11px}td table{min-width:0;font-size:11px}td th,td td{padding:6px}
    .target,.summary-table td:nth-child(2){white-space:pre-wrap}.summary-table th:nth-child(1){width:18%}.summary-table th:nth-child(2){width:21%}.summary-table th:nth-child(3){width:13%}.summary-table th:nth-child(4){width:48%}.network-table th:nth-child(1){width:28%}.network-table th:nth-child(2){width:16%}.network-table th:nth-child(3){width:26%}.network-table th:nth-child(4){width:14%}.network-table th:nth-child(5){width:16%}
    .cell-sub{display:block;font-size:11px;color:#597087;line-height:1.5;margin-top:4px;overflow-wrap:anywhere}.mono{font-family:Consolas,"Courier New",monospace;font-weight:600}.status{display:inline-block;font-size:11px;font-weight:600;border:1px solid #ccd8e5;border-radius:5px;padding:3px 8px;white-space:normal}.status.good{background:#edf7f0;color:#24613c;border-color:#b8d6c3}.status.bad{background:#fff0ef;color:#912d29;border-color:#e4bbb8}.status.warn{background:#fff8e9;color:#805b11;border-color:#e4d2a7}.status.neutral{background:#edf2f7;color:#415b76}
    details{min-width:0}summary{cursor:pointer;font-weight:600;color:#285e92}.supplement{margin-top:17px;border:1px solid #d5e0ea;border-radius:6px;background:#fafcfe}.supplement>summary{padding:11px 14px;background:#f0f5fa;font-size:13px}.supplement[open]>.fields,.supplement[open]>.data-group,.supplement[open]>p,.supplement[open]>.table-scroll{margin:14px}.row-details summary{font-size:11px}.row-details[open]{min-width:0}.row-details[open]>div,.row-details[open]>dl{margin-top:9px}.row-details h4{font-size:11px}
    .expanded-record>td{padding-top:4px;background:#fafcfe;border-bottom:2px solid #d5dfe9}.expanded-record .record-detail-body{padding:7px 3px 10px}.expanded-record .fields{display:grid}.expanded-record .field dd{font-size:12px}.expanded-record .row-details>summary{font-size:12px}.expanded-record h4{font-size:13px}
    .raw{font:12px/1.6 Consolas,"Courier New",monospace;white-space:pre-wrap;overflow-wrap:anywhere;padding:13px;background:#f3f6fa;border:1px solid #dbe3ec;border-radius:5px;margin:8px 0}.value-list{padding-left:22px}.value-list li{padding:2px 0;white-space:pre-wrap;overflow-wrap:anywhere}.record-list{padding-left:25px}.record-list>li{padding:8px 0 15px;border-bottom:1px solid #dce5ee}
    .activity-list{padding:0 16px 0 37px;margin:0}.activity-list>li{padding:13px 0;border-bottom:1px solid #dce5ee}.activity-list>li:last-child{border:0}.activity-list p{white-space:pre-wrap;overflow-wrap:anywhere;margin-top:5px}.event-heading{display:flex;align-items:center;gap:10px;flex-wrap:wrap;font-size:11px;color:#53697f}.event-data{margin-top:8px;font-size:12px}.foot{font-size:11px;color:#526a7e;border-top:1px solid #cbd7e4;margin-top:27px;padding-top:16px}.foot p+p{margin-top:5px}.back-link{font-size:11px;margin-top:15px}
    @media(max-width:760px){main{padding:14px 10px 30px}.report-header,.section,.diagnostic{padding:16px}.fields{grid-template-columns:1fr}.field:nth-child(odd){border-right:0}h1{font-size:25px}.summary-card{min-width:90px;flex:1}.table-scroll>table{min-width:620px}.row-details .table-scroll>table{min-width:0}.diagnostic-heading{flex-wrap:wrap}}
    @media print{@page{margin:13mm}html,body{background:#fff!important;color:#111}main{width:100%;max-width:none;padding:0;margin:0;font-size:10pt}.report-header{padding:0 0 14px;box-shadow:none;border-radius:0}.section,.diagnostic{padding:14px 0;border:0;border-top:1px solid #b6c6d6;border-radius:0;margin-top:18px}.diagnostic-heading,h1,h2,h3,h4{break-after:avoid}.summary-card{padding:8px 12px}.table-scroll{overflow:visible;border-radius:0}table{min-width:0!important;font-size:8pt}th,td{padding:6px;overflow-wrap:anywhere}thead{display:table-header-group}tr{break-inside:avoid}.status,th,.summary-card{print-color-adjust:exact;-webkit-print-color-adjust:exact}.back-link{display:none}.raw{white-space:pre-wrap}.row-details[open]{display:block}a{color:#111;text-decoration:none}.supplement:not([open]){border-style:dashed}}
  `;

  function toHTML(input) {
    const report = reportValue(input), jobs = report.jobs;
    const heading = report.label || 'Network diagnostics report';
    const counts = { results: jobs.length, completed: jobs.filter(j => j.status === 'completed').length,
      running: jobs.filter(j => ['running', 'queued'].includes(j.status)).length,
      attention: jobs.filter(j => ['failed', 'error', 'cancelled'].includes(j.status)).length };
    const summary = jobs.length ? table(['Diagnostic', 'Target', 'Run status', 'Summary'], jobs.map((job, index) => [
      '<a href="#result-' + (index + 1) + '">' + esc(jobName(job)) + '</a>',
      esc(job.target || 'Local computer'), badge(job.status), esc(jobSummary(job))
    ]), 'summary-table') : '<p class="muted">No diagnostic results were selected.</p>';
    const context = Object.fromEntries(Object.entries(report.system).filter(([key]) => !['hostname', 'os', 'arch'].includes(key)));
    const body = jobs.map((job, index) => {
      const snapshot = networkData(job);
      return '<section class="diagnostic" id="result-' + (index + 1) + '"><header class="diagnostic-heading"><div><p class="number">Diagnostic ' +
        (index + 1) + ' of ' + jobs.length + '</p><h2>' + esc(jobName(job)) +
        '</h2><p class="target">' + esc(job.target || 'Local computer') + '</p></div>' + badge(job.status) + '</header>' +
        fields([['Started', timestamp(job.started)], ['Finished / snapshot', timestamp(job.finished || report.exported)],
          ['Duration', duration(job, report.exported)], ['Result status', statusText(job.status)]]) +
        (sourceInfo?.sourceFields(job).length ? '<section class="data-group"><h3>Diagnostic source</h3>' + fields(sourceInfo.sourceFields(job)) + '</section>' : '') +
        '<p class="result-summary">' + esc(jobSummary(job)) + '</p>' +
        (snapshot ? renderNetwork(snapshot) : '<h3>Recorded results</h3>' + (recordedData(job) !== undefined
          ? renderValue(recordedData(job), 0) : '<p class="muted">No structured result data was available. Any recorded activity appears below.</p>')) +
        (record(job.options) && Object.keys(job.options).length ? '<details class="supplement"><summary>Diagnostic options</summary>' + renderValue(job.options, 0) + '</details>' : '') +
        activityHTML(job) + '<p class="back-link"><a href="#result-summary">Back to result summary</a></p></section>';
    }).join('');
    return '<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' +
      '<meta name="color-scheme" content="light"><title>' + esc(heading) + '</title><style>' + stylesheet + '</style></head><body><main>' +
      '<header class="report-header"><p class="eyebrow">Network Toolkit report</p><h1>' + esc(heading) + '</h1><div class="meta"><p>Exported ' + esc(timestamp(report.exported)) +
      ' | Version ' + esc(report.version || 'Not reported') + '</p><p>Computer: ' + esc(report.system.hostname || 'Not reported') +
      (report.system.os ? ' | ' + esc(report.system.os) + (report.system.arch ? ' ' + esc(report.system.arch) : '') : '') +
      '</p><p>Times use ' + esc(timeContext()) + ', followed by the original recorded instant. This report contains saved observations from the selected diagnostic runs.</p></div>' +
      '<div class="summary-cards">' + [[counts.results, 'Selected results'], [counts.completed, 'Completed runs'], [counts.running, 'In-progress snapshots'], [counts.attention, 'Failed / cancelled runs']]
        .map(([count, name]) => '<div class="summary-card"><strong>' + count + '</strong><span>' + esc(name) + '</span></div>').join('') + '</div>' +
      (report.notes ? '<section class="report-notes"><h2>Report notes</h2><p>' + esc(report.notes) + '</p></section>' : '') + '</header>' +
      '<section class="section" id="result-summary"><h2>Result summary</h2>' + summary +
      '<p class="section-note">Run status describes completion of the diagnostic, not the health of every device or service. Select a diagnostic name to jump to its results.</p></section>' +
      body + (Object.keys(context).length ? '<details class="supplement system-context"><summary>Exporting computer: adapters and system context</summary>' + renderValue(context, 0) + '</details>' : '') +
      '<footer class="foot"><p>All recorded rows are exported. App list filters and pagination do not remove report entries. Expand a Details section to inspect additional recorded fields before printing.</p>' +
      '<p>Unanswered probes can result from filtering or rate limiting. Hostnames and neighbor information can be missing or stale. Conventional TCP service labels do not identify an application.</p></footer>' +
      '</main></body></html>';
  }

  function textValue(value, depth) {
    depth = depth || 0;
    const indent = '  '.repeat(Math.min(depth, 12));
    if (primitive(value) || value === undefined) return String(scalar(value)).split(/\r?\n/).map(line => indent + line).join('\n');
    if (depth > 10) return indent + json(value);
    if (Array.isArray(value)) {
      if (!value.length) return indent + 'None recorded';
      return value.map((item, index) => primitive(item) ? indent + (index + 1) + '. ' + scalar(item)
        : indent + (index + 1) + '.\n' + textValue(item, depth + 1)).join('\n');
    }
    return Object.entries(value).map(([key, item]) => simple(item)
      ? indent + title(key) + ': ' + scalar(item, key).replace(/\r?\n/g, '\n' + indent + '  ')
      : indent + title(key) + ':\n' + textValue(item, depth + 1)).join('\n') || indent + 'No fields recorded';
  }

  function timeContext() {
    const zone=resultTime?.zoneLabel?.()||'local',clock=resultTime?.preferences?.().hourFormat==='12'?'12-hour':'24-hour';
    return (zone==='local'?'the exporting computer local timezone':zone)+' with '+clock+' clocks';
  }

  function toText(input) {
    const report = reportValue(input);
    const lines = [report.label || 'Network diagnostics list', 'Network Toolkit ' + (report.version || ''),
      'Exported: ' + timestamp(report.exported), 'Computer: ' + (report.system.hostname || 'Not reported'),
      'Selected results: ' + report.jobs.length, 'Times use '+timeContext()+' with original recorded instants. All recorded rows are included.', ''];
    if (report.notes) lines.push('NOTES', String(report.notes), '');
    report.jobs.forEach((job, index) => {
      lines.push('='.repeat(64), (index + 1) + '. ' + jobName(job),
        'Target: ' + String(job.target || 'Local computer').replace(/\r?\n/g, '\n  '), 'Status: ' + statusText(job.status),
        'Started: ' + timestamp(job.started), 'Finished / snapshot: ' + timestamp(job.finished || report.exported),
        'Duration: ' + duration(job, report.exported), 'Summary: ' + jobSummary(job), '');
      const network = networkData(job);
      const sourceFields = sourceInfo?.sourceFields(job) || [];
      if (sourceFields.length) lines.push('DIAGNOSTIC SOURCE', ...sourceFields.map(([name, value]) => name + ': ' + String(value).replace(/\r?\n/g, '\n  ')), '');
      if (network) {
        lines.push('DEVICE LIST (' + network.hosts.length + ' recorded)');
        network.hosts.forEach((host, hostIndex) => {
          lines.push((hostIndex + 1) + '. ' + (host.address || 'Not reported') + ' | ' + hostName(host) + ' | ' + hostOutcome(host));
          const rest = Object.fromEntries(Object.entries(host).filter(([key]) => !['address', 'hostname'].includes(key)));
          if (Object.keys(rest).length) lines.push(textValue(rest, 1));
          lines.push('');
        });
        if (!network.hosts.length) lines.push('No device rows were recorded at export.', '');
        const meta = Object.fromEntries(Object.entries(network.data).filter(([key]) => key !== 'hosts'));
        if (Object.keys(meta).length) lines.push('SCAN COUNTS AND CONTEXT', textValue(meta, 0), '');
      } else lines.push('RESULTS', recordedData(job) === undefined ? 'No structured result data was recorded.' : textValue(recordedData(job), 0), '');
      if (record(job.options) && Object.keys(job.options).length) lines.push('DIAGNOSTIC OPTIONS', textValue(job.options, 0), '');
      const events = Array.isArray(job.events) ? job.events.filter(record) : [];
      if (events.length) {
        lines.push('ACTIVITY (' + events.length + ' recorded events)');
        events.forEach(event => {
          lines.push(timestamp(event.time) + ' [' + (event.level || 'info') + '] ' + (event.message || ''));
          if (event.data !== undefined) lines.push(textValue(event.data, 1));
        });
        lines.push('');
      }
    });
    const context = Object.fromEntries(Object.entries(report.system).filter(([key]) => key !== 'hostname'));
    if (Object.keys(context).length) lines.push('='.repeat(64), 'EXPORTING COMPUTER CONTEXT', textValue(context, 0), '');
    lines.push('End of exported list. A diagnostic snapshot does not establish current device health.');
    return lines.join('\n').replace(/\r\n?/g, '\n').replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g, '').replace(/\n/g, '\r\n') + '\r\n';
  }

  return Object.freeze({ toHTML, toText });
});
