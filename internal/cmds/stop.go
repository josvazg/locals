package cmds

import (
	"fmt"
	"locals/api/locals"
	"locals/internal/platform"
	"locals/internal/render"
	"log"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func stopCmd(p *locals.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	var wipe bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the web proxy and restore DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return stop(p, localsDir, dryrun, wipe)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	cmd.Flags().BoolVarP(&wipe, "wipe", "", false, "wipe the endpoints configured as well")
	return cmd
}

func stop(p *locals.Platform, localsDir string, dryrun, wipe bool) error {
	state := render.State{
		DNSListen: locals.DefaultDNSListen,
		LocalsDir: localsDir,
		SystemCA:  p.Env.SystemCA(),
	}
	qual := ""
	if dryrun {
		qual = "dryrun "
	}
	if err := unconfigureDNS(&state, dryrun); err != nil {
		return fmt.Errorf("failed to %sunconfigure DNS config: %w", qual, err)
	}
	if err := stopService("dns", &state, dryrun); err != nil {
		return fmt.Errorf("failed to %sstop embedded DNS server: %w", qual, err)
	}
	if err := stopService("web", &state, dryrun); err != nil {
		return fmt.Errorf("failed to %sstop embedded web server: %w", qual, err)
	}
	if err := uninstallMkcert(dryrun); err != nil {
		return fmt.Errorf("failed to %sinstall mkcert: %w", qual, err)
	}
	if wipe {
		if err := run(dryrun, "rm", filepath.Join(localsDir, "web", "*.json")); err != nil {
			return fmt.Errorf("failed to %swipe endpoint configs: %w", qual, err)
		}
	}
	return nil
}

func unconfigureDNS(state *render.State, dryrun bool) error {
	if runtime.GOOS == "darwin" {
		return unconfigureMacDNS(state, dryrun)
	}
	panic("unsupported")
}

func unconfigureMacDNS(state *render.State, dryrun bool) error {
	if platform.IsIPOnInterface("lo0", state.DNSListen) {
		if err := run(dryrun, "sudo", "ifconfig", "lo0", "-alias", state.DNSListen); err != nil {
			return fmt.Errorf("failed to remove lo0 DNS redirect: %w", err)
		}
	}
	if strings.HasSuffix(strings.TrimSpace(resolverMacLocalsFile), "/") {
		panic(fmt.Sprintf("dangerous resolverMacLocalsFile value: %s", resolverMacConfigDir))
	}
	if err := run(dryrun, "sudo", "rm", "-f", resolverMacLocalsFile); err != nil {
		return fmt.Errorf("failed to remove locals resolver file %q: %w",
			resolverMacLocalsFile, err)
	}
	return nil
}

func stopService(service string, state *render.State, dryrun bool) error {
	pidFile := filepath.Join(state.LocalsDir, fmt.Sprintf("%s.pid", service))
	if pid := readPIDFromFile(pidFile); pid >= 0 {
		if processExistsForPID(pid) {
			run(dryrun, "kill", strconv.Itoa(pid))
			log.Printf("🛑 Terminated locals %s (PID: %d)", service, pid)
		} else {
			log.Printf("⚠️ PID file exists but process %d is already dead.", pid)
		}
		run(dryrun, "rm", pidFile)
	} else {
		log.Printf("ℹ️ No %s PID file found. Nothing to kill.", service)
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
