package devrunner

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"unsafe"
)

type processOwner struct {
	cmd *exec.Cmd
	job windows.Handle
}

func prepare(cmd *exec.Cmd) (*processOwner, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	return &processOwner{cmd, job}, nil
}
func (p *processOwner) attach() error {
	h, e := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.cmd.Process.Pid))
	if e != nil {
		return e
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(p.job, h)
}
func (p *processOwner) interrupt() {
	if p.cmd.Process != nil {
		_ = windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(p.cmd.Process.Pid))
	}
}
func (p *processOwner) kill()  { _ = windows.TerminateJobObject(p.job, 1) }
func (p *processOwner) close() { windows.CloseHandle(p.job) }

func (p *processOwner) settle() {} // Closing the owned Job Object terminates its remaining processes.
