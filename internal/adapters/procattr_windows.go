//go:build windows

package adapters

import (
	"os/exec"
	"syscall"
)

// setProcAttr sets platform-specific process attributes for spawned debug adapters.
// On Windows, we create a new process group to allow for better process management.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
