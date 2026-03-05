package cmds

import (
	"fmt"
	"path/filepath"

	"locals/api/locals"
	"locals/internal/render"

	"github.com/spf13/cobra"
)

func offCmd(p *locals.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Stop the web proxy and restore DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			return off(p, localsDir)
		},
	}
}

func off(p *locals.Platform, localsDir string) error {
	state := render.State{
		DNSListen: DefaultDNSListen,
		LocalsDir: localsDir,
		SystemCA:  p.Env.SystemCA(),
	}
	offScriptCode, err := render.Off(state)
	if err != nil {
		return fmt.Errorf("failed to render the off script: %w", err)
	}
	offScript := &script{name: filepath.Join(localsDir, "off.sh"), contents: offScriptCode}
	if err := save(p, offScript); err != nil {
		return fmt.Errorf("failed to save off.sh script: %w", err)
	}
	if dryrun {
		return show(offScript)
	}
	return run(p, offScript)
}
