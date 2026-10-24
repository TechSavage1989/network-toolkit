'use strict';

// Development-only fixtures:
// node --max-old-space-size=64 scripts/subnet-planner.test.cjs
// The shipped browser asset itself has no Node or other runtime dependency.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const modulePath = path.resolve(__dirname, '../web/assets/subnet-planner.js');
const planner = require(modulePath);

test('exports the same pure API in a browser without module, DOM or network globals', () => {
  const context = vm.createContext({});
  vm.runInContext(fs.readFileSync(modulePath, 'utf8'), context, { timeout: 1000 });
  assert.deepEqual(Object.keys(context), ['NetworkToolkitSubnetPlanner']);
  assert.deepEqual(Object.keys(context.NetworkToolkitSubnetPlanner), Object.keys(planner));
  assert.equal(context.NetworkToolkitSubnetPlanner.calculate('192.0.2.17/28').cidr, '192.0.2.16/28');
  assert.equal(Object.isFrozen(planner), true);
});

test('IPv4 parsing preserves unsigned endpoints and decimal input without coercing invalid addresses', () => {
  for (const [input, number] of [
    ['0.0.0.0', 0], ['128.0.0.0', 2147483648], ['192.168.10.42', 3232238122],
    ['203.0.113.9', 3405803785], ['255.255.255.255', 4294967295]
  ]) {
    assert.equal(planner.parseIPv4(input), number);
    assert.equal(planner.formatIPv4(number), input);
  }
  assert.equal(planner.formatIPv4(planner.parseIPv4(' 001.002.003.004 ')), '1.2.3.4');
  for (const input of ['', '1.2.3', '1.2.3.4.5', '1.2.3.256', '-1.2.3.4', '1.2.3.4/24',
    '1.2.3. 4', '0x01.2.3.4', '1e2.2.3.4', '0001.2.3.4', null, undefined, {},
    -1, 1.5, 4294967296, Infinity, NaN]) {
    assert.throws(() => planner.parseIPv4(input), { code: 'subnet_address' });
  }
  assert.throws(() => planner.formatIPv4(4294967296), { code: 'subnet_address' });
});

test('all 33 CIDR masks match independent dotted-mask fixtures and reject holes', () => {
  const masks = [
    '0.0.0.0', '128.0.0.0', '192.0.0.0', '224.0.0.0', '240.0.0.0', '248.0.0.0',
    '252.0.0.0', '254.0.0.0', '255.0.0.0', '255.128.0.0', '255.192.0.0',
    '255.224.0.0', '255.240.0.0', '255.248.0.0', '255.252.0.0', '255.254.0.0',
    '255.255.0.0', '255.255.128.0', '255.255.192.0', '255.255.224.0',
    '255.255.240.0', '255.255.248.0', '255.255.252.0', '255.255.254.0',
    '255.255.255.0', '255.255.255.128', '255.255.255.192', '255.255.255.224',
    '255.255.255.240', '255.255.255.248', '255.255.255.252', '255.255.255.254',
    '255.255.255.255'
  ];
  masks.forEach((mask, prefix) => {
    assert.equal(planner.parsePrefix(mask), prefix);
    assert.equal(planner.parsePrefix('/' + prefix), prefix);
    assert.equal(planner.calculate('192.0.2.17', prefix).maskAddress, mask);
  });
  for (const mask of ['255.0.255.0', '255.255.255.1', '127.255.255.255', '255.255.254.255', '0.0.0.1']) {
    assert.throws(() => planner.parsePrefix(mask), { code: 'subnet_mask' });
  }
  for (const value of [-1, 33, NaN, Infinity, 24.5, '', '/33', '24x', '24.0', '1e1', '255.255.255.256', null, {}]) {
    assert.throws(() => planner.parsePrefix(value));
  }
});

