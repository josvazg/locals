package cmds

import (
	"fmt"
	"locals/internal/cfg"
	"locals/internal/dnsctl"
	"locals/internal/mkcert"
	"locals/internal/platform"
	"locals/internal/service"
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func offCmd(p platform.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	var wipe bool
	cmd := &cobra.Command{
		Use:   "off",
		Short: "Deactivate locals' web proxy and restore default DNS config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if dryrun {
				log.Printf("DRYRUN")
				p = platform.NewDryrunPlatform(p)
			}
			config, err := newConfig(p, "", localsDir)
			if err != nil {
				return fmt.Errorf("failed to setup off config: %w", err)
			}
			return off(p, config, wipe, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	cmd.Flags().BoolVarP(&wipe, "wipe", "", false, "wipe the endpoints configured as well")
	return cmd
}

func off(p platform.Platform, config *cfg.Config, wipe, dryrun bool) error {
	qual := ""
	if dryrun {
		qual = "dryrun "
	}
	if err := dnsctl.NewDNSController(p, config).Release(); err != nil {
		return fmt.Errorf("failed to %sunconfigure DNS config: %w", qual, err)
	}
	svctl := service.New(config.LocalsDir, config.TempDir, p.Env("PATH"), p.Stdout())
	if err := stopService(svctl, "dns"); err != nil {
		return fmt.Errorf("failed to %sstop embedded DNS server: %w", qual, err)
	}
	if err := stopService(svctl, "web"); err != nil {
		return fmt.Errorf("failed to %sstop embedded web server: %w", qual, err)
	}
	if err := uninstallMkcert(p); err != nil {
		return fmt.Errorf("failed to %sdisable mkcert: %w", qual, err)
	}
	if wipe {
		serviceFiles, err := p.FS().ListFiles(filepath.Join(config.LocalsDir, "web", "*.json"))
		if err != nil {
			return fmt.Errorf("failed to list cert files: %w", err)
		}
		if err := p.FS().RemoveFiles(serviceFiles...); err != nil {
			return fmt.Errorf("failed to %swipe endpoint configs: %w", qual, err)
		}
		log.Printf("Wiped %d service files", len(serviceFiles))
		certFiles, err := p.FS().ListFiles(filepath.Join(config.LocalsDir, "certs", "*.pem"))
		if err != nil {
			return fmt.Errorf("failed to list cert files: %w", err)
		}
		if err := p.FS().RemoveFiles(certFiles...); err != nil {
			return fmt.Errorf("failed to %swipe service certificates: %w", qual, err)
		}
		log.Printf("Wiped %d cert files", len(certFiles))
	}
	return nil
}

func stopService(svctl service.Control, service string) error {
	if err := svctl.Stop(service); err != nil {
		return fmt.Errorf("failed to stop service %v: %w", service, err)
	}
	return nil
}

func uninstallMkcert(p platform.Platform) error {
	if err := mkcert.New(p.Stdout()).Uninstall(); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "item could not be found in the keychain") ||
			strings.Contains(errMsg, "not installed") {
			return nil
		}
		return fmt.Errorf("failed to uninstall mkcert: %w", err)
	}
	return nil
}
