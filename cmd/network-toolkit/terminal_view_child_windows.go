//go:build windows

package main

import "os/exec"

func configureTerminalViewChild(cmd *exec.Cmd) {
	configureTerminalChild(cmd)
	// The stdin bootstrap needs a designated inherited handle. Go's Windows
	// StartProcess installs PROC_THREAD_ATTRIBUTE_HANDLE_LIST containing only
	// the duplicated stdin/stdout/stderr handles (and no additional handles).
	// NoInheritHandles=true would suppress even that explicitly supplied pipe.
	cmd.SysProcAttr.NoInheritHandles = false
}
