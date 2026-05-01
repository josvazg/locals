package cmds

import (
	"context"
	"fmt"
	"locals/internal/web"
	"log"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	DefaultConfigDir = "web"

	// Localhost only (IPv4 loopback; matches *.locals A records). Using 127.0.0.1 avoids :443 binding all interfaces.
	ListenAddr = "127.0.0.1:443"
)

func webCmd(ctx context.Context, cfgDir string) *cobra.Command {
	var logFile string
	cmd := &cobra.Command{
		Use:   "web [configDir]",
		Short: "Run the locals Web reverse proxy service",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := log.Default()
			if logFile != "" {
				f, err := loggerFile(logFile)
				if err != nil {
					return fmt.Errorf("failed to setup log file: %w", err)
				}
				logger = log.New(f, "", log.LstdFlags)
				defer f.Close()
			}
			webDir := DefaultConfigDir
			if len(args) > 0 {
				webDir = args[0]
			}
			cmd.SilenceUsage = true
			return web.New(ListenAddr, ensureAbsolutePath(webDir, cfgDir), logger).Run(ctx)
		},
	}
	cmd.Flags().StringVarP(&logFile, "log", "", "", "file to log to")
	return cmd
}

func ensureAbsolutePath(dir, cfgDir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(cfgDir, filepath.Clean(dir))
}
