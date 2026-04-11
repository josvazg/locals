package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

type Proc interface {
	Run(cmd string, args ...string) (string, error)
	Launch(cmd string, args ...string) (int, error)
	IsProcessAlive(pid int) bool
}

type osProc struct{}

func (_ *osProc) Run(cmd string, args ...string) (string, error) {
	return run(cmd, args...)
}

func (_ *osProc) Launch(cmd string, args ...string) (int, error) {
	return launch(cmd, args...)
}

func (_ *osProc) IsProcessAlive(pid int) bool {
	return isProcessAlive(pid)
}

func run(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		cli := strings.Join(append([]string{cmd}, args...), " ")
		return "", fmt.Errorf("failed to run %s: %w", cli, err)
	}
	return string(out), err
}

func launch(cmd string, args ...string) (int, error) {
	cli := strings.Join(append([]string{cmd}, args...), " ")
	command := exec.Command(cmd, args...)
	// FIX: Disconnect the background process from the test's pipes.
	// This prevents the parent 'CombinedOutput' from hanging.
	command.Stdout = nil
	command.Stderr = nil
	command.Stdin = nil
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
	if err := command.Start(); err != nil {
		return -1, fmt.Errorf("failed to launch %q: %w", cli, err)
	}

	// Release allows the Go test to "forget" the process
	pid := command.Process.Pid
	command.Process.Release()

	return pid, nil
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))

	if err == nil {
		return true
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if errors.Is(err, syscall.EPERM) {
			return true
		}
		if errors.Is(err, syscall.ESRCH) {
			return false
		}
	}
	return false
}
