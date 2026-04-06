package cmds

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	domainConfig = `{
  "url": "%s",
  "endpoint": "%s",
  "cert": "%s",
  "key": "%s"
}`
)

func addCmd(localsDir string) *cobra.Command {
	var dryrun bool
	cmd := &cobra.Command{
		Use:   "add service endpoint",
		Short: "Add access to an HTTPs endpoint",
		Long: "Add an HTTPs service URL redirect to a custom endpoint\n" +
			"Eg. add https://whoami.locals:\n" +
			"$ add https://whoami.locals localhost:8080",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			domain := args[0]
			targetURL := args[1]
			if dryrun {
				log.Printf("DRYRUN")
			}
			return add(localsDir, domain, targetURL, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func add(localsDir, domain, targetURL string, dryrun bool) error {
	certFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s.pem", domain))
	keyFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s-key.pem", domain))
	if err := run(dryrun, "mkcert",
		"-cert-file", certFile, "-key-file", keyFile,
		domain, "*.locals", "localhost", "127.0.0.1"); err != nil {
		return fmt.Errorf("failed to setup certificates for domain %s: %w", domain, err)
	}
	domainCfgFile := filepath.Join(localsDir, "web", fmt.Sprintf("%s.json", domain))
	domainCfgJSON := fmt.Sprintf(domainConfig, domain, targetURL, certFile, keyFile)
	if err := heredoc(dryrun, domainCfgJSON, domainCfgFile); err != nil {
		return fmt.Errorf("failed to setup web redirection for domain %s: %w", domain, err)
	}
	log.Printf("▶️ Added access to %s -> %s", domain, targetURL)
	return nil
}
