//go:build !windows

package dap

import (
	"os/exec"
	"syscall"
)

// killProcessGroup kills a process and its entire process group.
// On Unix systems, we use negative PID to signal the entire process group.
func killProcessGroup(pid int, cmd *exec.Cmd) error {
	if pid > 0 {
		// Kill the entire process group (negative PID)
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			// ESRCH means the process doesn't exist (already terminated), which is fine
			if err != syscall.ESRCH {
				return err
			}
		}
	} else if cmd != nil && cmd.Process != nil {
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
// On Unix, we create a new session so the process becomes a process group leader.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
