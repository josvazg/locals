package cmds

import (
	"fmt"
	"path/filepath"

	"locals/api/locals"
	"locals/internal/render"

	"github.com/spf13/cobra"
)

func rmCmd(p *locals.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "rm service",
		Short: "Remove HTTPs access to an endpoint",
		Long: "Remove HTTPs access to an endpoint by service URL. Eg:\n" +
			"$ rm whoami.service localhost:8009",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			domain := args[0]
			return rm(p, localsDir, domain)
		},
	}
}

func rm(p *locals.Platform, localsDir, domain string) error {
	rs := render.State{
		LocalsDir: localsDir,
		SystemCA:  p.Env.SystemCA(),
	}
	rmCode, err := render.Remove(rs)
	if err != nil {
		return fmt.Errorf("failed to render the rm script: %w", err)
	}
	rmScript := &script{name: filepath.Join(localsDir, "rm.sh"), contents: rmCode}
	if err := save(p, rmScript); err != nil {
		return fmt.Errorf("failed to save the rm script: %w", err)
	}
	if dryrun {
		return show(rmScript, domain)
	}
	return run(p, rmScript, domain)
}
