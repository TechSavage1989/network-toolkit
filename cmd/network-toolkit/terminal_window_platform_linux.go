//go:build linux

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
)

func acquireTerminalWindowLock(path string) (release func(), err error) {
	file, err := openTerminalWindowLockFile(path)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrTerminalWindowInUse
		}
		return nil, fmt.Errorf("lock the Terminal instance: %w", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
			_ = file.Close()
		})
	}, nil
}

func configureTerminalChild(cmd *exec.Cmd) {
	// Development/QA children have their own session and no parent-death signal.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
