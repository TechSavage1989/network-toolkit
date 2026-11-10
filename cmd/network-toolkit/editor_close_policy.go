package main

// Primary workspaces can open editable documents. Detached windows use their
// own close policies. A timeout is not consent to lose an open document.
func mainEditorCloseApproval(title string) bool {
	return title == appName || title == appName+" Terminal" || title == standaloneEditorTitle
}
