package cmds

import (
	"errors"
	"fmt"
	"io/fs"
	"locals/api/locals"
	"path/filepath"
)

type script struct {
	name     string
	contents []byte
}

func save(p *locals.Platform, script *script) error {
	scriptName := filepath.Base(script.name)
	if err := p.IO.Remove(script.name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to remove %s script to rewrite it: %w", scriptName, err)
	}
	if err := p.IO.WriteFile(script.name, script.contents, 0750); err != nil {
		return fmt.Errorf("failed to save %s script: %w", scriptName, err)
	}
	return nil
}
