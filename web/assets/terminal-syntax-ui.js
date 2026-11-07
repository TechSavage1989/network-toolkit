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
  // Universal profile sample (v1.21.12): prompts, links, paths, logs and states.
  const UNIVERSAL_SAMPLE = [
    'biqu@CR10SPro:~$ ping -c 3 192.0.2.1',
    '3 packets transmitted, 3 received, 0% packet loss, time 2003ms',
    'switch#show interfaces status',
    'Gi1/0/1   Uplink to core   connected   10   full  1000',
    'Gi1/0/2                    notconnect  1    auto  auto',
    '*Nov  7 10:00:01: %LINK-3-UPDOWN: Interface GigabitEthernet1/0/2, changed state to down',
    '% Invalid input detected at \'^\' marker.',
    '2026-11-07 10:00:00 WARN  [main] cache nearly full',
    '2026-11-07 10:00:01 INFO  backup copied to \\\\fileserver\\backups\\core-1.cfg',
    'Status page: https://example.com/status',
    'admin@fw01> show interface ethernet1/1',
    '  MAC address 00:1a:2b:3c:4d:5e, IP 192.0.2.10/24, MTU 1500'
  ].join('\n');
  const UNIVERSAL = 'smart', UNIVERSAL_NAME = 'Universal (devices, servers and logs)';
  const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const appState = () => typeof state === 'undefined' ? {} : state;
  const prefs = () => R.preferences(appState().terminalSyntax);
  const current = () => appState().terminalHighlight === false ? 'none' : prefs().active;
  const save = () => { if (typeof saveState === 'function') saveState(); };
  const refreshTerminals = () => { if (typeof updateTerminalTheme === 'function') updateTerminalTheme(); };
  let editing = null, sample = SAMPLE, universalSample = UNIVERSAL_SAMPLE, timer = 0, deleteArmed = false, trying = null;

  // ----- Profile colors (v1.21.11) -----------------------------------------
  // Each rule set can override its seven colors. A color being tried in the
  // picker is kept here, never in saved state, until it is applied.
  const SLOT_NAMES = { red: 'Red', green: 'Green', yellow: 'Yellow', blue: 'Blue', magenta: 'Magenta', cyan: 'Cyan', underline: 'Underline' };
  function colorsFor(id) { const saved = prefs().colors[id] || {}; return trying && trying.id === id ? { ...saved, [trying.slot]: trying.value } : { ...saved }; }
  function setColor(id, slot, value) {
    const p = prefs(), colors = { ...p.colors }, slots = { ...(colors[id] || {}) };
    if (value) slots[slot] = value; else delete slots[slot];
    if (Object.keys(slots).length) colors[id] = slots; else delete colors[id];
    appState().terminalSyntax = { ...p, colors };
  }
  function repaint() { refreshTerminals(); if (typeof updateTerminalHighlightStatus === 'function') updateTerminalHighlightStatus(); if (typeof document !== 'undefined' && document.querySelector('[data-syntax-editor]')) render(); }
  function editColor(slot, id = prefs().active) {
    const picker = root.NetworkToolkitColorPicker, set = R.find(prefs(), id);
    if (!picker || !set || !SLOT_NAMES[slot]) return false;
    const appearance = root.NetworkToolkitTerminalAppearance, pref = appState().terminalAppearance, theme = appState().theme;
    const base = appearance?.syntaxColors ? appearance.syntaxColors(pref, theme)[slot] : '#cccccc', current = colorsFor(id)[slot] || base;
    picker.open({ title: 'Choose ' + SLOT_NAMES[slot] + ' color for ' + set.name, value: current, defaultValue: base, allowAlpha: false,
      background: appearance?.theme ? appearance.theme(pref, theme).background : undefined, semanticColors: {},
      onPreview: value => { trying = { id, slot, value }; repaint(); },
      onApply: (value, meta) => { trying = null; setColor(id, slot, meta?.reset ? null : String(value).slice(0, 7).toLowerCase()); save(); repaint(); },
      onCancel: () => { trying = null; repaint(); } });
    return true;
  }
  function resetColors(id = prefs().active) { if (!prefs().colors[id]) return false; const p = prefs(), colors = { ...p.colors }; delete colors[id]; appState().terminalSyntax = { ...p, colors }; save(); repaint(); return true; }

  function optionsHTML() {
    const p = prefs(), option = (value, label) => '<option value="' + escape(value) + '">' + escape(label) + '</option>';
    return option('none', 'None') + option(UNIVERSAL, UNIVERSAL_NAME) +
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
    return { terminal, syntax: { ...(appearance?.syntaxColors ? appearance.syntaxColors(pref, theme) : {}), ...(editing ? colorsFor(editing.id) : {}) } };
  }
  function allSets() { const p = prefs(); return [...R.BUILT_IN, ...p.custom]; }
  function pick(id) {
    deleteArmed = false;
    if (id === UNIVERSAL) { editing = { id: UNIVERSAL, name: UNIVERSAL_NAME, universal: true }; return; }
    const set = R.find(prefs(), id) || R.BUILT_IN[0]; editing = { ...R.normalize(set), id: set.id, builtIn: !!R.builtIn(set.id) };
  }
  // Universal has no word lists: it recognizes device output by its shape.
  // Its page holds the rule switches, colors and underline options instead.
  function universalBodyHTML(setOptions) {
    const c = colors(), appearance = root.NetworkToolkitTerminalAppearance;
    return '<div class="terminal-syntax-editor terminal-syntax-universal" data-syntax-editor>' +
      '<div class="terminal-syntax-toolbar"><label for="syntaxEditSet">Rule set</label><select id="syntaxEditSet">' + setOptions + '</select>' +
      '<button type="button" class="btn small ghost" data-syntax-act="import">Import .msyn…</button><input type="file" id="syntaxImportFile" accept=".msyn,.ini,.txt" multiple hidden></div>' +
      '<p class="hint">Universal recognizes prompts, interfaces, addresses, URLs and network paths, log levels and states from network devices, servers and shells, so it needs no word lists. Choose what it colors and pick each color. Changes apply right away.</p>' +
      (appearance?.universalEditorHTML ? appearance.universalEditorHTML() : '') +
      '<div class="terminal-syntax-preview"><label for="syntaxSample">Try it with your own text</label><textarea id="syntaxSample" rows="6" spellcheck="false">' + escape(universalSample) + '</textarea>' +
      '<pre id="syntaxPreview" class="terminal-syntax-preview-output" style="background:' + escape(c.terminal.background) + ';color:' + escape(c.terminal.foreground) + '" aria-label="Preview"></pre></div></div>';
  }
  // One card per color, like the Universal page (owner request, v1.21.14):
  // on/off switch, name and note, color button, then the words or pattern
  // (editable for your own and imported sets, folded away for built-ins).
  function cardHTML(color, value, note, on, builtIn) {
    const pattern = '<textarea id="syntaxRule-' + color + '" data-syntax-color="' + color + '" rows="2" spellcheck="false" autocomplete="off" aria-label="' + R.LABELS[color] + ' words or pattern"' + (builtIn ? ' readonly' : '') + '>' + escape(editing.rules[color]) + '</textarea>';
    return '<div class="terminal-universal-rule terminal-syntax-card" data-syntax-slot="' + color + '">' +
      '<input type="checkbox" id="syntaxSlotOn-' + color + '" data-syntax-slot-on="' + color + '"' + (on ? ' checked' : '') + '>' +
      '<label for="syntaxSlotOn-' + color + '"><strong>' + R.LABELS[color] + '</strong><small>' + escape(note || (color === 'underline' ? 'Underlined text' : builtIn ? '' : 'Your words or pattern')) + '</small></label>' +
      '<button type="button" class="ntk-color-trigger' + (color === 'underline' ? ' is-underline' : '') + '" data-syntax-swatch="' + color + '" aria-label="Choose the ' + R.LABELS[color] + ' color for this rule set" title="Change the ' + R.LABELS[color] + ' color for this rule set"><span class="ntk-color-chip" aria-hidden="true" style="background-color:' + escape(value) + ';--ntk-swatch:' + escape(value) + '"></span><code>' + escape(value) + '</code></button>' +
      (builtIn ? '<details class="terminal-syntax-pattern"><summary>Pattern</summary>' + pattern + '</details>' : '<div class="terminal-syntax-pattern">' + pattern + '</div>') +
      '<p class="terminal-syntax-error" id="syntaxError-' + color + '" role="alert"></p></div>';
  }
  function setOff(id, slots) {
    const p = prefs(), off = { ...p.off };
    if (slots.length) off[id] = slots; else delete off[id];
    appState().terminalSyntax = { ...p, off }; save(); repaint();
  }
  function setSlot(id, slot, on) { const current = prefs().off[id] || []; setOff(id, on ? current.filter(item => item !== slot) : [...new Set([...current, slot])]); }
  function bodyHTML() {
    const c = colors(), set = editing, locked = set.builtIn ? ' readonly' : '', disabled = set.builtIn ? ' disabled' : '';
    const setOptions = '<option value="' + UNIVERSAL + '"' + (set.universal ? ' selected' : '') + '>' + escape(UNIVERSAL_NAME) + '</option><optgroup label="Keyword rule sets">' + R.BUILT_IN.map(item => '<option value="' + item.id + '"' + (item.id === set.id ? ' selected' : '') + '>' + escape(item.name) + '</option>').join('') + '</optgroup>' +
      (prefs().custom.length ? '<optgroup label="Your rule sets">' + prefs().custom.map(item => '<option value="' + item.id + '"' + (item.id === set.id ? ' selected' : '') + '>' + escape(item.name) + '</option>').join('') + '</optgroup>' : '');
    if (set.universal) return universalBodyHTML(setOptions);
    const notes = R.builtIn(set.id)?.notes || {}, off = prefs().off[set.id] || [];
    return '<div class="terminal-syntax-editor" data-syntax-editor>' +
      '<div class="terminal-syntax-toolbar"><label for="syntaxEditSet">Rule set</label><select id="syntaxEditSet">' + setOptions + '</select>' +
      '<button type="button" class="btn small ghost" data-syntax-act="duplicate">Duplicate</button>' +
      (set.builtIn ? '' : '<button type="button" class="btn small ghost" data-syntax-act="delete">' + (deleteArmed ? 'Confirm delete' : 'Delete') + '</button>') +
      '<button type="button" class="btn small ghost" data-syntax-act="import">Import .msyn…</button><input type="file" id="syntaxImportFile" accept=".msyn,.ini,.txt" multiple hidden>' +
      '<button type="button" class="btn small ghost" data-syntax-act="export">Export .msyn</button></div>' +
      (set.builtIn ? '<p class="hint">Built-in rule sets can\'t be changed, but you can turn each color on or off and pick its color. Duplicate one to edit its words or patterns.</p>' : '<p class="hint">Changes are saved as you type. Each color uses a regular expression or, with Regular expressions off, a comma-separated word list. MobaXterm patterns can be pasted as they are.</p>') +
      '<div class="terminal-syntax-meta"><label for="syntaxEditName">Name</label><input id="syntaxEditName" maxlength="' + R.LIMITS.name + '" value="' + escape(set.name) + '"' + locked + '>' +
      '<label class="check"><input type="checkbox" id="syntaxEditRegex"' + (set.regex ? ' checked' : '') + disabled + '> Regular expressions</label>' +
      '<label class="check"><input type="checkbox" id="syntaxEditCase"' + (set.caseSensitive ? ' checked' : '') + disabled + '> Case sensitive</label></div>' +
      '<div class="terminal-universal-rules terminal-syntax-cards">' + R.COLORS.map(color => cardHTML(color, c.syntax[color] || '', notes[color], !off.includes(color), set.builtIn)).join('') +
        (root.NetworkToolkitTerminalAppearance?.promptCardHTML ? root.NetworkToolkitTerminalAppearance.promptCardHTML() : '') + '</div>' +
      '<div class="terminal-universal-actions"><button type="button" class="btn small ghost" data-syntax-act="all-on">Turn all colors on</button><button type="button" class="btn small ghost" data-syntax-act="reset-colors">Reset colors</button></div>' +
      '<div class="terminal-syntax-preview"><label for="syntaxSample">Try it with your own text</label><textarea id="syntaxSample" rows="6" spellcheck="false">' + escape(sample) + '</textarea>' +
      '<pre id="syntaxPreview" class="terminal-syntax-preview-output" style="background:' + escape(c.terminal.background) + ';color:' + escape(c.terminal.foreground) + '" aria-label="Preview"></pre></div></div>';
  }
  function footerHTML() {
    return '<button type="button" class="btn ghost" data-action="close-modal">Close</button><button type="button" class="btn primary" data-syntax-act="use">' + useLabel() + '</button>';
  }
  function useLabel() { return editing?.universal ? 'Use Universal' : 'Use this rule set'; }
  function render(focusId) {
    const body = document.querySelector('#modalRoot .modal-body');
    if (!body || !body.querySelector('[data-syntax-editor]')) return;
    body.innerHTML = bodyHTML(); bind(); preview();
    const use = document.querySelector('#modalRoot [data-syntax-act="use"]'); if (use) use.textContent = useLabel();
    root.NetworkToolkitTerminalAppearance?.sync?.(); // Universal and prompt card controls
    if (focusId) document.getElementById(focusId)?.focus?.();
  }
  // Called after Universal settings change (terminal-appearance.js sync).
  function refreshUniversal() { if (editing && typeof document !== 'undefined' && document.querySelector('[data-syntax-editor]')) preview(); }
  function open(target) {
    if (typeof openModal !== 'function') return;
    // The Display panel floats above dialogs; close it so the editor is usable.
    if (typeof setTerminalToolsOpen === 'function') setTerminalToolsOpen(false, false);
    const display = document.getElementById('terminalSessionPanel');
    if (display && !/(^|\s)hidden(\s|$)/.test(display.className)) display.querySelector('[data-terminal-extra="close-session"]')?.click();
    const active = current(), id = target && (target === UNIVERSAL || R.find(prefs(), target)) ? target : active;
    pick(id === 'none' ? UNIVERSAL : id);
    openModal('Syntax highlighting', bodyHTML(), footerHTML(), true);
    bind(); preview();
    root.NetworkToolkitTerminalAppearance?.sync?.();
  }
  function bind() {
    const editor = document.querySelector('[data-syntax-editor]');
    if (!editor || editor.dataset.bound) return;
    editor.dataset.bound = '1';
    editor.addEventListener('click', event => { const swatch = event.target.closest?.('[data-syntax-swatch]'); if (swatch && editing) { event.preventDefault(); editColor(swatch.dataset.syntaxSwatch, editing.id); } });
    editor.addEventListener('input', event => {
      const el = event.target;
      if (el.id === 'syntaxSample') { if (editing.universal) universalSample = el.value.slice(0, 20000); else sample = el.value.slice(0, 20000); schedule(); return; }
      if (editing.builtIn || editing.universal) return;
      if (el.id === 'syntaxEditName') editing.name = el.value;
      else if (el.dataset.syntaxColor) editing.rules[el.dataset.syntaxColor] = el.value;
      else return;
      schedule(true);
    });
    editor.addEventListener('change', event => {
      const el = event.target;
      if (el.id === 'syntaxEditSet') { pick(el.value); render('syntaxEditSet'); return; }
      if (el.id === 'syntaxImportFile') { void importFiles(el.files); el.value = ''; return; }
      if (el.dataset.syntaxSlotOn && !editing.universal) { setSlot(editing.id, el.dataset.syntaxSlotOn, el.checked); return; }
      if (editing.builtIn || editing.universal) return;
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
    if (!editing || editing.builtIn || editing.universal) return;
    const p = prefs(), set = R.normalize({ ...editing });
    set.id = editing.id;
    appState().terminalSyntax = { ...p, custom: p.custom.map(item => item.id === set.id ? set : item) };
    save(); sync();
    if (prefs().active === set.id) refreshTerminals();
  }
  // The Device prompt color as the terminal uses it (Universal's prompt color).
  function promptColor() {
    const appearance = root.NetworkToolkitTerminalAppearance, H = root.NTTerminalHighlight;
    if (!appearance?.highlightOptions || !H) return '';
    const settings = appearance.highlightOptions(appState().terminalAppearance, appState().theme);
    return H.legend(settings.theme, { ...settings, ruleset: null, ruleSet: null, rules: {}, prompt: 'auto' }).find(mark => mark.kind === 'prompt')?.color || '';
  }
  function universalPreview(output) {
    const appearance = root.NetworkToolkitTerminalAppearance, H = root.NTTerminalHighlight;
    if (!appearance?.highlightOptions || !H) { output.textContent = universalSample; return; }
    const settings = appearance.highlightOptions(appState().terminalAppearance, appState().theme), terminal = colors().terminal;
    // Universal's own colors, even while a keyword rule set is the active profile.
    const marks = Object.fromEntries(H.legend(settings.theme, { ...settings, ruleset: null, ruleSet: null, rules: {}, prompt: 'auto' }).map(mark => [mark.kind, mark]));
    // The name in a prompt takes the Device prompt look, or plain text.
    marks['prompt-name'] = { ...marks.prompt, borderWidth: settings.underlines === false ? 0 : marks.prompt?.borderWidth };
    marks['prompt-plain'] = { color: terminal.foreground, borderWidth: 0 };
    output.style.background = terminal.background; output.style.color = terminal.foreground;
    output.innerHTML = universalSample.split('\n').slice(0, 200).map(line => {
      let html = '', last = 0;
      for (const hit of H.analyze(line, 'ssh', settings.rules, null, { prompt: settings.prompt })) {
        const mark = marks[hit.kind]; if (!mark || hit.start < last) continue;
        html += escape(line.slice(last, hit.start)) + '<span data-universal-kind="' + escape(hit.kind) + '" style="color:' + escape(mark.color) + ';' + (mark.borderWidth ? 'border-bottom:' + Number(mark.borderWidth) + 'px ' + escape(mark.borderStyle) + ' ' + escape(mark.color) + ';' : '') + '">' + escape(line.slice(hit.start, hit.end)) + '</span>';
        last = hit.end;
      }
      return html + escape(line.slice(last));
    }).join('\n');
  }
  function preview() {
    const output = document.getElementById('syntaxPreview');
    if (!output || !editing) return;
    if (editing.universal) { universalPreview(output); return; }
    // Errors come from every pattern; matching leaves switched-off colors out.
    const compiled = R.compile(editing), active = R.compile(R.withSwitches(editing, prefs())), c = colors(), H = root.NTTerminalHighlight;
    for (const color of R.COLORS) { const error = document.getElementById('syntaxError-' + color); if (error) error.textContent = compiled.errors[color] || ''; }
    const promptMode = root.NetworkToolkitTerminalAppearance?.highlightOptions?.(appState().terminalAppearance, appState().theme)?.prompt || 'auto';
    const named = { 'prompt-name': promptColor(), 'prompt-plain': c.terminal.foreground };
    output.innerHTML = sample.split('\n').slice(0, 200).map(line => {
      const hits = H ? H.analyze(line, 'ssh', {}, active, { prompt: promptMode }) : active.match(line), marks = [];
      for (let i = 0; i <= line.length; i++) marks.push({ color: '', underline: false });
      for (const hit of hits) for (let i = hit.start; i < hit.end; i++) { if (named[hit.kind] !== undefined) { marks[i].color = named[hit.kind]; continue; } const color = hit.kind.slice(4); if (color === 'underline') marks[i].underline = true; else marks[i].color = c.syntax[color]; }
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
    // The copy starts with the original's colors and switches.
    const colors = p.colors[editing.id] ? { ...p.colors, [id]: { ...p.colors[editing.id] } } : p.colors, off = p.off[editing.id] ? { ...p.off, [id]: [...p.off[editing.id]] } : p.off;
    appState().terminalSyntax = { ...p, custom: [...p.custom, set], colors, off };
    save(); sync(); pick(id); render('syntaxEditName');
  }
  function remove() {
    if (editing.builtIn) return;
    if (!deleteArmed) { deleteArmed = true; render(); document.querySelector('[data-syntax-act="delete"]')?.focus?.(); return; }
    const p = prefs(), id = editing.id, wasActive = p.active === id;
    const colors = { ...p.colors }, off = { ...p.off }; delete colors[id]; delete off[id];
    appState().terminalSyntax = { ...p, active: wasActive ? 'smart' : p.active, custom: p.custom.filter(item => item.id !== id), colors, off };
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
      if (open_) { event.preventDefault(); open(open_.dataset.terminalSyntaxSet); return; }
      const act = event.target.closest?.('[data-syntax-act]');
      if (!act || !editing || !document.querySelector('[data-syntax-editor]')) return;
      event.preventDefault();
      const action = act.dataset.syntaxAct;
      if (action === 'duplicate') duplicate();
      else if (action === 'delete') remove();
      else if (action === 'import') document.getElementById('syntaxImportFile')?.click();
      else if (action === 'export') exportSet();
      else if (action === 'all-on') setOff(editing.id, []);
      else if (action === 'reset-colors') resetColors(editing.id);
      else if (action === 'use') { persist(); choose(editing.id); notify('Using ' + (editing.universal ? 'Universal' : editing.name) + ' for terminal output.'); }
    });
  }

  root.NetworkToolkitTerminalSyntaxUI = Object.freeze({ controlHTML, optionsHTML, sync, choose, open, current, sample: () => SAMPLE, colorsFor, editColor, resetColors, slotNames: SLOT_NAMES, refreshUniversal, universalSample: () => UNIVERSAL_SAMPLE });
})(typeof window === 'object' ? window : globalThis);
