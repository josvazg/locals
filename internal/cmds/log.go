package cmds

import (
	"fmt"
	"log"
	"os"
)

func setupLog(logFile string) error {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	// Redirect standard output and error for the rest of the process
	os.Stdout = f
	os.Stderr = f
	log.SetOutput(f) // If you use the standard logger
	return nil
}
