package main

// Both primary workspaces can open editable documents, including the independent
// Terminal's SFTP editor. Successful closes still use the normal shutdown path;
// a timeout is not consent to lose those documents. Detached terminal children
// retain their separate return-to-owner policy and do not use this selector.
func mainEditorCloseApproval(title string) bool {
	return title == appName || title == appName+" Terminal"
}
