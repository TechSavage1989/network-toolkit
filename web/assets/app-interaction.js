/* Suppress browser chrome without taking over the app's own context menus. */
(function () {
 'use strict';
 // Capture also covers plain controls and handlers that stop propagation.
 // Do not stop the event, move focus or intercept keys: editor/terminal/result
 // menus, their keyboard paths and normal text editing remain owned by them.
 document.addEventListener('contextmenu',event=>event.preventDefault(),{capture:true});
})();
