'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const renderer = require('../web/assets/report-export.js');

function reportFixture(count = 3) {
  const hosts = Array.from({length: count}, (_, i) => ({
    address: '192.0.' + (2 + Math.floor(i / 250)) + '.' + (i % 250 + 1),
    hostname: 'switch-' + (i + 1) + '.fixture.example',
    hostnames: ['switch-' + (i + 1) + '.fixture.example', 'alias-' + (i + 1) + '.fixture.example'],
    hostnameStatus: 'resolved', hostnameSource: 'reverse lookup',
    mac: '02:00:00:00:' + Math.floor(i / 256).toString(16).padStart(2, '0') + ':' + (i % 256).toString(16).padStart(2, '0'),
    vendor: 'Example Network Equipment', manufacturer: 'Fixture Manufacturing',
    model: 'Access switch 48-port', modelSource: 'Device-reported UPnP',
    status: i % 3 === 2 ? 'no_response' : 'responded', responded: i % 3 !== 2,
    completed: true, latencyMs: i % 3 === 2 ? null : 0.42 + i / 10,
    openPorts: i % 3 === 2 ? [] : [22, 443], evidence: i % 3 === 2 ? [] : ['icmp', 'tcp:22'],
    ports: [{port: 22, state: 'open', service: 'ssh', elapsedMs: 0.7},
      {port: 443, state: 'open', service: 'https', elapsedMs: 1.2}]
  }));
  return {
    app: 'Network Toolkit', version: '1.10.2', exported: '2026-10-21T05:00:10Z',
    label: 'Branch network validation', notes: 'Check switching and name resolution before the maintenance window.\nRecorded test data only.',
    system: {hostname: 'ENGINEER-LAPTOP', os: 'windows', arch: 'amd64', interfaces: [
      {index: 7, name: 'Ethernet', up: true, addresses: ['192.0.2.50/24'], mac: '02:aa:bb:cc:dd:ee', mtu: 1500}
    ]},
    jobs: [
      {id: 'discovery-fixture', tool: 'discover', target: '192.0.2.0/23', status: 'completed',
        started: '2026-10-21T05:00:00Z', finished: '2026-10-21T05:00:04Z',
        options: {ports: '22,443', identify: true, resolveHostnames: true},
        result: {summary: count + ' addresses tested. Review recorded device outcomes below.', data: {
          scope: 'subnet', network: '192.0.2.0/23', total: count, completed: count,
          responded: hosts.filter(h => h.responded).length, notStarted: 0, hosts,
          note: 'A hostname may resolve even when a device does not answer a probe.'
        }}, events: [{time: '2026-10-21T05:00:04Z', level: 'info', message: 'Fixture scan finished.'}]},
      {id: 'ping-fixture', tool: 'ping', target: '192.0.2.1', status: 'completed',
        started: '2026-10-21T05:00:01Z', finished: '2026-10-21T05:00:05Z',
        result: {summary: '3 of 4 ICMP replies; 25% no response.', data: {
          target: '192.0.2.1', statistics: {sent: 4, received: 3, lost: 1, lossPercent: 25, minMs: 0.4, maxMs: 1.1, avgMs: 0.7},
          samples: [{sequence: 1, address: '192.0.2.1', success: true, rttMs: 0.4, ttl: 64, status: 'success'},
            {sequence: 2, address: '192.0.2.1', success: false, rttMs: 0, ttl: 0, status: 'timeout'}]
        }}},
      {id: 'dns-fixture', tool: 'dns', target: 'switch-1.fixture.example', status: 'completed',
        started: '2026-10-21T05:00:06Z', finished: '2026-10-21T05:00:07Z',
        result: {summary: 'Two records returned.', data: {query: 'switch-1.fixture.example', server: 'Configured DNS resolver',
          records: [{type: 'A', name: 'switch-1.fixture.example', value: '192.0.2.1', ttl: 300},
            {type: 'TXT', name: 'switch-1.fixture.example', value: 'ownership=fixture; ' + 'long-but-valid-text-'.repeat(20), ttl: 0}]}}}
    ]
  };
}