test('calculator accepts CIDR and dotted masks while retaining the entered host address', () => {
  for (const input of ['10.7.9.254/22', '10.7.9.254/255.255.252.0', '10.7.9.254 255.255.252.0']) {
    const result = planner.calculate(input, 24);
    assert.equal(result.address, '10.7.9.254');
    assert.equal(result.cidr, '10.7.8.0/22');
    assert.equal(result.maskAddress, '255.255.252.0');
    assert.equal(result.wildcardAddress, '0.0.3.255');
    assert.equal(result.firstAddress, '10.7.8.1');
    assert.equal(result.lastAddress, '10.7.11.254');
    assert.equal(result.broadcastAddress, '10.7.11.255');
    assert.equal(result.total, 1024);
    assert.equal(result.usable, 1022);
  }
  assert.equal(planner.calculate('192.168.1.130', '255.255.255.192').cidr, '192.168.1.128/26');
  assert.equal(planner.calculate('192.168.1.130/26', 8).prefix, 26);
  for (const input of ['192.0.2.1/', '/24', '192.0.2.1//24', '192.0.2.1/24/25', '192.0.2.1 24 25', '192.0.2.1/255.0.255.0']) {
    assert.throws(() => planner.calculate(input));
  }
});

test('/0, /31 and /32 report exact endpoints and distinguish broadcast from upper address', () => {
  const whole = planner.calculate('203.0.113.9/0');
  assert.equal(whole.cidr, '0.0.0.0/0');
  assert.equal(whole.total, 4294967296);
  assert.equal(whole.usable, 4294967294);
  assert.equal(whole.maskAddress, '0.0.0.0');
  assert.equal(whole.wildcardAddress, '255.255.255.255');
  assert.equal(whole.firstAddress, '0.0.0.1');
  assert.equal(whole.lastAddress, '255.255.255.254');
  assert.equal(whole.upperAddress, '255.255.255.255');
  assert.equal(whole.broadcastAddress, '255.255.255.255');
  assert.match(whole.note, /default route/);
  assert.match(whole.note, /limited broadcast/);
  for (const [input, first, last] of [
    ['0.0.0.1/31', '0.0.0.0', '0.0.0.1'],
    ['192.0.2.11/31', '192.0.2.10', '192.0.2.11'],
    ['255.255.255.255/31', '255.255.255.254', '255.255.255.255']
  ]) {
    const result = planner.calculate(input);
    assert.equal(result.kind, 'point-to-point');
    assert.equal(result.total, 2);
    assert.equal(result.usable, 2);
    assert.equal(result.firstAddress, first);
    assert.equal(result.lastAddress, last);
    assert.equal(result.upperAddress, last);
    assert.equal(result.broadcast, null);
    assert.equal(result.broadcastAddress, null);
    assert.match(result.note, /RFC 3021 point-to-point/);
  }
  for (const address of ['0.0.0.0', '192.0.2.11', '255.255.255.255']) {
    const result = planner.calculate(address + '/32');
    assert.equal(result.kind, 'host');
    assert.equal(result.total, 1);
    assert.equal(result.usable, 1);
    assert.equal(result.firstAddress, address);
    assert.equal(result.lastAddress, address);
    assert.equal(result.upperAddress, address);
    assert.equal(result.broadcast, null);
    assert.equal(result.broadcastAddress, null);
  }
});

