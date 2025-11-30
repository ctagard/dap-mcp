//go:build !windows

package adapters

import (
	"os/exec"
	"syscall"
)

// setProcAttr sets platform-specific process attributes for spawned debug adapters.
// On Unix, we create a new session so the process becomes a process group leader,
// allowing us to kill the entire process tree when terminating.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
