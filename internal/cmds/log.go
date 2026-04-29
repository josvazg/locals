package cmds

import (
	"fmt"
	"locals/internal/platform"
	"log"
)

func setupLog(p platform.Platform, logFile string) error {
	f, err := p.FS().AppendTo(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	// Redirect standard output and error for the rest of the process
	p.SetStdout(f)
	p.SetStderr(f)
	log.SetOutput(f) // If you use the standard logger
	return nil
}