test('historical classes use address bits independently of CIDR and retain special-use caveats', () => {
  for (const [address, letter, prefix, mask] of [
    ['0.0.0.0', 'A', 8, '255.0.0.0'], ['1.0.0.0', 'A', 8, '255.0.0.0'],
    ['126.255.255.255', 'A', 8, '255.0.0.0'], ['127.255.255.255', 'A', 8, '255.0.0.0'],
    ['128.0.0.0', 'B', 16, '255.255.0.0'], ['191.255.255.255', 'B', 16, '255.255.0.0'],
    ['192.0.0.0', 'C', 24, '255.255.255.0'], ['223.255.255.255', 'C', 24, '255.255.255.0'],
    ['224.0.0.0', 'D', null, null], ['239.255.255.255', 'D', null, null],
    ['240.0.0.0', 'E', null, null], ['255.255.255.255', 'E', null, null]
  ]) {
    const result = planner.classify(address);
    assert.equal(result.class, letter, address);
    assert.equal(result.defaultPrefix, prefix, address);
    assert.equal(result.defaultMask, mask, address);
    assert.match(result.label, /historical/);
  }
  assert.equal(planner.classify('0.0.0.0').exception, 'This host on this network');
  assert.equal(planner.classify('0.1.2.3').exception, 'This network');
  assert.equal(planner.classify('127.0.0.2').exception, 'Loopback');
  assert.equal(planner.classify('255.255.255.255').exception, 'Limited broadcast');
  assert.equal(planner.classify('224.0.0.1').purpose, 'Multicast');
  assert.equal(planner.classify('240.1.2.3').purpose, 'Reserved');
  const broad = planner.page({ address: '192.168.1.2', prefix: 1 });
  assert.equal(broad.classInfo.class, 'C');
  assert.equal(broad.classInfo.defaultPrefix, 24);
  assert.equal(broad.selected.cidr, '128.0.0.0/1');
  assert.equal(broad.parentPrefix, 0);
  assert.equal(broad.networkCount, 2);
});

test('all 33 automatic mask scopes fix complete octets, with one row at every aligned prefix', () => {
  const scopes = {
    0: { address: '0.0.0.0', pattern: '*.*.*.*' },
    8: { address: '192.0.0.0', pattern: '192.*.*.*' },
    16: { address: '192.168.0.0', pattern: '192.168.*.*' },
    24: { address: '192.168.91.0', pattern: '192.168.91.*' },
    32: { address: '192.168.91.173', pattern: '192.168.91.173' }
  };
  // Independent prefix/count/end fixtures catch boundary errors such as
  // carrying /8, /16, /24 or /32 into the preceding octet's possible list.
  for (const [prefix, parentPrefix, total, last] of [
    [0, 0, 1, '0.0.0.0'],
    [1, 0, 2, '128.0.0.0'], [2, 0, 4, '192.0.0.0'],
    [3, 0, 8, '224.0.0.0'], [4, 0, 16, '240.0.0.0'],
    [5, 0, 32, '248.0.0.0'], [6, 0, 64, '252.0.0.0'],
    [7, 0, 128, '254.0.0.0'], [8, 8, 1, '192.0.0.0'],
    [9, 8, 2, '192.128.0.0'], [10, 8, 4, '192.192.0.0'],
    [11, 8, 8, '192.224.0.0'], [12, 8, 16, '192.240.0.0'],
    [13, 8, 32, '192.248.0.0'], [14, 8, 64, '192.252.0.0'],
    [15, 8, 128, '192.254.0.0'], [16, 16, 1, '192.168.0.0'],
    [17, 16, 2, '192.168.128.0'], [18, 16, 4, '192.168.192.0'],
    [19, 16, 8, '192.168.224.0'], [20, 16, 16, '192.168.240.0'],
    [21, 16, 32, '192.168.248.0'], [22, 16, 64, '192.168.252.0'],
    [23, 16, 128, '192.168.254.0'], [24, 24, 1, '192.168.91.0'],
    [25, 24, 2, '192.168.91.128'], [26, 24, 4, '192.168.91.192'],
    [27, 24, 8, '192.168.91.224'], [28, 24, 16, '192.168.91.240'],
    [29, 24, 32, '192.168.91.248'], [30, 24, 64, '192.168.91.252'],
    [31, 24, 128, '192.168.91.254'], [32, 32, 1, '192.168.91.173']
  ]) {
    const result = planner.page({ address: '192.168.91.173', prefix, pageSize: 128 });
    const expected = scopes[parentPrefix];
    assert.equal(planner.defaultParentPrefix('192.168.91.173', prefix), parentPrefix, '/' + prefix);
    assert.equal(result.parentPrefix, parentPrefix, '/' + prefix);
    assert.equal(result.parent.cidr, expected.address + '/' + parentPrefix, '/' + prefix);
    assert.equal(result.basePattern, expected.pattern, '/' + prefix);
    assert.equal(result.networkCount, total, '/' + prefix);
    assert.equal(result.total, total, '/' + prefix);
    assert.equal(result.rows.length, total, '/' + prefix);
    assert.equal(result.pageCount, 1, '/' + prefix);
    assert.equal(result.currentSubnetPage, 1, '/' + prefix);
    assert.equal(result.rows[0].cidr, expected.address + '/' + prefix, '/' + prefix);
    assert.equal(result.rows.at(-1).cidr, last + '/' + prefix, '/' + prefix);
    assert.equal(result.rows.filter(row => row.selected).length, 1, '/' + prefix);
    assert.equal(result.rows.find(row => row.selected).cidr, result.selected.cidr, '/' + prefix);
  }
  assert.throws(() => planner.defaultParentPrefix('not an address', 21), { code: 'subnet_address' });
  assert.throws(() => planner.defaultParentPrefix('192.168.91.173', 33), { code: 'subnet_prefix' });
});

