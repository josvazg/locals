package cmds

import (
	"fmt"
	"path/filepath"

	"locals/api/locals"
	"locals/internal/render"

	"github.com/spf13/cobra"
)

func onCmd(p *locals.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "on",
		Short: "Start the web proxy and grab DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			return on(p, localsDir, dryrun)
		},
	}
}

func on(p *locals.Platform, localsDir string, dryrun bool) error {
	state := render.State{
		DNSListen: locals.DefaultDNSListen,
		LocalsDir: localsDir,
		SystemCA:  p.Env.SystemCA(),
	}
	onScriptCode, err := render.On(state)
	if err != nil {
		return fmt.Errorf("failed to render the on script: %w", err)
	}
	onScript := &script{name: filepath.Join(localsDir, "on.sh"), contents: onScriptCode}
	if err := save(p, onScript); err != nil {
		return fmt.Errorf("failed to save on.sh script: %w", err)
	}
	if dryrun {
		return show(onScript)
	}
	return run(p, onScript)
}
