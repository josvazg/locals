package cmds

import (
	"context"
	"fmt"
	"path/filepath"

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

func Run(ctx context.Context, p *locals.Platform, args []string) error {
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

	on := onCmd(p, cfgDir)
	off := offCmd(p, cfgDir)
	add := addCmd(p, cfgDir)
	rm := rmCmd(p, cfgDir)
	start := startCmd(p, cfgDir)
	stop := stopCmd(p, cfgDir)
	serve := serveCmd(cfgDir)
	for _, c := range []*cobra.Command{on, off, add, rm} {
		c.Flags().BoolVarP(&dryrun, "dryrun", "", false,
			"show locals script but do not run them")
	}
	rootCmd.AddCommand(
		on,
		off,
		add,
		rm,
		dnsCmd(ctx, p),
		webCmd(ctx, p, cfgDir),
		statusCmd(p, cfgDir),
		envCmd(),
		start,
		stop,
		serve,
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
	dirs := []string{locals.WebDir, locals.CertsDir}

	for _, d := range dirs {
		path := filepath.Join(cfgDir, d)
		if err := p.IO.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create config subdir: %w", err)
		}
	}
	return nil
}
