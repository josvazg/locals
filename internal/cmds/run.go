package cmds

import (
	"fmt"
	"path/filepath"

	"locals/api/locals"
)

func run(p *locals.Platform, script *script, args ...string) error {
	scriptName := filepath.Base(script.name)
	if err := p.Execute(script.name, args...); err != nil {
		return fmt.Errorf("failed to run %s script: %w", scriptName, err)
	}
	return nil
}
