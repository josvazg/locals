package cmds

import (
	"errors"
	"fmt"
	"locals/api/locals"
	"locals/internal/platform"
	"log"
	"net"
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

	resolverConf = "/etc/systemd/resolved.conf.d/locals.conf"
)

type Config struct {
	DNSListen string
	LocalsDir string
	SystemCA  string
	LocalsBin string
}

func onCmd(p *locals.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	cmd := &cobra.Command{
		Use:   "on",
		Short: "Activate locals by starting the web proxy and grab local DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if dryrun {
				log.Printf("DRYRUN")
			}
			return on(p, localsDir, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func on(p *locals.Platform, localsDir string, dryrun bool) error {
	localsBin, err := localsBinary()
	if err != nil {
		return fmt.Errorf("failed to resolve path to locals: %w", err)
	}
	state := Config{
		DNSListen: locals.DefaultDNSListen,
		LocalsDir: localsDir,
		LocalsBin: localsBin,
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
	return probeServices(state.DNSListen, ListenAddr)
}

func localsBinary() (string, error) {
	binary := os.Args[0]
	if pathExists(binary) {
		return binary, nil
	}
	binPath, err := readOutput("command", "-v", "locals")
	if err != nil {
		return "", fmt.Errorf("failed to find locals binary path: %w", err)
	}
	return binPath, nil
}

func installMkcert(dryrun bool) error {
	if err := run(dryrun, "mkcert", "-install"); err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}
	log.Printf("For CLI usage run:\nsource <(locals env)")
	return nil
}

func launchDNS(cfg *Config, dryrun bool) error {
	pidFile := filepath.Join(cfg.LocalsDir, "dns.pid")
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
	pid, err := launch(dryrun, "sudo", "nohup", cfg.LocalsBin,
		"dns", cfg.DNSListen,
		"--log", filepath.Join(os.TempDir(), "locals-dns.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded DNS server: %w", err)
	}
	if err := os.WriteFile(pidFile, ([]byte)(fmt.Sprintf("%d", pid)), 0640); err != nil {
		return fmt.Errorf("failed to write the embedded DNS server PID file: %w", err)
	}
	log.Printf("✅ locals DNS started on %s (PID: %d)", cfg.DNSListen, pid)
	return nil
}

func configureDNS(cfg *Config, dryrun bool) error {
	if runtime.GOOS == "darwin" {
		return configureMacDNS(cfg, dryrun)
	}
	return configureLinuxDNS(cfg, dryrun)
}

func configureMacDNS(cfg *Config, dryrun bool) error {
	if !platform.IsIPOnInterface("lo0", cfg.DNSListen) {
		err := run(dryrun, "sudo", "ifconfig", "lo0", "alias",
			cfg.DNSListen, "netmask", "255.255.255.255")
		if err != nil {
			return fmt.Errorf("failed to set lo0 DNS redirect: %w", err)
		}
	}
	resolverCfg := fmt.Sprintf("nameserver %s\nport 53\n", cfg.DNSListen)
	if err := heredoc(dryrun, resolverCfg, resolverMacLocalsFile); err != nil {
		return fmt.Errorf("failed to write resolver config: %w", err)
	}
	return nil
}

func configureLinuxDNS(cfg *Config, dryrun bool) error {
	if pathExists("/run/systemd/resolve") && test("systemctl", "is-active", "systemd-resolved") {
		return configureLinuxResolved(cfg, dryrun)
	}
	log.Printf("📡 systemd-resolved not found. Falling back to /etc/resolv.conf bind-mount.")
	return configureLinuxBindMount(cfg, dryrun)
}

func configureLinuxResolved(cfg *Config, dryrun bool) error {
	log.Printf("📡 systemd-resolved detected. Using Routing Domain setup.")
	localsResolvedCfg := fmt.Sprintf("[Resolve]\nDNS=%s\nDomains=~locals\n", cfg.DNSListen)
	if err := heredoc(dryrun, localsResolvedCfg, resolverConf); err != nil {
		return fmt.Errorf("failed to configure locals resolved: %w", err)
	}
	if err := run(dryrun, "sudo", "systemctl", "restart", "systemd-resolved"); err != nil {
		return fmt.Errorf("failed to restart systemd resolved: %w", err)
	}
	log.Printf("🔒 systemd-resolved configured to route .locals to %s", cfg.DNSListen)
	return nil
}

func configureLinuxBindMount(cfg *Config, dryrun bool) error {
	if test("mountpoint", "-q", "/etc/resolv.conf") {
		log.Printf("⚠️ /etc/resolv.conf already replaced. Skipping.")
		return nil
	}
	resolvConfLocal := filepath.Join(cfg.LocalsDir, "resolv.patched.conf")
	resolvCfg := fmt.Sprintf("nameserver %s\noptions edns0 trust-ad", cfg.DNSListen)
	if err := heredoc(dryrun, resolvCfg, resolvConfLocal); err != nil {
		return fmt.Errorf("failed to create alternate resolv.conf: %w", err)
	}
	if err := run(dryrun, "sudo", "mount", "--bind", resolvConfLocal, "/etc/resolv.conf"); err != nil {
		return fmt.Errorf("failed to bind mount /etc/resolv.conf: %w", err)
	}
	log.Printf("🔒 /etc/resolv.conf mounted to redirect DNS queries to locals dns first")
	return nil
}

func launchWeb(cfg *Config, dryrun bool) error {
	pidFile := filepath.Join(cfg.LocalsDir, "web.pid")
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
	webCfg := filepath.Join(cfg.LocalsDir, "web")
	pid, err := launch(dryrun, "sudo", "nohup", cfg.LocalsBin, "web",
		webCfg, "--log", filepath.Join(os.TempDir(), "locals-web.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded web server: %w", err)
	}
	if err := os.WriteFile(pidFile, ([]byte)(fmt.Sprintf("%d", pid)), 0640); err != nil {
		return fmt.Errorf("failed to write the embedded web server PID file: %w", err)
	}
	log.Printf("✅ locals web started on %s (PID: %d)", cfg.DNSListen, pid)
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

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func probeServices(dnsListen, webListen string) error {
	return errors.Join(
		retry(func() error { return probeDNS(dnsListen) }, probeRetries, probePause),
		retry(func() error { return probeWeb(webListen) }, probeRetries, probePause),
	)
}

func retry(f func() error, retries int, pause time.Duration) error {
	var err error
	for range retries {
		if err = f(); err == nil {
			return nil
		}
		time.Sleep(pause)
	}
	return err
}

func probeDNS(dnsListen string) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("udp", completeAddress(dnsListen, 53))
	if err != nil {
		return fmt.Errorf("probe failed, check failure at %s/locals-dns.log: %w", os.TempDir(), err)
	}
	defer conn.Close()
	return nil
}

func probeWeb(webListen string) error {
	conn, err := net.DialTimeout("tcp", completeAddress(webListen, 443), 2*time.Second)
	if err != nil {
		return fmt.Errorf("probe failed, check failure at %s/locals-web.log: %w", os.TempDir(), err)
	}
	defer conn.Close()
	return nil
}

func completeAddress(addr string, defaultPort int) string {
	parts := strings.Split(addr, ":")
	if len(parts) == 1 {
		return fmt.Sprintf("%s:%d", addr, defaultPort)
	}
	if parts[0] == "" {
		return fmt.Sprintf("127.0.0.1%s", addr)
	}
	return addr
}