test('automatic lists do not depend on historical class or special-use categories', () => {
  for (const [address, letter, basePattern, parent] of [
    ['10.18.91.173', 'A', '10.18.*.*', '10.18.0.0/16'],
    ['172.18.91.173', 'B', '172.18.*.*', '172.18.0.0/16'],
    ['192.168.91.173', 'C', '192.168.*.*', '192.168.0.0/16'],
    ['224.18.91.173', 'D', '224.18.*.*', '224.18.0.0/16'],
    ['240.18.91.173', 'E', '240.18.*.*', '240.18.0.0/16'],
    ['0.18.91.173', 'A', '0.18.*.*', '0.18.0.0/16'],
    ['127.18.91.173', 'A', '127.18.*.*', '127.18.0.0/16']
  ]) {
    const result = planner.page({ address, prefix: 21, pageSize: 128 });
    assert.equal(result.classInfo.class, letter, address);
    assert.equal(result.basePattern, basePattern, address);
    assert.equal(result.parent.cidr, parent, address);
    assert.equal(result.parentPrefix, 16, address);
    assert.equal(result.networkCount, 32, address);
    assert.equal(result.rows.length, 32, address);
  }
});

test('explicit containing scopes remain available without overstating non-octet patterns', () => {
  for (let parentPrefix = 0; parentPrefix <= 26; parentPrefix++) {
    const result = planner.page({ address: '203.0.113.77', prefix: 26, parentPrefix });
    assert.equal(result.parentPrefix, parentPrefix);
    assert.equal(result.parent.network <= result.selected.network, true);
    assert.equal(result.parent.upper >= result.selected.upper, true);
  }
  const all = planner.page({ address: '203.0.113.77', prefix: 26, parentPrefix: 0 });
  assert.equal(all.parent.cidr, '0.0.0.0/0');
  assert.equal(all.basePattern, '*.*.*.*');
  assert.equal(all.networkCount, 67108864);
  const nonOctet = planner.page({ address: '192.168.91.173', prefix: 26, parentPrefix: 17 });
  assert.equal(nonOctet.basePattern, '192.168.0.0/17');
  assert.equal(nonOctet.networkCount, 512);
  assert.throws(() => planner.page({ address: '203.0.113.77', prefix: 26, parentPrefix: 27 }), { code: 'subnet_parent' });
});

