package cmds

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"locals/api/locals"

	"github.com/spf13/cobra"
)

var dryrun bool

// --- DATA STRUCTURES (Traefik YAML) ---
type TraefikConfig struct {
	HTTP HTTPConfig `yaml:"http"`
}

type HTTPConfig struct {
	Routers  map[string]Router  `yaml:"routers"`
	Services map[string]Service `yaml:"services"`
}

type Router struct {
	Rule    string     `yaml:"rule"`
	Service string     `yaml:"service"`
	TLS     *TLSConfig `yaml:"tls,omitempty"`
}

type TLSConfig struct {
	Domains []Domain `yaml:"domains"`
}

type Domain struct {
	Main string   `yaml:"main"`
	SANs []string `yaml:"sans,omitempty"`
}

type Service struct {
	LoadBalancer LoadBalancer `yaml:"loadBalancer"`
}

type LoadBalancer struct {
	Servers []Server `yaml:"servers"`
}

type Server struct {
	URL string `yaml:"url"`
}

func Run(p *locals.Platform, args []string) error {
	cfgDir, err := localsDir(p)
	if err != nil {
		return fmt.Errorf("failed to compute config dir location: %w", err)
	}
	if err := initFileSystem(p, cfgDir); err != nil {
		return fmt.Errorf("failed to create config state: %w", err)
	}

	rootCmd := &cobra.Command{
		Use: "locals",
		// This prevents Cobra from printing errors to os.Stderr automatically
		// allowing you to handle them via your Platform.Stderr instead.
		SilenceErrors: true,
		SilenceUsage:  false,
	}
	rootCmd.SetOut(p.Stdout)
	rootCmd.SetErr(p.Stderr)
	rootCmd.SetArgs(args)

	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	on := onCmd(p, cfgDir)
	off := offCmd(p, cfgDir)
	add := addCmd(p, cfgDir)
	rm := rmCmd(p, cfgDir)
	for _, c := range []*cobra.Command{on, off, add, rm} {
		c.Flags().BoolVarP(&dryrun, "dryrun", "", false,
			"show locals script but do not run them")
	}
	rootCmd.AddCommand(
		on,
		off,
		add,
		rm,
		listCmd(p, cfgDir),
		dnsCmd(ctx),
		webCmd(ctx, cfgDir),
		statusCmd(p, cfgDir),
		envCmd(),
	)
	return rootCmd.Execute()
}

func localsDir(p *locals.Platform) (string, error) {
	localsDir := p.Env(locals.ENV_VAR_LOCALS_CONFIG_DIR)
	if localsDir != "" {
		return localsDir, nil
	}
	homeDir, err := p.HomeDir()
	if err != nil && p.Env(locals.ENV_VAR_LOCALS_CONFIG_DIR) == "" {
		return "", fmt.Errorf("locals failed : %w", err)
	}
	return filepath.Join(homeDir, locals.DirName), nil
}

// initFileSystem creates ~/.config/locals directories
func initFileSystem(p *locals.Platform, cfgDir string) error {
	dirs := []string{locals.WebDir, locals.CertsDir, locals.LogsDir}

	for _, d := range dirs {
		path := filepath.Join(cfgDir, d)
		if _, err := p.IO.Stat(path); os.IsNotExist(err) {
			if err := p.IO.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("failed to create config subdir: %w", err)
			}
		}
	}
	return nil
}
