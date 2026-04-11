package cmds

import (
	"fmt"
	"locals/internal/platform"
	"strings"
)

func find(p platform.Platform, filename, pattern string) (bool, error) {
	contents, err := p.IO().ReadFile(filename)
	if err != nil {
		return false, fmt.Errorf("failed to read file %q: %v", filename, contents)
	}
	return strings.Contains(string(contents), pattern), nil
}