test('the screenshot /21 list contains all 32 exact networks, usable ranges and broadcasts', () => {
  const expected = [
    ['192.168.0.0', '192.168.0.1', '192.168.7.254', '192.168.7.255'],
    ['192.168.8.0', '192.168.8.1', '192.168.15.254', '192.168.15.255'],
    ['192.168.16.0', '192.168.16.1', '192.168.23.254', '192.168.23.255'],
    ['192.168.24.0', '192.168.24.1', '192.168.31.254', '192.168.31.255'],
    ['192.168.32.0', '192.168.32.1', '192.168.39.254', '192.168.39.255'],
    ['192.168.40.0', '192.168.40.1', '192.168.47.254', '192.168.47.255'],
    ['192.168.48.0', '192.168.48.1', '192.168.55.254', '192.168.55.255'],
    ['192.168.56.0', '192.168.56.1', '192.168.63.254', '192.168.63.255'],
    ['192.168.64.0', '192.168.64.1', '192.168.71.254', '192.168.71.255'],
    ['192.168.72.0', '192.168.72.1', '192.168.79.254', '192.168.79.255'],
    ['192.168.80.0', '192.168.80.1', '192.168.87.254', '192.168.87.255'],
    ['192.168.88.0', '192.168.88.1', '192.168.95.254', '192.168.95.255'],
    ['192.168.96.0', '192.168.96.1', '192.168.103.254', '192.168.103.255'],
    ['192.168.104.0', '192.168.104.1', '192.168.111.254', '192.168.111.255'],
    ['192.168.112.0', '192.168.112.1', '192.168.119.254', '192.168.119.255'],
    ['192.168.120.0', '192.168.120.1', '192.168.127.254', '192.168.127.255'],
    ['192.168.128.0', '192.168.128.1', '192.168.135.254', '192.168.135.255'],
    ['192.168.136.0', '192.168.136.1', '192.168.143.254', '192.168.143.255'],
    ['192.168.144.0', '192.168.144.1', '192.168.151.254', '192.168.151.255'],
    ['192.168.152.0', '192.168.152.1', '192.168.159.254', '192.168.159.255'],
    ['192.168.160.0', '192.168.160.1', '192.168.167.254', '192.168.167.255'],
    ['192.168.168.0', '192.168.168.1', '192.168.175.254', '192.168.175.255'],
    ['192.168.176.0', '192.168.176.1', '192.168.183.254', '192.168.183.255'],
    ['192.168.184.0', '192.168.184.1', '192.168.191.254', '192.168.191.255'],
    ['192.168.192.0', '192.168.192.1', '192.168.199.254', '192.168.199.255'],
    ['192.168.200.0', '192.168.200.1', '192.168.207.254', '192.168.207.255'],
    ['192.168.208.0', '192.168.208.1', '192.168.215.254', '192.168.215.255'],
    ['192.168.216.0', '192.168.216.1', '192.168.223.254', '192.168.223.255'],
    ['192.168.224.0', '192.168.224.1', '192.168.231.254', '192.168.231.255'],
    ['192.168.232.0', '192.168.232.1', '192.168.239.254', '192.168.239.255'],
    ['192.168.240.0', '192.168.240.1', '192.168.247.254', '192.168.247.255'],
    ['192.168.248.0', '192.168.248.1', '192.168.255.254', '192.168.255.255']
  ];
  for (const address of ['192.168.91.173/21', '192.168.91.173 255.255.248.0']) {
    const result = planner.page({ address, pageSize: 128 });
    assert.equal(result.basePattern, '192.168.*.*');
    assert.equal(result.networkCount, 32);
    assert.equal(result.rows.length, 32);
    assert.deepEqual(result.rows.map(row => [row.networkAddress, row.firstAddress, row.lastAddress, row.broadcastAddress]), expected);
    assert.equal(result.rows.every(row => row.total === 2048 && row.usable === 2046), true);
    assert.equal(result.rows[11].cidr, '192.168.88.0/21');
    assert.equal(result.rows[11].selected, true);
  }
});

