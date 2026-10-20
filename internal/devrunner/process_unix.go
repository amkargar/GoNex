//go:build !windows

package devrunner

import (
	"os/exec"
	"syscall"
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
