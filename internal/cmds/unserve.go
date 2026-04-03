package cmds

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/spf13/cobra"
)

func unserveCmd(localsDir string) *cobra.Command {
	var dryrun bool
	cmd := &cobra.Command{
		Use:   "unserve service",
		Short: "Unserve HTTPs from an endpoint",
		Long: "Unserve HTTPs from an endpoint by service URL. Eg:\n" +
			"$ unserve whoami.service localhost:8009",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			domain := args[0]
			return unserve(dryrun, domain, localsDir)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func unserve(dryrun bool, domain, localsDir string) error {
	domainCfgFile := filepath.Join(localsDir, "web", fmt.Sprintf("%s.json", domain))
	if err := run(dryrun, "rm", "-rf", domainCfgFile); err != nil {
		return fmt.Errorf("failed to remove domain %s config file: %w", domain, err)
	}
	certFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s.pem", domain))
	keyFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s-key.pem", domain))
	if err := run(dryrun, "rm", "-rf", certFile, keyFile); err != nil {
		return fmt.Errorf("failed to remove domain %s keys and certificates: %w", domain, err)
	}
	log.Printf("▶️ Removed access to %s", domain)
	return nil
}