test('automatic /23 and /31 scopes expose all 128 rows with the correct host semantics', () => {
  const subnets = planner.page({ address: '192.168.91.173', prefix: 23, pageSize: 128 });
  assert.equal(subnets.rows.length, 128);
  assert.equal(subnets.pageCount, 1);
  assert.deepEqual([subnets.rows[0].networkAddress, subnets.rows[0].firstAddress, subnets.rows[0].lastAddress, subnets.rows[0].broadcastAddress],
    ['192.168.0.0', '192.168.0.1', '192.168.1.254', '192.168.1.255']);
  assert.deepEqual([subnets.rows[127].networkAddress, subnets.rows[127].firstAddress, subnets.rows[127].lastAddress, subnets.rows[127].broadcastAddress],
    ['192.168.254.0', '192.168.254.1', '192.168.255.254', '192.168.255.255']);
  const endpoints = planner.page({ address: '192.168.91.173', prefix: 31, pageSize: 128 });
  assert.equal(endpoints.rows.length, 128);
  assert.equal(endpoints.pageCount, 1);
  assert.deepEqual([endpoints.rows[0].firstAddress, endpoints.rows[0].lastAddress], ['192.168.91.0', '192.168.91.1']);
  assert.deepEqual([endpoints.rows[127].firstAddress, endpoints.rows[127].lastAddress], ['192.168.91.254', '192.168.91.255']);
  assert.equal(endpoints.rows.every(row => row.total === 2 && row.usable === 2 && row.broadcast === null && row.broadcastAddress === null), true);
  assert.equal(endpoints.rows[86].cidr, '192.168.91.172/31');
  assert.equal(endpoints.rows[86].selected, true);
});

test('paging and numeric sort keep stable indices and locate the entered subnet', () => {
  const options = { address: '192.168.1.130', prefix: 26, pageSize: 2 };
  const first = planner.page(options);
  assert.deepEqual(first.rows.map(row => row.cidr), ['192.168.1.0/26', '192.168.1.64/26']);
  assert.equal(first.total, 4);
  assert.equal(first.pageCount, 2);
  assert.equal(first.currentSubnetPage, 2);
  assert.equal(first.rows.some(row => row.selected), false);
  const second = planner.page({ ...options, page: first.currentSubnetPage });
  assert.deepEqual(second.rows.map(row => [row.cidr, row.index, row.ordinal, row.selected]), [
    ['192.168.1.128/26', 2, 3, true], ['192.168.1.192/26', 3, 4, false]
  ]);
  assert.equal(second.firstResult, 3);
  assert.equal(second.lastResult, 4);
  const descending = planner.page({ ...options, sort: 'desc' });
  assert.deepEqual(descending.rows.map(row => row.cidr), ['192.168.1.192/26', '192.168.1.128/26']);
  assert.equal(descending.currentSubnetPage, 1);
  assert.equal(descending.rows[1].selected, true);
  assert.deepEqual(planner.page({ ...options, sort: 'desc', page: 2 }).rows.map(row => row.cidr), ['192.168.1.64/26', '192.168.1.0/26']);
  assert.deepEqual(planner.page({ address: '192.0.2.10', prefix: 32, parentPrefix: 24, filter: '192.0.2.8/30' }).rows.map(row => row.address),
    ['192.0.2.8', '192.0.2.9', '192.0.2.10', '192.0.2.11']);

  const automatic = { address: '192.168.91.173', prefix: 21, pageSize: 10 };
  const ascending = planner.page(automatic);
  assert.equal(ascending.networkCount, 32);
  assert.equal(ascending.pageCount, 4);
  assert.equal(ascending.currentSubnetPage, 2);
  const currentAscending = planner.page({ ...automatic, page: ascending.currentSubnetPage });
  assert.equal(currentAscending.rows[1].cidr, '192.168.88.0/21');
  assert.equal(currentAscending.rows[1].selected, true);
  const reverse = planner.page({ ...automatic, sort: 'desc' });
  assert.equal(reverse.currentSubnetPage, 3);
  const currentDescending = planner.page({ ...automatic, sort: 'desc', page: reverse.currentSubnetPage });
  assert.equal(currentDescending.rows[0].cidr, '192.168.88.0/21');
  assert.equal(currentDescending.rows[0].selected, true);
});

