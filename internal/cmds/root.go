package cmds

import (
	"context"
	"fmt"
	"path/filepath"

	"locals/internal/platform"

	"github.com/spf13/cobra"
)

var dryrun bool

func Run(ctx context.Context, p *platform.Platform, args []string) error {
	cfgDir, err := localsDir(p)
	if err != nil {
		return fmt.Errorf("failed to compute config dir location: %w", err)
	}
	if err := initFileSystem(p, cfgDir); err != nil {
		return fmt.Errorf("failed to create config state: %w", err)
	}

	rootCmd := &cobra.Command{
		Use: "locals",
		// This prevents Cobra from printing errors to std err automatically
		// allowing you to handle them via your Platform.Stderr instead.
		SilenceErrors: true,
		SilenceUsage:  false,
	}
	rootCmd.SetOut(p.Stdout)
	rootCmd.SetErr(p.Stderr)
	rootCmd.SetArgs(args)

	rootCmd.AddCommand(
		onCmd(p, cfgDir),
		offCmd(p, cfgDir),
		addCmd(cfgDir),
		rmCmd(cfgDir),
		dnsCmd(ctx, p),
		webCmd(ctx, p, cfgDir),
		statusCmd(p, cfgDir),
		envCmd(),
	)
	return rootCmd.Execute()
}

func localsDir(p *platform.Platform) (string, error) {
	localsDir := p.Env(platform.EnvLocalsConfigDir)
	if localsDir != "" {
		return localsDir, nil
	}
	homeDir, err := p.HomeDir()
	if err != nil && p.Env(platform.EnvLocalsConfigDir) == "" {
		return "", fmt.Errorf("locals failed : %w", err)
	}
	return filepath.Join(homeDir, platform.DirName), nil
}

// initFileSystem creates ~/.config/locals directories
func initFileSystem(p *platform.Platform, cfgDir string) error {
	dirs := []string{platform.WebDir, platform.CertsDir}

	for _, d := range dirs {
		path := filepath.Join(cfgDir, d)
		if err := p.IO.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create config subdir: %w", err)
		}
	}
	return nil
}
