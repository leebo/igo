//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// setNewProcessGroup is a stub on Windows; process-group semantics differ
// (CREATE_NEW_PROCESS_GROUP flag) and the watcher is currently developed
// against unix only. SIGTERM also has no Windows equivalent. Treat
// `igo dev` on Windows as best-effort until the missing pieces land.
func setNewProcessGroup(cmd *exec.Cmd) {}

// signalProcessGroup just kills the lead process on Windows. Without the
// CREATE_NEW_PROCESS_GROUP flag at spawn time we cannot signal a tree, so
// any workers/sidecars the app forked may survive — known limitation.
func signalProcessGroup(pid int, _ syscall.Signal) {
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Kill()
	}
}