test('direct IP and CIDR filters intersect subnets arithmetically, including narrow and outside filters', () => {
  const options = { address: '192.168.1.130', prefix: 26, pageSize: 2 };
  for (const filter of ['192.168.1.131', '192.168.1.131/30', '192.168.1.131/32']) {
    const result = planner.page({ ...options, filter });
    assert.equal(result.total, 1);
    assert.equal(result.networkCount, 4);
    assert.equal(result.rows[0].cidr, '192.168.1.128/26');
    assert.equal(result.rows[0].selected, true);
    assert.equal(result.currentSubnetPage, 1);
  }
  const other = planner.page({ ...options, filter: '192.168.1.65' });
  assert.equal(other.rows[0].cidr, '192.168.1.64/26');
  assert.equal(other.currentSubnetPage, null);
  assert.equal(other.rows[0].selected, false);
  const network = planner.page({ ...options, filter: '192.168.1.42/25', sort: 'desc' });
  assert.equal(network.total, 2);
  assert.deepEqual(network.rows.map(row => row.cidr), ['192.168.1.64/26', '192.168.1.0/26']);
  for (const filter of ['0.0.0.0/0', '192.168.0.7/16']) {
    assert.equal(planner.page({ ...options, filter }).total, 4);
  }
  const empty = planner.page({ ...options, filter: '192.168.2.1', page: 100 });
  assert.equal(empty.total, 0);
  assert.equal(empty.page, 1);
  assert.equal(empty.pageCount, 0);
  assert.equal(empty.firstResult, 0);
  assert.equal(empty.lastResult, 0);
  assert.equal(empty.currentSubnetPage, null);
  assert.deepEqual(empty.rows, []);
  for (const filter of ['192.168', '192.168.1.0/33', '192.168.1.0/24 extra', '192.168.1.0-192.168.1.255']) {
    assert.throws(() => planner.page({ ...options, filter }));
  }

  const automatic = { address: '192.168.91.173', prefix: 21, pageSize: 128 };
  const direct = planner.page({ ...automatic, filter: '192.168.95.254' });
  assert.equal(direct.basePattern, '192.168.*.*');
  assert.equal(direct.networkCount, 32);
  assert.equal(direct.total, 1);
  assert.equal(direct.rows[0].cidr, '192.168.88.0/21');
  assert.equal(direct.rows[0].selected, true);
  const intersecting = planner.page({ ...automatic, filter: '192.168.64.0/19', sort: 'desc' });
  assert.deepEqual(intersecting.rows.map(row => row.cidr),
    ['192.168.88.0/21', '192.168.80.0/21', '192.168.72.0/21', '192.168.64.0/21']);
  assert.equal(intersecting.rows[0].selected, true);
  assert.equal(intersecting.currentSubnetPage, 1);
  const outside = planner.page({ ...automatic, filter: '192.169.0.0/16' });
  assert.equal(outside.total, 0);
  assert.equal(outside.currentSubnetPage, null);
  assert.equal(outside.basePattern, '192.168.*.*');
});

