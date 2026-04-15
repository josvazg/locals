package cmds

import (
	"fmt"
	"locals/internal/platform"
	"log"
	"path/filepath"

	"github.com/spf13/cobra"
)

func rmCmd(p platform.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	cmd := &cobra.Command{
		Use:   "rm service",
		Short: "Remove HTTPs access to an endpoint",
		Long: "Remove routing and certificate files for a .locals hostname.\n" +
			"Example — stop serving whoami.locals:\n\n" +
			"  locals rm whoami.locals",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			domain := args[0]
			if dryrun {
				log.Printf("DRYRUN")
				p = platform.NewDryrunPlatform(p)
			}
			return rm(p, domain, localsDir)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func rm(p platform.Platform, domain, localsDir string) error {
	domainCfgFile := filepath.Join(localsDir, "web", fmt.Sprintf("%s.json", domain))
	if err := p.IO().RemoveFiles(domainCfgFile); err != nil {
		return fmt.Errorf("failed to remove domain %s config file: %w", domain, err)
	}
	certFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s.pem", domain))
	keyFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s-key.pem", domain))
	if err := p.IO().RemoveFiles(certFile, keyFile); err != nil {
		return fmt.Errorf("failed to remove domain %s keys and certificates: %w", domain, err)
	}
	log.Printf("⏹️ Removed access to %s", domain)
	return nil
}
