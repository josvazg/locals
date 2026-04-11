package cmds

import (
	"fmt"
	"locals/internal/platform"
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

func addCmd(p platform.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	cmd := &cobra.Command{
		Use:   "add service endpoint",
		Short: "Add access to an HTTPs endpoint",
		Long: "Add an HTTPS redirect from a .locals hostname to a backend (host:port or URL with scheme).\n" +
			"Example — serve whoami.locals via localhost:8080:\n\n" +
			"  locals add whoami.locals localhost:8080",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			domain := args[0]
			targetURL := args[1]
			if dryrun {
				log.Printf("DRYRUN")
				p = platform.NewDryrunPlatform(p)
			}
			return add(p, localsDir, domain, targetURL)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func add(p platform.Platform, localsDir, domain, targetURL string) error {
	certFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s.pem", domain))
	keyFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s-key.pem", domain))
	out, err := p.Proc().Run("mkcert",
		"-cert-file", certFile, "-key-file", keyFile,
		domain, "*.locals", "localhost", "127.0.0.1")
	if err != nil {
		return fmt.Errorf("failed to setup certificates for domain %s: %w", domain, err)
	}
	log.Print(out)
	domainCfgFile := filepath.Join(localsDir, "web", fmt.Sprintf("%s.json", domain))
	domainCfgJSON := fmt.Sprintf(domainConfig, domain, targetURL, certFile, keyFile)
	if err := p.IO().CreateFile(domainCfgFile, domainCfgJSON); err != nil {
		return fmt.Errorf("failed to setup web redirection for domain %s: %w", domain, err)
	}
	log.Printf("▶️ Added access to %s -> %s", domain, targetURL)
	return nil
}