test('the complete /0 to /32 space has exact counts, final pages, ranks and reverse order', () => {
  const options = { address: '255.255.255.255', prefix: 32, parentPrefix: 0, pageSize: 100 };
  const first = planner.page(options);
  assert.equal(first.networkCount, 4294967296);
  assert.equal(first.total, 4294967296);
  assert.equal(first.pageCount, 42949673);
  assert.equal(first.currentSubnetPage, 42949673);
  assert.equal(first.rows.length, 100);
  assert.equal(first.rows[0].cidr, '0.0.0.0/32');
  assert.equal(first.rows[99].cidr, '0.0.0.99/32');
  const last = planner.page({ ...options, page: 42949673 });
  assert.equal(last.rows.length, 96);
  assert.equal(last.firstResult, 4294967201);
  assert.equal(last.lastResult, 4294967296);
  assert.equal(last.rows[0].cidr, '255.255.255.160/32');
  assert.equal(last.rows[95].cidr, '255.255.255.255/32');
  assert.equal(last.rows[95].index, 4294967295);
  assert.equal(last.rows[95].ordinal, 4294967296);
  assert.equal(last.rows[95].selected, true);
  const reverse = planner.page({ ...options, sort: 'desc', page: 42949673 });
  assert.equal(reverse.currentSubnetPage, 1);
  assert.equal(reverse.rows[0].cidr, '0.0.0.95/32');
  assert.equal(reverse.rows[95].cidr, '0.0.0.0/32');
  const filtered = planner.page({ ...options, filter: '255.255.255.128/25', page: 2 });
  assert.equal(filtered.total, 128);
  assert.equal(filtered.pageCount, 2);
  assert.equal(filtered.rows.length, 28);
  assert.equal(filtered.rows[0].cidr, '255.255.255.228/32');
  assert.equal(filtered.currentSubnetPage, 2);
  const points = planner.page({ ...options, prefix: 31, page: 21474837 });
  assert.equal(points.total, 2147483648);
  assert.equal(points.rows.length, 48);
  assert.equal(points.rows[47].cidr, '255.255.255.254/31');
  assert.equal(points.rows[47].broadcast, null);
  assert.equal(points.currentSubnetPage, 21474837);
});

test('page bounds clamp without unbounded output and invalid scalar types fail clearly', () => {
  const options = { address: '10.1.2.3', prefix: 23 };
  assert.equal(planner.page({ ...options, pageSize: 1000000000 }).rows.length, 128);
  assert.equal(planner.page({ ...options, pageSize: 0 }).rows.length, 1);
  assert.equal(planner.page({ ...options, page: -1 }).page, 1);
  const last = planner.page({ ...options, page: Number.MAX_SAFE_INTEGER });
  assert.equal(last.page, last.pageCount);
  assert.equal(last.rows.at(-1).cidr, '10.1.254.0/23');
  for (const changes of [{ page: NaN }, { page: 1.1 }, { page: Infinity }, { page: '1e3' },
    { pageSize: 2.5 }, { pageSize: null }, { sort: 'lexical' }, { filter: {} }, { parentPrefix: null }]) {
    assert.throws(() => planner.page({ ...options, ...changes }));
  }
  for (const value of [null, undefined, [], '192.0.2.1']) assert.throws(() => planner.page(value));
  const frozen = Object.freeze({ address: '192.0.2.3', prefix: 26, filter: '' });
  const result = planner.page(frozen);
  result.rows[0].cidr = 'changed fixture';
  assert.equal(planner.page(frozen).rows[0].cidr, '192.0.2.0/26');
});

test('huge-range paging completes under a bounded execution budget', () => {
  // Run this suite with --max-old-space-size=64 for the memory bound. The VM
  // deadline also catches accidental iteration over the entire address space.
  // No subprocess is needed, so the fixture runs in restricted build sandboxes.
  const context = vm.createContext({});
  vm.runInContext(fs.readFileSync(modulePath, 'utf8'), context, { timeout: 1000 });
  const script = `const p=NetworkToolkitSubnetPlanner;
    let count=0;
    for(let i=0;i<100;i++) {
      const r=p.page({address:'255.255.255.255',prefix:32,parentPrefix:0,pageSize:100,page:42949673,sort:i%2?'desc':'asc'});
      if(r.total!==4294967296||r.rows.length!==96) throw Error('incorrect bounded page');
      count+=r.rows.length;
    }
    count;`;
  assert.equal(vm.runInContext(script, context, { timeout: 3000 }), 9600);
});
