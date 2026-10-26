/* Shared visibility controls for navigation, settings and details panels. */
(function (root) {
  'use strict';
  const paths = {
    left: 'M15 5l-7 7 7 7', right: 'M9 5l7 7-7 7',
    up: 'M5 15l7-7 7 7', down: 'M5 9l7 7 7-7'
  };
  const revealBindings = new WeakMap();
  const interactive = 'button,a,input,select,textarea,label,summary,[role="tab"],[role="separator"],[contenteditable="true"]';
  function direction(edge, expanded) {
    return edge === 'right' ? (expanded ? 'right' : 'left') : edge === 'bottom' ? (expanded ? 'down' : 'up') : expanded ? 'left' : 'right';
  }
  function icon(edge, expanded) {
    const towards = direction(edge, expanded);
    return '<svg class="panel-visibility-icon" data-panel-direction="' + towards + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="' + paths[towards] + '"/></svg>';
  }
  function caption(name, expanded) { return (expanded ? 'Hide ' : 'Show ') + name; }
  function set(button, options) {
    if (!button) return;
    const {name, edge = 'left', expanded = false} = options;
    const text = caption(name, expanded), signature = JSON.stringify([name, edge, !!expanded]);
    button.classList.add('panel-visibility-toggle');
    button.setAttribute('aria-label', text); button.setAttribute('aria-expanded', String(!!expanded)); button.title = text;
    if (button._ntPanelVisibilitySignature !== signature || !button.querySelector('.panel-visibility-icon')) {
      button.innerHTML = icon(edge, expanded);
      button._ntPanelVisibilitySignature = signature;
    }
  }
  function bindReveal(surface, options) {
    if (!surface) return;
    let binding = revealBindings.get(surface);
    if (!binding) {
      binding = {options}; revealBindings.set(surface, binding);
      surface.addEventListener('click', event => {
        const current = binding.options, collapsed = typeof current.collapsed === 'function' ? current.collapsed() : current.collapsed;
        if (!collapsed || current.clickToReveal === false || event.defaultPrevented || event.button != null && event.button !== 0) return;
        const control = event.target?.closest?.(interactive);
        if (control && control !== surface) return;
        const button = typeof current.button === 'function' ? current.button() : current.button;
        if (!button || button.disabled || typeof button.click !== 'function') return;
        event.preventDefault(); event.stopPropagation(); button.click();
      });
    }
    binding.options = options;
    surface.classList.toggle('panel-reveal-surface', options.clickToReveal !== false && !!(typeof options.collapsed === 'function' ? options.collapsed() : options.collapsed));
  }
  root.NetworkToolkitPanelControls = Object.freeze({icon, caption, set, bindReveal});
})(globalThis);
