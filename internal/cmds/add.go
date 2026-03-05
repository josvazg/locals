package cmds

import (
	"fmt"
	"path/filepath"

	"locals/api/locals"
	"locals/internal/render"

	"github.com/spf13/cobra"
)

func addCmd(p *locals.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "add service endpoint",
		Short: "Add access to an HTTPs endpoint",
		Long: "Add a service url such as locals, such as:\n" +
			"$ add https://whoami.locals localhost:8080",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			targetURL := args[1]
			return add(p, localsDir, domain, targetURL)
		},
	}
}

func add(p *locals.Platform, localsDir, domain, targetURL string) error {
	rs := render.State{
		LocalsDir: localsDir,
		SystemCA:  p.Env.SystemCA(),
	}
	addCode, err := render.Add(rs)
	if err != nil {
		return fmt.Errorf("failed to render the add script: %w", err)
	}
	addScript := &script{name: filepath.Join(localsDir, "add.sh"), contents: addCode}
	if err := save(p, addScript); err != nil {
		return fmt.Errorf("failed to save the add script: %w", err)
	}
	if dryrun {
		return show(addScript, domain, targetURL)
	}
	return run(p, addScript, domain, targetURL)
}
