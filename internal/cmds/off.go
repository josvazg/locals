package cmds

import (
	"fmt"
	"locals/internal/platform"
	"log"
	"path/filepath"
	"strconv"
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
			return off(p, localsDir, wipe, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	cmd.Flags().BoolVarP(&wipe, "wipe", "", false, "wipe the endpoints configured as well")
	return cmd
}

func off(p platform.Platform, localsDir string, wipe, dryrun bool) error {
	cfg := Config{
		DNSListen: platform.DefaultDNSListen,
		LocalsDir: localsDir,
		SystemCA:  platform.SystemCA(p),
	}
	qual := ""
	if dryrun {
		qual = "dryrun "
	}
	if err := unconfigureDNS(p, &cfg); err != nil {
		return fmt.Errorf("failed to %sunconfigure DNS config: %w", qual, err)
	}
	if err := stopService(p, "dns", &cfg); err != nil {
		return fmt.Errorf("failed to %sstop embedded DNS server: %w", qual, err)
	}
	if err := stopService(p, "web", &cfg); err != nil {
		return fmt.Errorf("failed to %sstop embedded web server: %w", qual, err)
	}
	if err := uninstallMkcert(p); err != nil {
		return fmt.Errorf("failed to %sdisable mkcert: %w", qual, err)
	}
	if wipe {
		serviceFiles, err := p.IO().ListFiles(filepath.Join(localsDir, "web", "*.json"))
		if err != nil {
			return fmt.Errorf("failed to list cert files: %w", err)
		}
		if err := p.IO().RemoveFiles(serviceFiles...); err != nil {
			return fmt.Errorf("failed to %swipe endpoint configs: %w", qual, err)
		}
		log.Printf("Wiped %d service files", len(serviceFiles))
		certFiles, err := p.IO().ListFiles(filepath.Join(localsDir, "certs", "*.pem"))
		if err != nil {
			return fmt.Errorf("failed to list cert files: %w", err)
		}
		if err := p.IO().RemoveFiles(certFiles...); err != nil {
			return fmt.Errorf("failed to %swipe service certificates: %w", qual, err)
		}
		log.Printf("Wiped %d cert files", len(certFiles))
	}
	return nil
}

func stopService(p platform.Platform, service string, cfg *Config) error {
	pidFile := filepath.Join(cfg.LocalsDir, fmt.Sprintf("%s.pid", service))
	pid := readPIDFromFile(p, pidFile)
	if pid < 0 {
		log.Printf("ℹ️ No %s PID file found. Nothing to kill.", service)
		return nil
	}

	if p.Proc().IsProcessAlive(pid) {
		if _, err := p.Proc().Run("sudo", "kill", strconv.Itoa(pid)); err != nil {
			return fmt.Errorf("failed to stop locals %s (pid %d): %w", service, pid, err)
		}
		log.Printf("🛑 Terminated locals %s (PID: %d)", service, pid)
	} else {
		log.Printf("⚠️ PID file exists but process %d is already dead.", pid)
	}

	if _, err := p.Proc().Run("rm", pidFile); err != nil {
		return fmt.Errorf("failed to remove %s PID file %q: %w", service, pidFile, err)
	}
	return nil
}

func uninstallMkcert(p platform.Platform) error {
	if _, err := p.Proc().Run("mkcert", "-uninstall"); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "item could not be found in the keychain") ||
			strings.Contains(errMsg, "not installed") {
			return nil
		}
		return fmt.Errorf("failed to uninstall mkcert: %w", err)
	}
	return nil
}
