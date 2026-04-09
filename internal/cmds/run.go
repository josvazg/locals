package cmds

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"syscall"
)

func run(dryrun bool, cmd string, args ...string) error {
	cli := strings.Join(append([]string{cmd}, args...), " ")
	if dryrun {
		log.Printf("%s", cli)
		return nil
	}
	out, err := readOutput(cmd, args...)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func readOutput(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		fullCommand := append([]string{cmd}, args...)
		return "", fmt.Errorf("%v\n%s: %w", strings.Join(fullCommand, " "), string(out), err)
	}
	return string(out), nil
}

func test(cmd string, args ...string) bool {
	return run(false, cmd, args...) == nil
}

func launch(dryrun bool, cmd string, args ...string) (int, error) {
	cli := strings.Join(append([]string{cmd}, args...), " ")
	if dryrun {
		log.Printf("%s", cli)
		return -1, nil
	}
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
