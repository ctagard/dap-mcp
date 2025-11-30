//go:build windows

package dap

import (
	"os/exec"
	"syscall"
)

// killProcessGroup kills a process on Windows.
// Windows doesn't have Unix-style process groups, so we just kill the process directly.
// For proper child process cleanup, we use CREATE_NEW_PROCESS_GROUP flag.
func killProcessGroup(pid int, cmd *exec.Cmd) error {
	if cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Kill(); err != nil {
			// "process already finished" is not an error we care about
			if err.Error() != "os: process already finished" {
				return err
			}
		}
	}
	return nil
}

// setProcAttr sets platform-specific process attributes.
// On Windows, we create a new process group so we can potentially signal child processes.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
