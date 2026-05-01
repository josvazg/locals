package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const (
	createDirPerm = 0750
)

type Control interface {
	Launch(service, cmd string, args ...string) (int, error)
	Stop(service string) error
	IsRunning(service string) bool
}

type ctl struct {
	dir    string
	tmpDir string
	path   string
	l      *log.Logger
}

func New(dir, tmpDir, path string, sinkLog io.Writer) Control {
	logger := log.New(sinkLog, "", log.LstdFlags)
	return &ctl{dir: dir, tmpDir: tmpDir, path: path, l: logger}
}

func (c *ctl) Launch(service, cmd string, args ...string) (int, error) {
	pidFile := filepath.Join(c.dir, fmt.Sprintf("%s.pid", service))
	if pid := readPIDFromFile(pidFile); pid >= 0 {
		if isProcessAlive(pid) {
			c.l.Printf("⚠️ %s is already running (PID: %d). Skipping start.\n", service, pid)
			return pid, nil
		}
		c.l.Println("🔄 Cleaning up stale PID file from previous crash...")
		if err := os.Remove(pidFile); err != nil {
			return 0, fmt.Errorf("failed to remove PID file %q: %w", pidFile, err)
		}
	}
	cmdArgs := append([]string{"env", fmt.Sprintf("PATH=%s", c.path), "nohup", cmd}, args...)
	pid, err := c.launch("sudo", cmdArgs...)
	if err != nil {
		return 0, fmt.Errorf("failed to launch %v: %w", service, err)
	}
	if err := createFile(pidFile, fmt.Sprintf("%d", pid)); err != nil {
		return 0, fmt.Errorf("failed to write the embedded DNS server PID file: %w", err)
	}
	return pid, nil
}

func (c *ctl) Stop(service string) error {
	pidFile := filepath.Join(c.dir, fmt.Sprintf("%s.pid", service))
	pid := readPIDFromFile(pidFile)
	if pid < 0 {
		c.l.Printf("ℹ️ No %s PID file found. Nothing to kill.", service)
		return nil
	}

	if isProcessAlive(pid) {
		if out, err := exec.Command("sudo", "kill", strconv.Itoa(pid)).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to stop locals %s (pid %d): %v %w", service, pid, string(out), err)
		}
		c.l.Printf("🛑 Terminated locals %s (PID: %d)", service, pid)
	} else {
		c.l.Printf("⚠️ PID file exists but process %d is already dead.", pid)
	}

	if out, err := exec.Command("rm", pidFile).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove %s PID file %q: %v %w", service, pidFile, string(out), err)
	}
	return nil
}

func (c *ctl) IsRunning(service string) bool {
	pidFile := filepath.Join(c.dir, fmt.Sprintf("%s.pid", service))
	return isProcessAlive(readPIDFromFile(pidFile))
}

func (c *ctl) launch(cmd string, args ...string) (int, error) {
	fullCmd := fullCmd(cmd, args...)
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
		return -1, fmt.Errorf("failed to launch %q: %w", fullCmd, err)
	}

	// Release allows the Go test to "forget" the process
	pid := command.Process.Pid
	command.Process.Release()
	if !isProcessAlive(pid) {
		return -1, fmt.Errorf("failed to launch %q: pid %d not running", fullCmd, pid)
	}

	return pid, nil
}

func readPIDFromFile(pidFile string) int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return -1
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return -1
	}
	return pid
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

func fullCmd(cmd string, args ...string) string {
	return strings.Join(append([]string{cmd}, args...), " ")
}

func createFile(filename, content string) error {
	dir := filepath.Dir(filename)
	if !pathExists(dir) {
		log.Printf("Creating missing directory: %s", dir)
		if err := os.MkdirAll(dir, createDirPerm); err != nil {
			return fmt.Errorf("failed to create missing dir: %w", err)
		}
	}
	return os.WriteFile(filename, ([]byte)(content), 0600)
}

func pathExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
