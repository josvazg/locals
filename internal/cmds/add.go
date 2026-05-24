package cmds

import (
	"crypto/tls"
	"fmt"
	"locals/internal/mkcert"
	"locals/internal/platform"
	"log"
	"net"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

const (
	serviceConfig = `{
  "url": "%s",
  "endpoint": "%s",
  "cert": "%s",
  "key": "%s"
}`

	pipeConfig = `{
  "url": "%s",
  "endpoint": "%s"
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
	if IsHTTPS(targetURL) {
		return addTCPPipe(p, localsDir, domain, targetURL)
	}
	return addTLSService(p, localsDir, domain, targetURL)
}

func addTLSService(p platform.Platform, localsDir, domain, targetURL string) error {
	certFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s.pem", domain))
	keyFile := filepath.Join(localsDir, "certs", fmt.Sprintf("%s-key.pem", domain))
	err := mkcert.New(p.Stdout()).Generate(
		"-cert-file", certFile, "-key-file", keyFile,
		domain, "*.locals", "localhost", "127.0.0.1")
	if err != nil {
		return fmt.Errorf("failed to setup certificates for domain %s: %w", domain, err)
	}
	domainCfgFile := filepath.Join(localsDir, "web", fmt.Sprintf("%s.json", domain))
	domainCfgJSON := fmt.Sprintf(serviceConfig, domain, targetURL, certFile, keyFile)
	if err := p.FS().CreateFile(domainCfgFile, domainCfgJSON); err != nil {
		return fmt.Errorf("failed to setup HTTPS redirection for domain %s: %w", domain, err)
	}
	log.Printf("▶️ Added web TLS access to %s -> %s", domain, targetURL)
	return nil
}

func addTCPPipe(p platform.Platform, localsDir, domain, targetURL string) error {
	domainCfgFile := filepath.Join(localsDir, "web", fmt.Sprintf("%s.json", domain))
	domainCfgJSON := fmt.Sprintf(pipeConfig, domain, targetURL)
	if err := p.FS().CreateFile(domainCfgFile, domainCfgJSON); err != nil {
		return fmt.Errorf("failed to setup HTTP redirection for domain %s: %w", domain, err)
	}
	log.Printf("▶️ Added web TCP pipe access to %s -> %s", domain, targetURL)
	return nil
}

// IsHTTPS checks if a target address (e.g., "localhost:8443") is serving TLS.
func IsHTTPS(address string) bool {
	dialer := &net.Dialer{Timeout: 2 * time.Second}

	config := &tls.Config{InsecureSkipVerify: true}

	conn, err := tls.DialWithDialer(dialer, "tcp", address, config)
	if err != nil {
		return false
	}
	defer conn.Close()

	return conn.ConnectionState().HandshakeComplete
}
