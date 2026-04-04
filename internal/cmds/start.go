package cmds

import (
	"fmt"
	"locals/api/locals"
	"locals/internal/platform"
	"locals/internal/render"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	probeRetries = 5

	probePause = 1 * time.Second

	resolverMacConfigDir = "/etc/resolver"

	resolverMacLocalsFile = "/etc/resolver/locals"
)

func startCmd(p *locals.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the web proxy and grab DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return start(p, localsDir, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func start(p *locals.Platform, localsDir string, dryrun bool) error {
	state := render.State{
		DNSListen: locals.DefaultDNSListen,
		LocalsDir: localsDir,
		LocalsBin: localsBinary(),
		SystemCA:  p.Env.SystemCA(),
	}
	qual := ""
	if dryrun {
		qual = "dryrun "
	}
	if err := installMkcert(dryrun); err != nil {
		return fmt.Errorf("failed to %sinstall mkcert: %w", qual, err)
	}
	if err := launchDNS(&state, dryrun); err != nil {
		return fmt.Errorf("failed to %slaunch embedded DNS server: %w", qual, err)
	}
	if err := configureDNS(&state, dryrun); err != nil {
		return fmt.Errorf("failed to %sconfigure system DNS: %w", qual, err)
	}
	if err := launchWeb(&state, dryrun); err != nil {
		return fmt.Errorf("failed to %slaunch embedded Web server: %w", qual, err)
	}
	if dryrun {
		return nil
	}
	return probeServices(state.DNSListen, ":443")
}

func localsBinary() string {
	return os.Args[0]
}

func installMkcert(dryrun bool) error {
	if err := run(dryrun, "mkcert", "-install"); err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}
	log.Printf("For CLI usage run:\nsource <(locals env)")
	return nil
}

func launchDNS(state *render.State, dryrun bool) error {
	pidFile := filepath.Join(state.LocalsDir, "dns.pid")
	if pid := readPIDFromFile(pidFile); pid >= 0 {
		if processExistsForPID(pid) {
			fmt.Printf("⚠️ locals dns is already running (PID: %d). Skipping start.\n", pid)
			return nil
		}
		fmt.Println("🔄 Cleaning up stale PID file from previous crash...")
		if err := os.Remove(pidFile); err != nil {
			return fmt.Errorf("failed to remove PID file %q: %w", pidFile, err)
		}
	}
	pid, err := launch(dryrun, "sudo", "nohup", state.LocalsBin,
		"dns", state.DNSListen,
		"--log", filepath.Join(os.TempDir(), "locals-dns.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded DNS server: %w", err)
	}
	if err := os.WriteFile(pidFile, ([]byte)(fmt.Sprintf("%d", pid)), 0640); err != nil {
		return fmt.Errorf("failed to write the embedded DNS server PID file: %w", err)
	}
	log.Printf("✅ locals DNS started on %s (PID: %d))", state.DNSListen, pid)
	return nil
}

func configureDNS(state *render.State, dryrun bool) error {
	if runtime.GOOS == "darwin" {
		return configureMacDNS(state, dryrun)
	}
	panic("unsupported")
}

func configureMacDNS(state *render.State, dryrun bool) error {
	if !platform.IsIPOnInterface("lo0", state.DNSListen) {
		err := run(dryrun, "sudo", "ifconfig", "lo0", "alias",
			state.DNSListen, "netmask", "255.255.255.255")
		if err != nil {
			return fmt.Errorf("failed to set lo0 DNS redirect: %w", err)
		}
	}
	if err := run(dryrun, "mkdir", "-p", resolverMacConfigDir); err != nil {
		return fmt.Errorf("failed to mkdir the resovler dir: %w", err)
	}
	resolverCfg := fmt.Sprintf("nameserver %s\nport 53\n", state.DNSListen)
	if err := heredoc(dryrun, resolverCfg, resolverMacLocalsFile); err != nil {
		return fmt.Errorf("failed to write resolver config: %w", err)
	}
	return nil
}

func launchWeb(state *render.State, dryrun bool) error {
	pidFile := filepath.Join(state.LocalsDir, "web.pid")
	if pid := readPIDFromFile(pidFile); pid >= 0 {
		if processExistsForPID(pid) {
			fmt.Printf("⚠️ locals web is already running (PID: %d). Skipping start.\n", pid)
			return nil
		}
		fmt.Println("🔄 Cleaning up stale PID file from previous crash...")
		if err := os.Remove(pidFile); err != nil {
			return fmt.Errorf("failed to remove PID file %q: %w", pidFile, err)
		}
	}
	pid, err := launch(dryrun, "sudo", "nohup", state.LocalsBin,
		"web", "--log", filepath.Join(os.TempDir(), "locals-web.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded web server: %w", err)
	}
	if err := os.WriteFile(pidFile, ([]byte)(fmt.Sprintf("%d", pid)), 0640); err != nil {
		return fmt.Errorf("failed to write the embedded web server PID file: %w", err)
	}
	log.Printf("✅ locals web started on %s (PID: %d))", state.DNSListen, pid)
	return nil
}

func readPIDFromFile(pidFile string) int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return -1
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return -1
	}
	return pid
}

func processExistsForPID(pid int) bool {
	process, _ := os.FindProcess(pid)
	err := process.Signal(syscall.Signal(0))
	return err == nil
}