function deepFreeze(value) {
  if (value && typeof value === 'object') { Object.freeze(value); Object.values(value).forEach(deepFreeze); }
  return value;
}

function runTests() {
  let passed = 0;
  const check = (name, fn) => { fn(); passed++; process.stdout.write('PASS: ' + name + '\n'); };
  check('complete device lists exceed the former 200-row limit and retain detail fields', () => {
    const r = reportFixture(305), html = renderer.toHTML(r), txt = renderer.toText(r);
    assert.equal((html.match(/data-device-row/g) || []).length, 305);
    assert.match(html, /switch-305\.fixture\.example/);
    assert.match(txt, /305\. 192\.0\.3\.55 \| switch-305\.fixture\.example/);
    for (const marker of ['alias-305.fixture.example', 'Fixture Manufacturing', 'Device-reported UPnP']) {
      assert.ok(html.includes(marker)); assert.ok(txt.includes(marker));
    }
    assert.doesNotMatch(html, /first 200|data-list-query|data-list-page|\[object Object\]/);
    assert.match(html, /colspan="5"/);
  });
  check('imported labels, messages, field names and values cannot inject executable markup', () => {
    const r = reportFixture(1), attack = '<img src=x onerror=alert(1)><script>alert(1)</script>';
    r.label = attack; r.notes = attack; r.system.hostname = attack;
    r.jobs[0].result.data.hosts[0].hostname = attack;
    r.jobs[1].result.data[attack] = {nested: attack};
    r.jobs[0].events[0].message = attack;
    const html = renderer.toHTML(r), txt = renderer.toText(r);
    assert.doesNotMatch(html, /<script|<img/i);
    assert.ok(html.includes('&lt;script&gt;alert(1)&lt;/script&gt;'));
    assert.ok(txt.includes(attack));
    assert.doesNotMatch(html, /<link\b|<iframe\b|<script\b|\sonclick=/i);
  });
  check('running network snapshots retain early hosts and latest updates without changing the report', () => {
    const r = reportFixture(0), job = r.jobs[0];
    delete job.result; job.status = 'running'; delete job.finished;
    job.events = [{data: {type: 'host', value: {address: '192.0.2.2', hostname: 'old-name', responded: false}}}];
    job.liveHosts = {'192.0.2.1': {address: '192.0.2.1', hostname: 'early-retained', responded: true},
      '192.0.2.2': {address: '192.0.2.2', hostname: 'latest-name', responded: true}};
    const before = JSON.stringify(r), html = renderer.toHTML(r), txt = renderer.toText(r);
    assert.equal((html.match(/data-device-row/g) || []).length, 2);
    assert.ok(html.includes('early-retained') && html.includes('latest-name'));
    assert.ok(txt.includes('early-retained') && txt.includes('latest-name'));
    assert.equal(JSON.stringify(r), before);
    assert.ok(html.indexOf('early-retained') < html.indexOf('latest-name'));
    assert.match(txt, /Duration: 10\.0 s/);
  });
  check('multiple scan scopes remain readable and complete in HTML and text exports', () => {
    const r = reportFixture(3), job = r.jobs[0];
    job.target = '192.0.2.1-3\n198.51.100.0/30\n203.0.113.7';
    job.result.data.scopes = ['192.0.2.1-192.0.2.3', '198.51.100.0/30', '203.0.113.7'];
    job.result.data.network = job.result.data.scopes.join(', ');
    const before = JSON.stringify(r), html = renderer.toHTML(r), txt = renderer.toText(r);
    assert.match(html, /\.target,\.summary-table td:nth-child\(2\)\{white-space:pre-wrap\}/);
    assert.ok(html.includes('<p class="target">' + job.target + '</p>'));
    assert.match(txt, /Target: 192\.0\.2\.1-3\r\n  198\.51\.100\.0\/30\r\n  203\.0\.113\.7/);
    for (const output of [html, txt]) {
      assert.ok(output.includes('Scan scopes'));
      for (const scope of job.result.data.scopes) assert.ok(output.includes(scope));
    }
    assert.equal((html.match(/data-device-row/g) || []).length, 3);
    assert.equal(JSON.stringify(r), before);
  });
  check('completed final hosts are authoritative over stale live/event inventory', () => {
    const r = reportFixture(1);
    r.jobs[0].liveHosts = {'192.0.2.250': {address: '192.0.2.250', hostname: 'stale-live-row'}};
    assert.doesNotMatch(renderer.toHTML(r), /stale-live-row/);
    assert.doesNotMatch(renderer.toText(r), /stale-live-row/);
  });
  check('latest complete live snapshots clear obsolete MAC and vendor fields', () => {
    for (const useLive of [false, true]) {
      const r = reportFixture(0), job = r.jobs[0];
      delete job.result; job.status = 'running';
      job.events = [{data: {type: 'host', value: {address: '192.0.2.8', mac: 'aa:bb:cc:dd:ee:ff', vendor: 'Obsolete vendor'}}}];
      const latest = {address: '192.0.2.8', status: 'responded', responded: true, identityNote: 'Neighbor entries conflict; identity omitted.'};
      if (useLive) job.liveHosts = {'192.0.2.8': latest};
      else job.events.push({data: {type: 'host', value: latest}});
      const html = renderer.toHTML(r), txt = renderer.toText(r);
      // Activity retains the earlier observation; the current device list must not.
      const htmlList = html.slice(html.indexOf('<h3>Device results'), html.indexOf('<details class="supplement activity"'));
      const textList = txt.slice(txt.indexOf('DEVICE LIST'), txt.indexOf('ACTIVITY'));
      assert.doesNotMatch(htmlList, /Obsolete vendor|aa:bb:cc:dd:ee:ff/);
      assert.doesNotMatch(textList, /Obsolete vendor|aa:bb:cc:dd:ee:ff/);
      assert.ok(htmlList.includes('Neighbor entries conflict'));
    }
  });
  check('legacy responders and former subnet checks keep their recorded meanings', () => {
    const r = reportFixture(0);
    r.jobs = [{tool: 'quick', status: 'completed', options: {scope: 'subnet'}, result: {data: {scope: 'subnet', hosts: [
      {address: '192.0.2.1', evidence: ['icmp']},
      {address: '192.0.2.2', evidence: ['icmp'], responded: false},
      {address: '192.0.2.3', evidence: [], hostname: 'name-only.fixture.example'}
    ]}}}];
    for (const body of [renderer.toHTML(r), renderer.toText(r)]) {
      assert.ok(body.includes('Network check (previous version)'));
      assert.ok(body.includes('Response evidence recorded'));
      assert.ok(body.includes('No response'));
      assert.ok(body.includes('Not reported'));
    }
  });
  check('nested arrays preserve zero, false, null and fields beyond table columns', () => {
    const r = reportFixture(0);
    r.jobs = [{tool: 'custom', status: 'completed', result: {data: {
      entries: [{name: 'entry-one', zero: 0, enabled: false, absent: null, x: 1, y: 2, z: 3, finalField: 'last-column-marker', nested: {record: 'deep-marker'}}],
      values: ['alpha', 0, false], output: 'line one\nline two'
    }}}];
    for (const body of [renderer.toHTML(r), renderer.toText(r)]) {
      for (const marker of ['last-column-marker', 'deep-marker', 'line one', 'line two', 'Not reported']) assert.ok(body.includes(marker));
      assert.doesNotMatch(body, /\[object Object\]/);
    }
    assert.match(renderer.toText(r), /Zero: 0/);
    assert.match(renderer.toText(r), /Enabled: No/);
  });
  check('measurements have readable precision without changing counts or source data', () => {
    const r = reportFixture(1);
    r.jobs[0].result.data.hosts[0].latencyMs = 0.8200000000000001;
    r.jobs[1].result.data.statistics = {lossPercent: 100 / 3, minMs: 0.000123456, sent: 4294967296};
    const before = JSON.stringify(r), html = renderer.toHTML(r), txt = renderer.toText(r);
    for (const body of [html, txt]) {
      assert.ok(body.includes('0.82'));
      assert.ok(body.includes('33.333'));
      assert.ok(body.includes('0.000123'));
      assert.ok(body.includes('4294967296'));
      assert.doesNotMatch(body, /0\.8200000000000001|33\.333333333333/);
    }
    assert.equal(JSON.stringify(r), before);
  });
  check('browser and CommonJS APIs are equivalent, deterministic and do not mutate frozen inputs', () => {
    const source = fs.readFileSync(path.join(__dirname, '../web/assets/report-export.js'), 'utf8');
    const context = vm.createContext({URL});
    vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/assets/source-interface.js'), 'utf8'), context, {timeout: 1000});
    vm.runInContext(fs.readFileSync(path.join(__dirname, '../web/assets/result-time.js'), 'utf8'), context, {timeout: 1000});
    vm.runInContext(source, context, {timeout: 1000});
    const r = deepFreeze(reportFixture(3));
    assert.equal(context.NetworkToolkitReportExport.toHTML(r), renderer.toHTML(r));
    assert.equal(context.NetworkToolkitReportExport.toText(r), renderer.toText(r));
    assert.equal(renderer.toHTML(r), renderer.toHTML(r));
    const text = renderer.toText(r);
    assert.doesNotMatch(text.replace(/\r\n/g, ''), /[\r\n]/);
    assert.ok(text.endsWith('\r\n'));
  });
  check('measurement timestamps remain visible as local instants with original source precision', () => {
    const r=reportFixture(1),stamp='2026-10-26T19:20:30.123456789Z';
    r.jobs[0].result.data.hosts[0].time=stamp;
    r.jobs[1].result.data.samples[0].time=stamp;
    const before=JSON.stringify(r);
    for(const output of [renderer.toHTML(r),renderer.toText(r)]){
      assert(output.includes(stamp));assert(output.includes('123'));
      assert(output.includes('local'));
      assert(!output.includes('Times are shown in UTC.'));
    }
    assert.equal(JSON.stringify(r),before);
  });
  check('report captions and formatted values follow selected clock and timezone preferences', () => {
    const resultTime=require('../web/assets/result-time.js'),r=reportFixture(1),stamp='2026-10-26T19:20:30.123456789Z';
    r.jobs[0].result.data.hosts[0].time=stamp;r.jobs[1].result.data.samples[0].time=stamp;
    const before=JSON.stringify(r);
    try{
      resultTime.configure({hourFormat:'12',zoneMode:'utc',timeZone:'UTC'});
      for(const output of [renderer.toHTML(r),renderer.toText(r)]){
        assert.match(output,/UTC with 12-hour clocks/);assert.match(output,/07:20:30\.123\s*PM/i);
        assert(output.includes(stamp));assert(!output.includes('Recorded (local)'));assert(!output.includes('Recorded time (local)'));
      }
      resultTime.configure({hourFormat:'24',zoneMode:'named',timeZone:'Asia/Tokyo'});
      for(const output of [renderer.toHTML(r),renderer.toText(r)]){
        assert(output.includes('Asia/Tokyo with 24-hour clocks'));assert(output.includes('Recorded time (Asia/Tokyo)'));assert.match(output,/04:20:30\.123/);
      }
      assert.equal(JSON.stringify(r),before);
    }finally{resultTime.configure(null);}
  });
  check('live and completed history summaries export without inventing discarded raw rows', () => {
    const history={count:999998,received:999997,lost:1,measured:999997,minMs:0,maxMs:9999,sumMs:1234567,firstTime:'2026-10-01T00:00:00Z',lastTime:'2026-10-27T00:00:00Z'};
    const job={id:'history-export',tool:'monitor',status:'running',sampleStatistics:{sent:1000000,received:999999,lost:1},sampleHistory:history,events:[{data:{type:'sample',value:{sequence:999999,time:'2026-10-27T00:00:01Z',success:true,rttMs:7}}},{data:{type:'sample',value:{sequence:1000000,time:'2026-10-27T00:00:02Z',success:true,rttMs:8}}}]};
    const before=JSON.stringify(job);
    for(const output of [renderer.toHTML({jobs:[job]}),renderer.toText({jobs:[job]})]){assert.match(output,/History/i);assert.match(output,/999[,]?998/);assert.match(output,/9999|9,999/);assert.match(output,/latest 2 retained sample rows/);}
    assert.equal(JSON.stringify(job),before);
    job.result={data:{history:{...history,count:777},samples:[],statistics:{sent:777}}};
    for(const output of [renderer.toHTML({jobs:[job]}),renderer.toText({jobs:[job]})]){assert.match(output,/777/);assert.doesNotMatch(output,/999[,]?998/);}
  });
  check('empty reports and absent data remain explicit and do not invent results', () => {
    assert.match(renderer.toHTML({jobs: []}), /No diagnostic results were selected/);
    assert.match(renderer.toText({jobs: []}), /Selected results: 0/);
    const r = {jobs: [{tool: 'dns', status: 'failed', error: 'Lookup failed'}]};
    assert.match(renderer.toHTML(r), /Lookup failed/);
    assert.match(renderer.toText(r), /No structured result data was recorded/);
  });
  check('trace hostnames and recorded source stay visible in complete, live and failed exports', () => {
    const selected = {mode: 'selected', status: 'validated', interfaceIndex: 7, interfaceName: 'Ethernet 5',
      interfaceDescription: 'USB Gigabit Ethernet', address: '192.0.2.20', family: 'ipv4',
      dnsServer: '192.0.2.53:53', nameResolution: 'bound_dns', note: 'No routing fallback.'};
    const trace = {id: 'trace-source', tool: 'trace', target: '203.0.113.1', status: 'completed',
      result: {summary: 'Destination reached', data: {source: selected, hops: [
        {hop: 1, address: '192.0.2.1', hostname: 'gateway.fixture.example', hostnames: ['gateway.fixture.example'], hostnameStatus: 'resolved', rttMs: 1.5},
        {hop: 2, address: '203.0.113.1', hostnameStatus: 'not_found', rttMs: 9.2}
      ]}}};
    for (const output of [renderer.toHTML({jobs: [trace]}), renderer.toText({jobs: [trace]})]) {
      for (const marker of ['Ethernet 5 - USB Gigabit Ethernet', '192.0.2.20', '192.0.2.53:53', 'gateway.fixture.example', '203.0.113.1']) assert.ok(output.includes(marker), marker);
    }
    delete trace.result; trace.status = 'running'; trace.events = [{data: {type: 'source', value: selected}}];
    assert.match(renderer.toHTML({jobs: [trace]}), /<h3>Diagnostic source<\/h3>/);
    assert.match(renderer.toText({jobs: [trace]}), /Source IP: 192\.0\.2\.20/);
    trace.status = 'failed'; trace.result = {data: {source: {...selected, status: 'invalid', error: '<removed adapter>'}}};
    assert.match(renderer.toHTML({jobs: [trace]}), /&lt;removed adapter&gt;/);
    assert.match(renderer.toText({jobs: [trace]}), /Invalid; no fallback to Automatic/);
  });
  process.stdout.write('Report exporter: ' + passed + ' groups passed.\n');
}

module.exports = {reportFixture, runTests};
if (require.main === module) runTests();
