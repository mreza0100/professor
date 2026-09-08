//go:build linux || darwin

package run

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// configureBoundedCommand gives the engine and every helper it starts one
// process group. Cancellation kills the group, and WaitDelay prevents an
// inherited descriptor from keeping a bounded pipe open forever.
func configureBoundedCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = 500 * time.Millisecond
}
