package xray

import (
	"os/exec"
	"syscall"
)

// xray must not outlive swift if swift is killed.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
