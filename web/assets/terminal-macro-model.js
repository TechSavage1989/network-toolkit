/* Shared macro schema. Steps send input to remote terminals; no local code is evaluated. */
(() => {
  'use strict';

  const root = typeof window === 'object' ? window : globalThis;
  const MAX_DEFINITION_BYTES = 4 * 1024 * 1024;
  const MAX_STEP_TEXT_BYTES = 64 * 1024;
  const MAX_STEPS = 1024;
  const MAX_PARAMETERS = 128;
  const DEFAULT_TIMEOUT_MS = 30000;
  const DEFAULT_PACING_MS = 100;
  const MAX_TIMEOUT_MS = 3600000;
  const MAX_PACING_MS = 60000;
  const DEFAULT_PROMPT = '(?:^|[\\r\\n])[^\\r\\n]{0,160}[#>$%][ \\t]*$';
  const parameterName = /^[A-Za-z][A-Za-z0-9_]{0,63}$/;
  const identifier = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,79}$/;
  const keyValues = Object.freeze({
    Enter: '\r', Tab: '\t', Escape: '\x1b', 'Ctrl+C': '\x03', 'Ctrl+Z': '\x1a',
    'Ctrl+D': '\x04', Up: '\x1b[A', Down: '\x1b[B', Left: '\x1b[D', Right: '\x1b[C', Backspace: '\x7f'
  });
  const own = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
  const string = value => typeof value === 'string' ? value : '';
  const number = (value, fallback) => typeof value === 'number' ? value : fallback;
  const object = value => value !== null && typeof value === 'object' && !Array.isArray(value);
  const builtin = name => /^(?:host|username|session)$/i.test(name);
  const placeholders = () => /\{\{([A-Za-z][A-Za-z0-9_]{0,63})\}\}/g;
  const bytes = text => {
    if (typeof TextEncoder !== 'undefined') return new TextEncoder().encode(text).length;
    // UTF-8 length without relying on a browser DOM or mutating the original text.
    let length = 0;
    for (const char of text) {
      const point = char.codePointAt(0);
      length += point <= 0x7f ? 1 : point <= 0x7ff ? 2 : point <= 0xffff ? 3 : 4;
    }
    return length;
  };
  const fail = message => { throw new Error(message); };

  function normalizeShortcut(value) {
    value = string(value).trim();
    if (!value) return '';
    const parts = value.split('+').map(part => part.trim());
    const modifiers = new Set();
    const modifierNames = { ctrl: 'Ctrl', control: 'Ctrl', alt: 'Alt', shift: 'Shift', meta: 'Meta', cmd: 'Meta', command: 'Meta' };
    for (const part of parts.slice(0, -1)) {
      if (!own(modifierNames, part.toLowerCase())) fail('Choose a shortcut such as Ctrl+Shift+1 or Alt+F6.');
      modifiers.add(modifierNames[part.toLowerCase()]);
    }
    let key = parts[parts.length - 1];
    if (/^Key.$/i.test(key)) key = key.slice(3);
    if (key.length === 1 || /^f/i.test(key)) key = key.toUpperCase();
    else key = ['Space', 'Enter', 'Tab', 'Escape', 'Up', 'Down', 'Left', 'Right'].find(candidate => candidate.toLowerCase() === key.toLowerCase()) || key;
    if (!/^(?:[A-Z0-9]|F(?:[1-9]|1[0-9]|2[0-4])|Space|Enter|Tab|Escape|Up|Down|Left|Right)$/.test(key) || (!modifiers.size && !/^F/.test(key))) {
      fail('Choose a shortcut with a modifier or a function key.');
    }
    return ['Ctrl', 'Alt', 'Shift', 'Meta'].filter(name => modifiers.has(name)).concat(key).join('+');
  }

  function normalize(source) {
    source = object(source) ? source : {};
    const definition = {
      id: string(source.id).trim(), name: string(source.name).trim(),
      description: string(source.description).trim(), folder: string(source.folder).trim(),
      tags: Array.isArray(source.tags) ? [...new Set(source.tags.map(value => string(value).trim()).filter(Boolean))] : [],
      favorite: source.favorite === true, shortcut: string(source.shortcut).trim(),
      parameters: (Array.isArray(source.parameters) ? source.parameters : []).map(value => {
        value = object(value) ? value : {};
        const name = string(value.name).trim();
        return { name, label: string(value.label).trim() || name, defaultValue: value.secret === true ? '' : string(value.defaultValue), secret: value.secret === true, required: value.required !== false };
      }),
      steps: (Array.isArray(source.steps) ? source.steps : []).map((value, index) => {
        value = object(value) ? value : {};
        const step = {
          id: string(value.id).trim() || `step-${index + 1}`, kind: string(value.kind).trim() || 'command',
          text: string(value.text), key: string(value.key), timeoutMs: number(value.timeoutMs, 0),
          delayMs: number(value.delayMs, 0), disabled: value.disabled === true
        };
        switch (step.kind) {
          case 'command': case 'text': step.key = ''; step.delayMs = 0; break;
          case 'key': step.text = ''; step.delayMs = 0; step.timeoutMs = 0; break;
          case 'wait': step.key = ''; step.delayMs = 0; break;
          case 'delay': step.text = ''; step.key = ''; step.timeoutMs = 0; break;
          case 'pause': step.key = ''; step.delayMs = 0; step.timeoutMs = 0; break;
        }
        return step;
      }),
      waitForPrompt: source.waitForPrompt !== false,
      prompt: string(source.prompt).trim() || DEFAULT_PROMPT,
      timeoutMs: source.timeoutMs === 0 ? DEFAULT_TIMEOUT_MS : number(source.timeoutMs, DEFAULT_TIMEOUT_MS),
      pacingMs: number(source.pacingMs, DEFAULT_PACING_MS), autoPager: source.autoPager === true
    };
    return definition;
  }

  function checkTypes(value, fields, label) {
    if (!object(value)) fail(`${label} must be an object.`);
    for (const [field, type] of Object.entries(fields)) {
      if (value[field] === undefined || value[field] === null) continue;
      if (type === 'array' ? !Array.isArray(value[field]) : typeof value[field] !== type) fail(`${label} ${field} must be ${type === 'array' ? 'a list' : type}.`);
    }
  }

  function metadata(value, label, limit, multiline = false) {
    const controls = multiline ? /[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]/ : /[\x00-\x1f\x7f-\x9f]/;
    if ([...value].length > limit || controls.test(value)) fail(`${label} must contain at most ${limit} characters without control characters.`);
  }

  function checkTemplate(text) {
    if (/\{\{|\}\}/.test(text.replace(placeholders(), ''))) fail('Use parameter placeholders in the form {{name}}.');
  }

  // Go's RE2 compiler remains authoritative. This checks common syntax errors
  // immediately while accepting RE2's inline flags and named capture syntax.
  function checkPattern(pattern, label) {
    if (/\(\?(?:[=!]|<[=!])/.test(pattern) || /\\[1-9]/.test(pattern) || /\\k</.test(pattern)) fail(`${label} must use RE2 syntax without lookaround or backreferences.`);
    let syntax = pattern.replace(/\(\?P</g, '(?<').replace(/\(\?[imsU-]+\)/g, '').replace(/\(\?[imsU-]+:/g, '(?:');
    // Quoted literals are accepted by RE2 and need escaping for this syntax check.
    syntax = syntax.replace(/\\Q([\s\S]*?)(?:\\E|$)/g, (_, literal) => literal.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
    try { new RegExp(syntax); } catch (_) { fail(`${label} is not a valid regular expression.`); }
  }

  function validate(source) {
    checkTypes(source, { id: 'string', name: 'string', description: 'string', folder: 'string', tags: 'array', favorite: 'boolean', shortcut: 'string', parameters: 'array', steps: 'array', waitForPrompt: 'boolean', prompt: 'string', timeoutMs: 'number', pacingMs: 'number', autoPager: 'boolean' }, 'Macro');
    for (const tag of source.tags || []) if (typeof tag !== 'string') fail('Each tag must be text.');
    for (const parameter of source.parameters || []) checkTypes(parameter, { name: 'string', label: 'string', defaultValue: 'string', secret: 'boolean', required: 'boolean' }, 'Parameter');
    for (const step of source.steps || []) checkTypes(step, { id: 'string', kind: 'string', text: 'string', key: 'string', timeoutMs: 'number', delayMs: 'number', disabled: 'boolean' }, 'Step');
    const definition = normalize(source);
    if (!identifier.test(definition.id)) fail('Choose a macro ID of 1 to 80 letters, digits or ._:-.');
    metadata(definition.name, 'Macro name', 100);
    if (!definition.name) fail('Enter a macro name.');
    metadata(definition.description, 'Description', 2000, true);
    metadata(definition.folder, 'Folder', 120);
    definition.shortcut = normalizeShortcut(definition.shortcut);
    if (!Number.isInteger(definition.timeoutMs) || definition.timeoutMs < 100 || definition.timeoutMs > MAX_TIMEOUT_MS || !Number.isInteger(definition.pacingMs) || definition.pacingMs < 0 || definition.pacingMs > MAX_PACING_MS) fail('Macro timeout must be 100 ms to 1 hour and pacing must be 0 to 60 seconds.');
    if (bytes(definition.prompt) > 4096) fail('Prompt pattern exceeds 4096 bytes.');
    checkPattern(definition.prompt, 'The prompt pattern');
    if ((source.tags || []).length > 32) fail('A macro can have at most 32 tags.');
    for (const tag of definition.tags) metadata(tag, 'Tag', 64);
    if (definition.parameters.length > MAX_PARAMETERS) fail('A macro can have at most 128 parameters.');
    const parameters = new Set();
    for (const parameter of definition.parameters) {
      if (!parameterName.test(parameter.name) || parameters.has(parameter.name) || builtin(parameter.name)) fail('Use unique parameter names beginning with a letter; host, username and session are built in.');
      parameters.add(parameter.name);
      metadata(parameter.label, 'Parameter label', 100);
      if (bytes(parameter.defaultValue) > 8192 || /[\x00\r\n]/.test(parameter.defaultValue)) fail('Parameter defaults must be single-line text of at most 8192 bytes.');
    }
    if (!definition.steps.length || definition.steps.length > MAX_STEPS) fail('A macro must have 1 to 1024 steps.');
    const stepIds = new Set();
    let active = 0;
    for (let index = 0; index < definition.steps.length; index++) {
      const step = definition.steps[index];
      const original = source.steps[index];
      if (!identifier.test(step.id) || stepIds.has(step.id)) fail('Each macro step needs a unique ID of at most 80 characters.');
      stepIds.add(step.id);
      // Check discarded fields as the server does before it canonicalizes them.
      if (bytes(string(original.text)) > MAX_STEP_TEXT_BYTES || /\x00/.test(string(original.text))) fail(`Step ${index + 1} text must be at most 64 KiB without NUL.`);
      const timeout = number(original.timeoutMs, 0), delay = number(original.delayMs, 0);
      if (!Number.isInteger(timeout) || timeout < 0 || timeout > MAX_TIMEOUT_MS || (timeout > 0 && timeout < 100) || !Number.isInteger(delay) || delay < 0 || delay > MAX_TIMEOUT_MS) fail(`Step ${index + 1} has an invalid timeout or delay.`);
      switch (step.kind) {
        case 'command': case 'text':
          if (!step.disabled && !step.text) fail(`Enter text for step ${index + 1}.`);
          if (step.kind === 'command' && /[\r\n]/.test(step.text)) fail(`Step ${index + 1} command must be one line; use separate commands or a Send text step.`);
          break;
        case 'key': if (!own(keyValues, step.key)) fail(`Choose a supported key for step ${index + 1}.`); break;
        case 'wait':
          if (!step.disabled) {
            if (!step.text) fail(`Enter an output pattern for step ${index + 1}.`);
            checkPattern(step.text, `Step ${index + 1} output pattern`);
          }
          break;
        case 'delay': case 'pause': break;
        default: fail(`Step ${index + 1} has an unknown action.`);
      }
      if (['command', 'text', 'wait', 'pause'].includes(step.kind)) {
        checkTemplate(step.text);
        for (const name of detectParams(step.text)) if (!builtin(name) && !parameters.has(name)) fail(`Define the parameter ${name} used in step ${index + 1}.`);
      }
      if (!step.disabled) active++;
    }
    if (!active) fail('Enable at least one macro step.');
    // encoding/json also escapes HTML and the two JavaScript line separators.
    const stored = { ...definition, parameters: definition.parameters.map(parameter => {
      const value = { ...parameter };
      if (!value.defaultValue) delete value.defaultValue;
      return value;
    }), steps: definition.steps.map(step => {
      const value = { ...step };
      if (!value.text) delete value.text;
      if (!value.key) delete value.key;
      return value;
    }) };
    for (const field of ['description', 'folder', 'shortcut']) if (!stored[field]) delete stored[field];
    const encoded = JSON.stringify(stored).replace(/[<>&\u2028\u2029]/g, char => `\\u${char.charCodeAt(0).toString(16).padStart(4, '0')}`);
    if (bytes(encoded) > MAX_DEFINITION_BYTES) fail('Macro definition exceeds the 4 MiB limit.');
    return definition;
  }

  function detectParams(text) {
    return [...new Set([...string(text).matchAll(placeholders())].map(match => match[1]))];
  }

  function expand(text, values = {}) {
    if (typeof text !== 'string' || bytes(text) > MAX_STEP_TEXT_BYTES) fail('Macro step text exceeds 64 KiB or is invalid text.');
    checkTemplate(text);
    if (!object(values)) fail('Parameter values must be an object.');
    const result = text.replace(placeholders(), (_, name) => {
      const key = own(values, name) ? name : builtin(name) && own(values, name.toLowerCase()) ? name.toLowerCase() : null;
      if (key === null) fail(`Enter a value for ${name}.`);
      const value = values[key];
      if (typeof value !== 'string' || /[\x00\r\n]/.test(value)) fail(`Parameter ${name} must be a single line of text.`);
      return value;
    });
    if (bytes(result) > MAX_STEP_TEXT_BYTES) fail('Expanded macro step exceeds 64 KiB.');
    return result;
  }

  function fromCommands(text) {
    return string(text).replace(/\r\n?/g, '\n').split('\n').filter(line => line.trim() !== '').map((text, index) => ({ id: `step-${index + 1}`, kind: 'command', text, key: '', timeoutMs: 0, delayMs: 0, disabled: false }));
  }

  root.NetworkToolkitMacroModel = Object.freeze({
    normalize, validate, expand, fromCommands, detectParams, normalizeShortcut,
    keyBytes: name => own(keyValues, name) ? keyValues[name] : undefined,
    DEFAULT_PROMPT, DEFAULT_TIMEOUT_MS, DEFAULT_PACING_MS,
    MAX_DEFINITION_BYTES, MAX_STEP_TEXT_BYTES, MAX_STEPS, MAX_PARAMETERS, MAX_TIMEOUT_MS, MAX_PACING_MS
  });
})();
