package main

// Installation cannot expose an honest percentage while Windows releases file
// locks and the new interface starts. Keep an indeterminate native indicator
// alive in the helper after the original application's WebView has closed.
type updateProgressWindow interface {
	Set(string)
	Close()
}
type silentUpdateProgress struct{}

func (silentUpdateProgress) Set(string) {}
func (silentUpdateProgress) Close()     {}
