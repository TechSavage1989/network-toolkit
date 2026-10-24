'use strict';
const fs = require('node:fs'), vm = require('node:vm'), assert = require('node:assert/strict'), {test} = require('node:test');
const {document: makeDocument} = require('./terminal-dom-fixture.cjs');
const html = fs.readFileSync(require.resolve('../web/index.html'), 'utf8');
const configuration = fs.readFileSync(require.resolve('../web/assets/configuration-layout.js'), 'utf8');
const discovery = fs.readFileSync(require.resolve('../web/assets/discovery-workspace.js'), 'utf8');
function base(markup) {
  const document = makeDocument(markup); document.body = document;
  for (const node of [document, ...document.querySelectorAll('div,section,aside')]) node.style.setProperty = function (key, value) { this[key] = value; };
  const calls = [], listeners = new Map(), state = {activePage: 'ping', toolLayout: {width: 380, collapsed: false}, discoveryDetails: {width: 430, height: 275, hidden: false, dock: 'right'}};
  const c = {document, state, console, Map, Set, Math, Number, String, innerWidth: 1500, MutationObserver: class {observe() {}}, ResizeObserver: class {observe() {} unobserve() {}}, requestAnimationFrame() {}, saveState: () => calls.push('saved'), icon: () => '<svg></svg>', closeEntryContextMenu() {}, updateToolResultSticky() {}, activeTool: () => state.activePage, TOOLS: {ping: {}}, $: (selector, owner = document) => owner.querySelector(selector), $$: (selector, owner = document) => owner.querySelectorAll(selector), addEventListener: (type, fn) => { const group = listeners.get(type) || []; group.push(fn); listeners.set(type, group); }, removeEventListener: (type, fn) => listeners.set(type, (listeners.get(type) || []).filter(value => value !== fn))};
  c.window = c; vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/panel-controls.js'), 'utf8'), c);
  return {c, state, calls, document, listeners};
}
function configHarness(width = 1400) {
  const h = base('<div id="toolWorkspace"><div id="toolCommandBar"><button id="toolConfigurationToggle"></button></div><section id="toolConfiguration"><div id="toolConfigHeading"><span>Configuration</span><button id="configurationExpandAll"></button></div></section><div id="toolDivider"></div><div id="pageContent"></div></div>');
  h.document.getElementById('toolWorkspace').clientWidth = width;
  for (const name of ['safeToolLayout', 'splitToolPage', 'compactToolWorkspace', 'toolConfigurationLimit', 'applyToolLayout', 'setToolPanel']) vm.runInContext(html.split('\n').find(line => line.startsWith('function ' + name + '(')), h.c);
  vm.runInContext('const toolPanelViews=new Map();', h.c);
  vm.runInContext(configuration, h.c); h.c.applyToolLayout(); h.c.NetworkToolkitConfiguration.init();
  return h;
}
function detailsHarness(dock = 'right', width = 1400, height = 700) {
  const h = base('<div id="networkResultsWorkspace"><div id="toolResultsScroll"></div><div id="deviceDetailsDivider"></div><aside id="selectedDevicePane"></aside></div><button data-device-pane-toggle></button>');
  h.state.discoveryDetails.dock = dock;
  h.document.getElementById('networkResultsWorkspace').getBoundingClientRect = () => ({width, height});
  h.document.getElementById('selectedDevicePane').getBoundingClientRect = () => ({width: h.state.discoveryDetails.width, height: h.state.discoveryDetails.height});
  vm.runInContext(discovery, h.c); h.c.NetworkToolkitWorkspace.init(); h.c.NetworkToolkitWorkspace.layoutDetails();
  return h;
}
test('configuration collapse keeps its previous width and moves its control beside the sidebar', () => {
  const h = configHarness(), divider = h.document.getElementById('toolDivider'), toggle = h.document.getElementById('toolConfigurationToggle');
  assert.equal(toggle.parentElement.id, 'toolConfigHeading');
  assert.equal(h.document.getElementById('toolConfigHeading').firstElementChild,toggle,'Hide control leads its panel heading');
  assert.equal(toggle.textContent,'','Configuration Hide control is icon-only');
  assert.equal(toggle.getAttribute('aria-label'),'Hide configuration');assert.equal(toggle.querySelector('svg').dataset.panelDirection,'left');
  h.document.emit('dblclick', divider);
  assert.equal(h.state.toolLayout.collapsed, true); assert.equal(h.state.toolLayout.width, 380);
  assert.equal(toggle.parentElement.id, 'toolConfigurationRail'); assert.equal(toggle.getAttribute('aria-expanded'), 'false'); assert.equal(divider.getAttribute('aria-valuenow'), '0');
  assert.equal(toggle.getAttribute('aria-label'),'Show configuration');assert.equal(toggle.querySelector('svg').dataset.panelDirection,'right');
  assert.equal(toggle.textContent,'','Configuration Show control is icon-only');
  let prevented = false;
  h.document.emit('pointerdown', divider, {preventDefault() { prevented = true; }});
  assert(prevented); assert.equal(h.state.toolLayout.width, 380);
  h.document.emit('dblclick', divider);
  assert.equal(h.state.toolLayout.collapsed, false); assert.equal(h.state.toolLayout.width, 380); assert.equal(toggle.parentElement.id, 'toolConfigHeading'); assert.equal(toggle.getAttribute('aria-expanded'), 'true');
});
test('configuration divider supports keyboard collapse and the nearby button restores it', () => {
  const h = configHarness(), divider = h.document.getElementById('toolDivider');
  h.document.emit('keydown', divider, {key: ' '}); assert(h.state.toolLayout.collapsed);
  h.document.emit('keydown', divider, {key: 'Enter'}); assert(!h.state.toolLayout.collapsed);
  h.document.emit('click', h.document.getElementById('toolConfigurationToggle')); assert(h.state.toolLayout.collapsed);
  h.document.emit('keydown', divider, {key: 'ArrowRight'}); assert(!h.state.toolLayout.collapsed); assert.equal(h.state.toolLayout.width, 380);
});
test('the whole collapsed configuration rail opens once while button descendants keep their own action',()=>{
  const h=configHarness(),toggle=h.document.getElementById('toolConfigurationToggle');toggle.click=()=>h.document.emit('click',toggle);
  h.document.emit('click',toggle);const rail=h.document.getElementById('toolConfigurationRail');assert(h.state.toolLayout.collapsed);assert(rail.classList.contains('panel-reveal-surface'));
  rail.emit('click',rail,{button:0});assert(!h.state.toolLayout.collapsed);assert.equal(h.document.getElementById('toolConfigHeading').firstElementChild,toggle);
  h.document.emit('click',toggle);rail.emit('click',rail,{button:2});assert(h.state.toolLayout.collapsed,'right click should not reveal');rail.emit('click',toggle.querySelector('svg'),{button:0});assert(h.state.toolLayout.collapsed,'the surface handler must not activate button descendants');h.document.emit('click',toggle.querySelector('svg'));assert(!h.state.toolLayout.collapsed,'the button itself should open once');
  assert.equal(rail.listeners.click.length,1,'rerendering added duplicate collapsed-surface handlers');const saves=h.calls.length;rail.emit('click',rail,{button:0});assert.equal(h.calls.length,saves,'expanded panels should not toggle from their old rail');
});
test('compact configuration controls follow the active panel and retain the desktop width', () => {
  const h = configHarness(800), toggle = h.document.getElementById('toolConfigurationToggle'), workspace = h.document.getElementById('toolWorkspace');
  h.document.emit('click', toggle); assert(workspace.classList.contains('configuration-hidden')); assert.equal(toggle.getAttribute('aria-expanded'), 'false'); assert.equal(toggle.parentElement.id, 'toolConfigurationRail');
  h.document.emit('click', toggle); assert(!workspace.classList.contains('configuration-hidden')); assert.equal(toggle.getAttribute('aria-expanded'), 'true'); assert.equal(h.state.toolLayout.width, 380);
});
for (const dock of ['right', 'bottom']) test(dock + ' device details collapse and restore without resetting size', () => {
  const h = detailsHarness(dock), divider = h.document.getElementById('deviceDetailsDivider'), workspace = h.document.getElementById('networkResultsWorkspace');
  h.document.emit('dblclick', divider); assert(h.state.discoveryDetails.hidden); assert(workspace.classList.contains('details-hidden')); assert.equal(divider.getAttribute('aria-valuenow'), '0');
  const show=h.document.querySelector('#deviceDetailsRail button');assert.equal(show.getAttribute('aria-label'),'Show device details');assert.equal(show.querySelector('svg').dataset.panelDirection,dock==='right'?'left':'up');
  h.document.emit('pointerdown', divider, {button: 0, clientX: 500, clientY: 500}); assert.equal((h.listeners.get('pointermove') || []).length, 0);
  h.document.emit('dblclick', divider); assert(!h.state.discoveryDetails.hidden); assert.equal(h.state.discoveryDetails.width, 430); assert.equal(h.state.discoveryDetails.height, 275);
  assert.equal(divider.getAttribute('aria-valuenow'), dock === 'right' ? '430' : '275'); assert.equal(divider.getAttribute('aria-orientation'), dock === 'right' ? 'vertical' : 'horizontal');
  h.document.emit('keydown', divider, {key: 'Enter'}); assert(h.state.discoveryDetails.hidden);
  h.document.emit('keydown', divider, {key: ' '}); assert(!h.state.discoveryDetails.hidden);
  h.document.emit('keydown', divider, {key: 'Enter'});
  h.document.emit('keydown', divider, {key: dock === 'right' ? 'ArrowLeft' : 'ArrowUp'}); assert(!h.state.discoveryDetails.hidden); assert.equal(h.state.discoveryDetails[dock === 'right' ? 'width' : 'height'], dock === 'right' ? 440 : 285);
  h.document.emit('keydown', divider, {key: 'Home'}); assert.equal(h.state.discoveryDetails[dock === 'right' ? 'width' : 'height'], dock === 'right' ? 330 : 260);
});
for(const dock of ['right','bottom'])test('blank '+dock+' device details rail restores the existing pane',()=>{
  const h=detailsHarness(dock),divider=h.document.getElementById('deviceDetailsDivider');h.document.emit('dblclick',divider);const rail=h.document.getElementById('deviceDetailsRail'),show=rail.querySelector('button');show.click=()=>h.document.emit('click',show);assert(h.state.discoveryDetails.hidden);rail.emit('click',rail,{button:0});assert(!h.state.discoveryDetails.hidden);assert.equal(h.state.discoveryDetails.width,430);assert.equal(h.state.discoveryDetails.height,275);assert.equal(rail.listeners.click.length,1);
});
test('drag resizing is preserved and the resized device details width survives collapse', () => {
  const h = detailsHarness(), divider = h.document.getElementById('deviceDetailsDivider');
  h.document.emit('pointerdown', divider, {button: 0, clientX: 500, clientY: 500});
  for (const move of h.listeners.get('pointermove') || []) move({clientX: 460, clientY: 500});
  for (const finish of [...h.listeners.get('pointerup') || []]) finish();
  assert.equal(h.state.discoveryDetails.width, 470);
  h.document.emit('dblclick', divider); h.document.emit('dblclick', divider);
  assert.equal(h.state.discoveryDetails.width, 470); assert.equal(divider.getAttribute('aria-valuenow'), '470');
});
