package cmds

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func envCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Environment for CLI usage",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return env()
		},
	}
}

func env() error {
	caPath, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return fmt.Errorf("failed to mkcert -CAROOT: %w", err)
	}
	cleanPath := strings.TrimSpace(string(caPath)) + "/rootCA.pem"
	fmt.Printf("export SSL_CERT_FILE='%s'\n", cleanPath)
	fmt.Printf("export CURL_CA_BUNDLE='%s'\n", cleanPath)
	return nil
}
