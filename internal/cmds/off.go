package cmds

import (
	"fmt"
	"locals/internal/platform"
	"log"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func offCmd(p *platform.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	var wipe bool
	cmd := &cobra.Command{
		Use:   "off",
		Short: "Deactivate locals' web proxy and restore default DNS config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if dryrun {
				log.Printf("DRYRUN")
			}
			return off(p, localsDir, dryrun, wipe)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	cmd.Flags().BoolVarP(&wipe, "wipe", "", false, "wipe the endpoints configured as well")
	return cmd
}

func off(p *platform.Platform, localsDir string, dryrun, wipe bool) error {
	cfg := Config{
		DNSListen: platform.DefaultDNSListen,
		LocalsDir: localsDir,
		SystemCA:  p.Env.SystemCA(),
	}
	qual := ""
	if dryrun {
		qual = "dryrun "
	}
	if err := unconfigureDNS(&cfg, dryrun); err != nil {
		return fmt.Errorf("failed to %sunconfigure DNS config: %w", qual, err)
	}
	if err := stopService("dns", &cfg, dryrun); err != nil {
		return fmt.Errorf("failed to %sstop embedded DNS server: %w", qual, err)
	}
	if err := stopService("web", &cfg, dryrun); err != nil {
		return fmt.Errorf("failed to %sstop embedded web server: %w", qual, err)
	}
	if err := uninstallMkcert(dryrun); err != nil {
		return fmt.Errorf("failed to %sinstall mkcert: %w", qual, err)
	}
	if wipe {
		serviceFiles := filepath.Join(localsDir, "web", "*.json")
		if err := safeSudoRemoves(dryrun, serviceFiles); err != nil {
			return fmt.Errorf("failed to %swipe endpoint configs: %w", qual, err)
		}
		certFiles := filepath.Join(localsDir, "certs", "*.pem") 
		if err := safeSudoRemoves(dryrun, certFiles); err != nil {
			return fmt.Errorf("failed to %swipe service certificates: %w", qual, err)
		}
	}
	return nil
}

func unconfigureDNS(cfg *Config, dryrun bool) error {
	if runtime.GOOS == "darwin" {
		return unconfigureMacDNS(cfg, dryrun)
	}
	return unconfigureLinuxDNS(dryrun)
}

func unconfigureLinuxDNS(dryrun bool) error {
	if test("mountpoint", "-q", "/etc/resolv.conf") {
		if err := run(dryrun, "sudo", "umount", "/etc/resolv.conf"); err != nil {
			return fmt.Errorf("failed to undo mount bind on /etc/resolv.conf: %w", err)
		}
	} else {
		log.Printf("ℹ️ /etc/resolv.conf was not mounted.")
	}
	if err := safeSudoRemoves(dryrun, resolverConf); err != nil {
		return fmt.Errorf("failed to remove resolved config: %w", err)
	}
	return nil
}

func unconfigureMacDNS(state *Config, dryrun bool) error {
	if platform.IsIPOnInterface("lo0", state.DNSListen) {
		if err := run(dryrun, "sudo", "ifconfig", "lo0", "-alias", state.DNSListen); err != nil {
			return fmt.Errorf("failed to remove lo0 DNS redirect: %w", err)
		}
	}
	if err := safeSudoRemoves(dryrun, resolverMacLocalsFile); err != nil {
		return fmt.Errorf("failed to remove locals resolver file %q: %w",
			resolverMacLocalsFile, err)
	}
	return nil
}

func stopService(service string, cfg *Config, dryrun bool) error {
	pidFile := filepath.Join(cfg.LocalsDir, fmt.Sprintf("%s.pid", service))
	pid := readPIDFromFile(pidFile)
	if pid < 0 {
		log.Printf("ℹ️ No %s PID file found. Nothing to kill.", service)
		return nil
	}

	if processExistsForPID(pid) {
		if err := run(dryrun, "sudo", "kill", strconv.Itoa(pid)); err != nil {
			return fmt.Errorf("failed to stop locals %s (pid %d): %w", service, pid, err)
		}
		log.Printf("🛑 Terminated locals %s (PID: %d)", service, pid)
	} else {
		log.Printf("⚠️ PID file exists but process %d is already dead.", pid)
	}

	if err := run(dryrun, "rm", pidFile); err != nil {
		return fmt.Errorf("failed to remove %s PID file %q: %w", service, pidFile, err)
	}
	return nil
}

func uninstallMkcert(dryrun bool) error {
	if err := run(dryrun, "mkcert", "-uninstall"); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "item could not be found in the keychain") ||
			strings.Contains(errMsg, "not installed") {
			return nil
		}
		return fmt.Errorf("failed to uninstall mkcert: %w", err)
	}
	return nil
}
