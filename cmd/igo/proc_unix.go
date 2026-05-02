//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// setNewProcessGroup makes the child the leader of a new process group, so
// signalProcessGroup can later kill the whole subtree the app spawned.
func setNewProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// signalProcessGroup sends sig to every process in the group whose leader is
// pid. Falls back to signalling the lead pid only if the group lookup fails
// (e.g. the process already exited).
func signalProcessGroup(pid int, sig syscall.Signal) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil || pgid <= 0 {
		_ = syscall.Kill(pid, sig)
		return
	}
	_ = syscall.Kill(-pgid, sig)
}
