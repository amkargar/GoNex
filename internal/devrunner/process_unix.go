//go:build !windows

package devrunner

import (
	"os/exec"
	"syscall"
	"time"
)

type processOwner struct{ cmd *exec.Cmd }

func prepare(cmd *exec.Cmd) (*processOwner, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &processOwner{cmd}, nil
}
func (p *processOwner) attach() error { return nil }
func (p *processOwner) interrupt() {
	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGINT)
	}
}
func (p *processOwner) kill() {
	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
}
func (p *processOwner) close() {}

// Group signals are asynchronous. Allow the kernel to finish descendants before
// returning from cleanup; zombies have no open sockets and need not be reaped
// by this CLI, which deliberately does not become a global subreaper.
func (p *processOwner) settle() {
	if p.cmd.Process == nil {
		return
	}
	until := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(until) {
		if syscall.Kill(-p.cmd.Process.Pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(time.Millisecond)
	}
}
