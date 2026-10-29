// Deliberately core-only. The application owns diffing, undo and file access.
export {EditorState, EditorSelection, StateEffect, StateField, Compartment,
  Annotation, Transaction, RangeSetBuilder} from '@codemirror/state';
export {EditorView, Decoration, WidgetType, keymap, lineNumbers, drawSelection,
  dropCursor} from '@codemirror/view';
