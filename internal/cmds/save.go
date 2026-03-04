package cmds

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type script struct {
	name string
	contents []byte
}

func save(script* script) error {
	scriptName := filepath.Base(script.name)
	if err := os.Remove(script.name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove %s script to rewrite it: %w", scriptName, err)
	}
	if err := os.WriteFile(script.name, script.contents, 0750); err != nil {
		return fmt.Errorf("failed to save %s script: %w", scriptName, err)
	}
	return nil
}
