/* Terminal "Syntax highlighting" menu and Customize dialog (owner request,
 * v1.21.6: MobaXterm-style syntax colors). Rule sets come from
 * terminal-syntax-rules.js; the choice is saved in state.terminalSyntax and
 * "None" keeps using the existing state.terminalHighlight switch. Changes to a
 * custom rule set are saved as you type. */
(function (root) {
  'use strict';
  const R = root.NTKTerminalSyntaxRules;
  const SAMPLE = [
    'Connection to 192.0.2.10 refused: permission denied',
    'Login successful. Last login: Mon Nov  2 08:52 from 192.0.2.60',
    'Warning: disk space low on /var, see https://example.com/status',
    'info: starting backup at 02:00',
    'interface GigabitEthernet1/0/1',
    ' description Uplink to core-1',
    ' switchport access vlan 10',
    ' no shutdown',
    'GigabitEthernet1/0/2 is down, line protocol is down',
    'Vlan10 is up, line protocol is up',
    '! end of configuration',
    'export PATH=$HOME/bin:$PATH   # add tools',
    'if [ -f "$(which bash)" ]; then echo ok; fi',
    'SELECT name FROM users WHERE id = 1;',
    'my $count = shift; die "missing" unless defined $count;'
  ].join('\n');
  const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const appState = () => typeof state === 'undefined' ? {} : state;
  const prefs = () => R.preferences(appState().terminalSyntax);
  const current = () => appState().terminalHighlight === false ? 'none' : prefs().active;
  const save = () => { if (typeof saveState === 'function') saveState(); };
  const refreshTerminals = () => { if (typeof updateTerminalTheme === 'function') updateTerminalTheme(); };
  let editing = null, sample = SAMPLE, timer = 0, deleteArmed = false;

  function optionsHTML() {
    const p = prefs(), option = (value, label) => '<option value="' + escape(value) + '">' + escape(label) + '</option>';
    return option('none', 'None') + option('smart', 'Network Toolkit device highlighting') +
      '<optgroup label="Keyword rule sets">' + R.BUILT_IN.map(set => option(set.id, set.name)).join('') + '</optgroup>' +
      (p.custom.length ? '<optgroup label="Your rule sets">' + p.custom.map(set => option(set.id, set.name)).join('') + '</optgroup>' : '');
  }
  function controlHTML() {
    return '<label class="terminal-syntax-setting" for="terminalSyntaxSelect">Syntax highlighting</label>' +
      '<select id="terminalSyntaxSelect" title="Colors recognized words and patterns in terminal output">' + optionsHTML() + '</select>' +
      '<button type="button" class="btn small ghost" data-terminal-syntax="customize" title="Edit, import or export syntax rule sets">Customize</button>';
  }
  function sync() {
    const select = typeof document === 'undefined' ? null : document.getElementById('terminalSyntaxSelect');
    if (!select) return;
    const signature = JSON.stringify(prefs().custom.map(set => [set.id, set.name]));
    if (select.dataset.signature !== signature) { select.innerHTML = optionsHTML(); select.dataset.signature = signature; }
    select.value = current();
  }
  function choose(value) {
    const p = prefs();
    if (value === 'none') { if (typeof setTerminalHighlight === 'function') setTerminalHighlight(false); sync(); return; }
    if (value !== 'smart' && !R.find(p, value)) { sync(); return; }
    appState().terminalSyntax = { ...p, active: value };
    if (appState().terminalHighlight === false && typeof setTerminalHighlight === 'function') setTerminalHighlight(true); else save();
    refreshTerminals(); sync();
    if (typeof updateTerminalHighlightStatus === 'function') updateTerminalHighlightStatus();
  }

  // ----- Customize dialog --------------------------------------------------
  function colors() {
    const appearance = root.NetworkToolkitTerminalAppearance, pref = appState().terminalAppearance, theme = appState().theme;
    const terminal = appearance?.theme ? appearance.theme(pref, theme) : { background: '#111', foreground: '#ddd' };
    return { terminal, syntax: appearance?.syntaxColors ? appearance.syntaxColors(pref, theme) : {} };
  }
  function allSets() { const p = prefs(); return [...R.BUILT_IN, ...p.custom]; }
  function pick(id) { const set = R.find(prefs(), id) || R.BUILT_IN[0]; editing = { ...R.normalize(set), id: set.id, builtIn: !!R.builtIn(set.id) }; deleteArmed = false; }
  function bodyHTML() {
    const c = colors(), set = editing, locked = set.builtIn ? ' readonly' : '', disabled = set.builtIn ? ' disabled' : '';
    const setOptions = '<optgroup label="Keyword rule sets">' + R.BUILT_IN.map(item => '<option value="' + item.id + '"' + (item.id === set.id ? ' selected' : '') + '>' + escape(item.name) + '</option>').join('') + '</optgroup>' +
      (prefs().custom.length ? '<optgroup label="Your rule sets">' + prefs().custom.map(item => '<option value="' + item.id + '"' + (item.id === set.id ? ' selected' : '') + '>' + escape(item.name) + '</option>').join('') + '</optgroup>' : '');
    const notes = R.builtIn(set.id)?.notes || {};
    return '<div class="terminal-syntax-editor" data-syntax-editor>' +
      '<div class="terminal-syntax-toolbar"><label for="syntaxEditSet">Rule set</label><select id="syntaxEditSet">' + setOptions + '</select>' +
      '<button type="button" class="btn small ghost" data-syntax-act="duplicate">Duplicate</button>' +
      (set.builtIn ? '' : '<button type="button" class="btn small ghost" data-syntax-act="delete">' + (deleteArmed ? 'Confirm delete' : 'Delete') + '</button>') +
      '<button type="button" class="btn small ghost" data-syntax-act="import">Import .msyn…</button><input type="file" id="syntaxImportFile" accept=".msyn,.ini,.txt" multiple hidden>' +
      '<button type="button" class="btn small ghost" data-syntax-act="export">Export .msyn</button></div>' +
      (set.builtIn ? '<p class="hint">Built-in rule sets can\'t be changed. Duplicate one to edit your own copy.</p>' : '<p class="hint">Changes are saved as you type. Each color is a regular expression or, with Regular expressions off, a comma-separated word list. MobaXterm patterns can be pasted as they are.</p>') +
      '<div class="terminal-syntax-meta"><label for="syntaxEditName">Name</label><input id="syntaxEditName" maxlength="' + R.LIMITS.name + '" value="' + escape(set.name) + '"' + locked + '>' +
      '<label class="check"><input type="checkbox" id="syntaxEditRegex"' + (set.regex ? ' checked' : '') + disabled + '> Regular expressions</label>' +
      '<label class="check"><input type="checkbox" id="syntaxEditCase"' + (set.caseSensitive ? ' checked' : '') + disabled + '> Case sensitive</label></div>' +
      '<div class="terminal-syntax-rules">' + R.COLORS.map(color => '<div class="terminal-syntax-rule"><label for="syntaxRule-' + color + '"><span class="terminal-syntax-swatch' + (color === 'underline' ? ' underline' : '') + '" style="' + (color === 'underline' ? 'border-bottom-color:' : 'background:') + escape(c.syntax[color] || 'currentColor') + '" aria-hidden="true"></span>' + R.LABELS[color] + (notes[color] ? '<span class="hint">' + escape(notes[color]) + '</span>' : '') + '</label>' +
        '<textarea id="syntaxRule-' + color + '" data-syntax-color="' + color + '" rows="2" spellcheck="false" autocomplete="off"' + locked + '>' + escape(set.rules[color]) + '</textarea><p class="terminal-syntax-error" id="syntaxError-' + color + '" role="alert"></p></div>').join('') + '</div>' +
      '<div class="terminal-syntax-preview"><label for="syntaxSample">Try it with your own text</label><textarea id="syntaxSample" rows="6" spellcheck="false">' + escape(sample) + '</textarea>' +
      '<pre id="syntaxPreview" class="terminal-syntax-preview-output" style="background:' + escape(c.terminal.background) + ';color:' + escape(c.terminal.foreground) + '" aria-label="Preview"></pre></div></div>';
  }
  function footerHTML() {
    return '<button type="button" class="btn ghost" data-action="close-modal">Close</button><button type="button" class="btn primary" data-syntax-act="use">Use this rule set</button>';
  }
  function render(focusId) {
    const body = document.querySelector('#modalRoot .modal-body');
    if (!body || !body.querySelector('[data-syntax-editor]')) return;
    body.innerHTML = bodyHTML(); bind(); preview();
    if (focusId) document.getElementById(focusId)?.focus?.();
  }
  function open() {
    if (typeof openModal !== 'function') return;
    // The Display panel floats above dialogs; close it so the editor is usable.
    if (typeof setTerminalToolsOpen === 'function') setTerminalToolsOpen(false, false);
    document.querySelector('#terminalSessionPanel:not(.hidden) [data-terminal-extra="close-session"]')?.click();
    const active = current();
    pick(active === 'none' || active === 'smart' ? 'standard' : active);
    openModal('Syntax highlighting', bodyHTML(), footerHTML(), true);
    bind(); preview();
  }
  function bind() {
    const editor = document.querySelector('[data-syntax-editor]');
    if (!editor || editor.dataset.bound) return;
    editor.dataset.bound = '1';
    editor.addEventListener('input', event => {
      const el = event.target;
      if (el.id === 'syntaxSample') { sample = el.value.slice(0, 20000); schedule(); return; }
      if (editing.builtIn) return;
      if (el.id === 'syntaxEditName') editing.name = el.value;
      else if (el.dataset.syntaxColor) editing.rules[el.dataset.syntaxColor] = el.value;
      else return;
      schedule(true);
    });
    editor.addEventListener('change', event => {
      const el = event.target;
      if (el.id === 'syntaxEditSet') { pick(el.value); render('syntaxEditSet'); return; }
      if (el.id === 'syntaxImportFile') { void importFiles(el.files); el.value = ''; return; }
      if (editing.builtIn) return;
      if (el.id === 'syntaxEditRegex') editing.regex = el.checked;
      else if (el.id === 'syntaxEditCase') editing.caseSensitive = el.checked;
      else return;
      persist(); preview();
    });
  }
  function schedule(store) {
    clearTimeout(timer);
    timer = setTimeout(() => { if (store) persist(); preview(); }, 180);
  }
  function persist() {
    if (!editing || editing.builtIn) return;
    const p = prefs(), set = R.normalize({ ...editing });
    set.id = editing.id;
    appState().terminalSyntax = { ...p, custom: p.custom.map(item => item.id === set.id ? set : item) };
    save(); sync();
    if (prefs().active === set.id) refreshTerminals();
  }
  function preview() {
    const output = document.getElementById('syntaxPreview');
    if (!output || !editing) return;
    const compiled = R.compile(editing), c = colors();
    for (const color of R.COLORS) { const error = document.getElementById('syntaxError-' + color); if (error) error.textContent = compiled.errors[color] || ''; }
    output.innerHTML = sample.split('\n').slice(0, 200).map(line => {
      const hits = compiled.match(line), marks = [];
      for (let i = 0; i <= line.length; i++) marks.push({ color: '', underline: false });
      for (const hit of hits) for (let i = hit.start; i < hit.end; i++) { const color = hit.kind.slice(4); if (color === 'underline') marks[i].underline = true; else marks[i].color = c.syntax[color]; }
      let html = '', openKey = null;
      for (let i = 0; i < line.length; i++) {
        const key = marks[i].color + '|' + marks[i].underline;
        if (key !== openKey) { if (openKey !== null) html += '</span>'; html += '<span style="' + (marks[i].color ? 'color:' + escape(marks[i].color) + ';' : '') + (marks[i].underline ? 'text-decoration:underline;' : '') + '">'; openKey = key; }
        html += escape(line[i]);
      }
      return html + (openKey !== null ? '</span>' : '');
    }).join('\n');
  }
  function duplicate() {
    const p = prefs();
    if (p.custom.length >= R.LIMITS.custom) { notify('You can keep up to ' + R.LIMITS.custom + ' rule sets. Delete one first.', 'error'); return; }
    const name = (editing.name + ' (copy)').slice(0, R.LIMITS.name), id = R.newId(p, name);
    const set = { ...R.normalize({ ...editing, name }), id };
    appState().terminalSyntax = { ...p, custom: [...p.custom, set] };
    save(); sync(); pick(id); render('syntaxEditName');
  }
  function remove() {
    if (editing.builtIn) return;
    if (!deleteArmed) { deleteArmed = true; render(); document.querySelector('[data-syntax-act="delete"]')?.focus?.(); return; }
    const p = prefs(), id = editing.id, wasActive = p.active === id;
    appState().terminalSyntax = { active: wasActive ? 'smart' : p.active, custom: p.custom.filter(item => item.id !== id) };
    save(); sync(); if (wasActive) refreshTerminals();
    pick('standard'); render('syntaxEditSet');
  }
  async function importFiles(list) {
    const files = Array.from(list || []).slice(0, 20);
    let added = 0, lastId = '';
    for (const file of files) {
      try {
        if (file.size > 256 * 1024) throw new Error('The file is larger than 256 KB.');
        const set = R.parseMsyn(new Uint8Array(await file.arrayBuffer())), p = prefs();
        if (p.custom.length >= R.LIMITS.custom) throw new Error('You can keep up to ' + R.LIMITS.custom + ' rule sets.');
        const fileName = String(file.name || '').replace(/\.[^.]+$/, '').replace(/[_]+/g, ' ').trim();
        const taken = name => allSets().some(item => item.name.toLowerCase() === name.toLowerCase());
        // Name sets after their file: MobaXterm exports often all carry the
        // same "Custom: ..." name inside.
        let name = fileName || set.name || 'Imported syntax';
        for (let n = 2; taken(name) && n < 100; n++) name = (fileName || set.name) + ' ' + n;
        const id = R.newId(p, name);
        appState().terminalSyntax = { ...p, custom: [...p.custom, { ...set, name: name.slice(0, R.LIMITS.name), id }] };
        added++; lastId = id;
      } catch (error) { notify('Could not import ' + (file.name || 'the file') + ': ' + error.message, 'error'); }
    }
    if (!added) return;
    save(); sync(); pick(lastId); render('syntaxEditSet');
    notify('Imported ' + added + ' rule set' + (added === 1 ? '' : 's') + '. Choose "Use this rule set" to apply it.');
  }
  function exportSet() {
    const name = String(editing.name || 'syntax').replace(/[\\/:*?"<>|]+/g, '_').trim().slice(0, 70) || 'syntax';
    const bytes = R.encode(R.toMsyn(editing));
    if (typeof download === 'function') download(bytes, name + '.msyn', 'application/octet-stream');
  }
  function notify(message, kind) { if (typeof toast === 'function') toast(message, kind); }

  if (typeof document !== 'undefined') {
    document.addEventListener('click', event => {
      const open_ = event.target.closest?.('[data-terminal-syntax="customize"]');
      if (open_) { event.preventDefault(); open(); return; }
      const act = event.target.closest?.('[data-syntax-act]');
      if (!act || !editing || !document.querySelector('[data-syntax-editor]')) return;
      event.preventDefault();
      const action = act.dataset.syntaxAct;
      if (action === 'duplicate') duplicate();
      else if (action === 'delete') remove();
      else if (action === 'import') document.getElementById('syntaxImportFile')?.click();
      else if (action === 'export') exportSet();
      else if (action === 'use') { persist(); choose(editing.id); notify('Using ' + editing.name + ' for terminal output.'); }
    });
  }

  root.NetworkToolkitTerminalSyntaxUI = Object.freeze({ controlHTML, optionsHTML, sync, choose, open, current, sample: () => SAMPLE });
})(typeof window === 'object' ? window : globalThis);
