'use strict';
const assert = require('node:assert/strict');
const source = require('../web/assets/source-interface.js');
let passed = 0;
function check(name, fn) { fn(); passed++; process.stdout.write('PASS: ' + name + '\n'); }
const adapters = [
  {index: 7, name: 'Ethernet 5', description: 'USB Gigabit Ethernet', up: true,
    addresses: ['192.0.2.20/24', '2001:db8::20/64', 'fe80::20%7/64', '0.0.0.0/0', 'ff02::1/16', '192.0.2.20/24']},
  {index: 8, name: 'VPN', description: 'Lab VPN interface', up: true, addresses: ['198.51.100.20/24']},
  {index: 9, name: 'Wi-Fi', up: false, addresses: ['203.0.113.20/24']},
  {index: 1, name: 'Loopback', up: true, loopback: true, addresses: ['127.0.0.1/8', '::1/128']},
  {index: 10, name: 'Pending', up: true, addresses: ['192.0.2.30/24', '192.0.2.31/24', '192.0.2.32/24'],
    addressDetails: [{address: '192.0.2.30', state: 'tentative'}, {address: '192.0.2.31', state: 'duplicate'}, {address: '192.0.2.32', state: 'invalid'}]}
];
const selected = source.choices(adapters).find(item => item.address === '192.0.2.20');
check('available sources retain adapter descriptions, dual-stack addresses, zones and VPN/loopback interfaces', () => {
  const items = source.choices(adapters);
  assert.equal(items.length, 6);
  assert.equal(selected.interfaceDescription, 'USB Gigabit Ethernet');
  assert.equal(items.find(item => item.address === 'fe80::20%7').family, 'ipv6');
  assert.ok(items.some(item => item.interfaceName === 'VPN'));
  assert.ok(items.some(item => item.address === '::1'));
  assert.ok(!items.some(item => ['Wi-Fi', 'Pending'].includes(item.interfaceName)));
});
check('stale index, alias or assigned address stays unavailable without changing to Automatic', () => {
  for (const changed of [
    adapters.filter(item => item.index !== 7),
    adapters.map(item => item.index === 7 ? {...item, name: 'Different adapter'} : item),
    adapters.map(item => item.index === 7 ? {...item, addresses: ['192.0.2.99/24']} : item)
  ]) {
    const status = source.availability(selected, changed, 'ping', false);
    assert.equal(status.ok, false); assert.deepEqual(status.selected, selected);
    const html = source.controlHTML(selected, changed, 'ping', false);
    assert.match(html, /selected>Unavailable: Ethernet 5 - USB Gigabit Ethernet/);
    assert.match(html, /source-unavailable/);
    assert.doesNotMatch(html, /value="" selected/);
  }
});
check('only the ten supported diagnostics receive explicit source options', () => {
  for (const tool of ['quick', 'ping', 'trace', 'tcp', 'monitor', 'http', 'tls', 'discover', 'identify', 'dns']) {
    assert.deepEqual(source.requestOptions(tool, selected), {sourceInterfaceIndex: 7, sourceInterfaceName: 'Ethernet 5', sourceAddress: '192.0.2.20'});
    assert.deepEqual(source.requestOptions(tool, null), {});
  }
  for (const tool of ['arp', 'routes', 'connections', 'wifi', 'ipconfig', 'wol', 'terminal', 'publicip']) assert.deepEqual(source.requestOptions(tool, selected), {});
});
check('IPv4-only scans report an incompatible shared IPv6 choice without changing it', () => {
  const ipv6 = source.choices(adapters).find(item => item.address === '2001:db8::20');
  assert.equal(source.availability(ipv6, adapters, 'trace', false).ok, true);
  for (const [tool, subnet] of [['discover', false], ['identify', false], ['ping', true], ['tcp', true]]) {
    assert.equal(source.availability(ipv6, adapters, tool, subnet).ok, false);
  }
  assert.equal(ipv6.address, '2001:db8::20');
});
check('result metadata uses its recorded source, then live event, then requested options', () => {
  const options = {sourceInterfaceIndex: 8, sourceInterfaceName: 'VPN', sourceAddress: '198.51.100.20'};
  const live = {...selected, mode: 'selected', status: 'validated', dnsServer: '192.0.2.53:53', nameResolution: 'bound_dns'};
  const final = {...live, status: 'invalid', error: 'Adapter removed'};
  const job = {id: 'fixture', tool: 'trace', options, events: [{data: {type: 'source', value: live}}], result: {data: {source: final}}};
  assert.equal(source.jobSource(job), final);
  assert.match(source.resultHTML(job), /Source: Ethernet 5 \/ 192.0.2.20 \(unavailable\)/);
  delete job.result; assert.equal(source.jobSource(job), live);
  delete job.events; assert.equal(source.jobSource(job).status, 'pending');
  assert.equal(source.jobSource(job).interfaceName, 'VPN');
  delete job.options; assert.equal(source.jobSource(job).mode, 'automatic');
});
check('labels and imported source errors are escaped and source selections normalize safely', () => {
  const attack = '<img src=x onerror=alert(1)>';
  const job = {id: attack, tool: 'trace', result: {data: {source: {...selected, mode: 'selected', status: 'invalid', interfaceName: attack, error: attack}}}};
  const html = source.resultHTML(job);
  assert.doesNotMatch(html, /<img/); assert.match(html, /&lt;img/);
  assert.equal(source.normalize({interfaceIndex: '7', address: '192.0.2.20'}), null);
  assert.equal(source.normalize({interfaceIndex: 7, address: 'not-an-ip'}), null);
  assert.equal(source.normalize(selected).address, selected.address);
});
process.stdout.write(passed + ' source-interface checks passed.\n');
