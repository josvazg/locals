package cmds

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"locals/api/locals"
)

func runScript(p *locals.Platform, script *script, args ...string) error {
	scriptName := filepath.Base(script.name)
	if err := p.Execute(script.name, args...); err != nil {
		return fmt.Errorf("failed to run %s script: %w", scriptName, err)
	}
	return nil
}

func run(dryrun bool, cmd string, args ...string) error {
	cli := strings.Join(append([]string{cmd}, args...), " ")
	if dryrun {
		log.Printf("%s", cli)
		return nil
	}
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

func heredoc(dryrun bool, heredoc, filename string) error {
	if dryrun {
		log.Printf("sudo tee \"%s\" > /dev/null <<EOF\n%s\nEOF", filename, heredoc)
	}
	cmd := exec.Command("sudo", "tee", filename)
	cmd.Stdin = strings.NewReader(heredoc)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func launch(dryrun bool, cmd string, args ...string) (int, error) {
	cli := strings.Join(append([]string{cmd}, args...), " ")
	if dryrun {
		log.Printf("%s", cli)
	}
	command := exec.Command(cmd, args...)
	if err := command.Start(); err != nil {
		return -1, fmt.Errorf("failed to launch %q: %w", cli, err)
	}
	return command.Process.Pid, nil
}
