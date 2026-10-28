# More room for editor documents

The editor's **Open local file** and **Connected session** disclosures start
collapsed. Open either when needed; they are independent and do not open a file
or start a connection by themselves. Their controls remain in the page, so hiding
them does not interrupt a pending read or save. The main workspace remembers each
explicit open/closed choice as a boolean in its existing editor preferences.
Detached editor choices last for that window's lifetime, preserving the existing
boundary that child windows do not write main-workspace settings. Fresh child
disclosures start collapsed and stay window-local; existing transferred appearance
preferences are unchanged, and no new transfer fields are added.

**Move / merge** is now a compact editor-toolbar button, not a permanent extra
row. Its dialog has the existing destination selector, Move tab, Move all/detach
editor, and pending-move cancellation. Escape or Close dismisses only the dialog,
never the move. The toolbar button shows meaningful pending/attention text when
action is needed; reopening it shows the full explanation. Tab stays in the
dialog, and dismissing it returns focus to the button.

The native ownership, acknowledgement, frozen-state, save and close protocols are
unchanged. Moving is not cloning. Unsaved content, undo history and file access
remain subject to the 108 safeguards and documented limits.

Focused tests cover default/explicit disclosure choices, retained state and
non-persistence of document text, keyboard menu access, unchanged polling of an
open destination selector, rejection feedback and cancellation. The existing
browser/native move tests open the real compact control before using its actions.
`editor-height109.browser.cjs` compares actual text and comparison viewport
rectangles against the published 108 checkout in main, detached and terminal-modal
contexts at normal/narrow widths and both themes. Authored tests are not proof of
execution: exact-source hosted reports and actual images are the acceptance gate.

Layout inspiration is limited to compact, discoverable actions and optional
panels described by the official Notepad++ [Views manual](https://github.com/notepad-plus-plus/npp-usermanual/blob/master/content/docs/views.md)
and [user-interface manual](https://github.com/notepad-plus-plus/npp-usermanual/blob/master/content/docs/user-interface.md).
No Notepad++ code, private reference screenshots or user documents are included.
