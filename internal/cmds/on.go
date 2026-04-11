package cmds

import (
	"errors"
	"fmt"
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

func onCmd(p platform.Platform, localsDir string) *cobra.Command {
	var dryrun bool
	cmd := &cobra.Command{
		Use:   "on",
		Short: "Activate locals by starting the web proxy and grab local DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			if dryrun {
				log.Printf("DRYRUN")
				p = platform.NewDryrunPlatform(p)
			}
			return on(p, localsDir, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func on(p platform.Platform, localsDir string, dryrun bool) error {
	localsBin, err := localsBinary(p)
	if err != nil {
		return fmt.Errorf("failed to resolve path to locals: %w", err)
	}
	state := Config{
		DNSListen: platform.DefaultDNSListen,
		LocalsDir: localsDir,
		LocalsBin: localsBin,
		SystemCA:  platform.SystemCA(p),
	}
	qual := ""
	if dryrun {
		qual = "dryrun "
	}
	if err := installMkcert(p); err != nil {
		return fmt.Errorf("failed to %sinstall mkcert: %w", qual, err)
	}
	if err := launchDNS(p, &state); err != nil {
		return fmt.Errorf("failed to %slaunch embedded DNS server: %w", qual, err)
	}
	if err := configureDNS(p, &state); err != nil {
		return fmt.Errorf("failed to %sconfigure system DNS: %w", qual, err)
	}
	if err := launchWeb(p, &state); err != nil {
		return fmt.Errorf("failed to %slaunch embedded Web server: %w", qual, err)
	}
	if dryrun {
		return nil
	}
	return probeServices(state.DNSListen, ListenAddr)
}

func localsBinary(p platform.Platform) (string, error) {
	binary := os.Args[0]
	if p.IO().PathExists(binary) {
		return binary, nil
	}
	binPath, err := p.Proc().Run("command", "-v", "locals")
	if err != nil {
		return "", fmt.Errorf("failed to find locals binary path: %w", err)
	}
	return binPath, nil
}

func installMkcert(p platform.Platform) error {
	out, err := p.Proc().Run("mkcert", "-install")
	if err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}
	log.Print(out)
	log.Printf("For CLI usage run:\nsource <(locals env)")
	return nil
}

func launchDNS(p platform.Platform, cfg *Config) error {
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
	pid, err := p.Proc().Launch("sudo", "nohup", cfg.LocalsBin,
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

func configureDNS(p platform.Platform, cfg *Config) error {
	if runtime.GOOS == "darwin" {
		return configureMacDNS(p, cfg)
	}
	return configureLinuxDNS(p, cfg)
}

func configureMacDNS(p platform.Platform, cfg *Config) error {
	if !platform.IsIPOnInterface("lo0", cfg.DNSListen) {
		_, err := p.Proc().Run("sudo", "ifconfig", "lo0", "alias",
			cfg.DNSListen, "netmask", "255.255.255.255")
		if err != nil {
			return fmt.Errorf("failed to set lo0 DNS redirect: %w", err)
		}
	}
	resolverCfg := fmt.Sprintf("nameserver %s\nport 53\n", cfg.DNSListen)
	if err := p.IO().CreateFile(resolverMacLocalsFile, resolverCfg); err != nil {
		return fmt.Errorf("failed to write resolver config: %w", err)
	}
	return nil
}

func configureLinuxDNS(p platform.Platform, cfg *Config) error {
	if p.IO().PathExists("/run/systemd/resolve") && isUsingSystemdResolved(p) {
		return configureLinuxResolved(p, cfg)
	}
	log.Printf("📡 systemd-resolved not found. Falling back to /etc/resolv.conf bind-mount.")
	return configureLinuxBindMount(p, cfg)
}

func isUsingSystemdResolved(p platform.Platform) bool {
	paths := []string{
		"/lib/systemd/system/systemd-resolved.service",
		"/usr/lib/systemd/system/systemd-resolved.service",
	}
	installed := false
	for _, path := range paths {
		if p.IO().PathExists(path) {
			installed = true
			break
		}
	}
	if !installed {
		return false
	}
	link, err := os.Readlink("/etc/resolv.conf")
	return err == nil && strings.Contains(link, "systemd-resolved")
}

func configureLinuxResolved(p platform.Platform, cfg *Config) error {
	log.Printf("📡 systemd-resolved detected. Using Routing Domain setup.")
	localsResolvedCfg := fmt.Sprintf("[Resolve]\nDNS=%s\nDomains=~locals\n", cfg.DNSListen)
	if err := p.IO().CreateFile(resolverConf, localsResolvedCfg); err != nil {
		return fmt.Errorf("failed to configure locals resolved: %w", err)
	}
	if _, err := p.Proc().Run("sudo", "systemctl", "restart", "systemd-resolved"); err != nil {
		return fmt.Errorf("failed to restart systemd resolved: %w", err)
	}
	log.Printf("🔒 systemd-resolved configured to route .locals to %s", cfg.DNSListen)
	return nil
}

func configureLinuxBindMount(p platform.Platform, cfg *Config) error {
	resolvConfMounted, err := find(p, "/proc/self/mountinfo", "/etc/resolv.conf")
	if err != nil {
		return fmt.Errorf("failed to check for mountinfo: %w", err)
	}
	if resolvConfMounted {
		log.Printf("⚠️ /etc/resolv.conf already replaced. Skipping.")
		return nil
	}
	resolvConfLocal := filepath.Join(cfg.LocalsDir, "resolv.patched.conf")
	resolvCfg := fmt.Sprintf("nameserver %s\noptions edns0 trust-ad", cfg.DNSListen)
	if err := p.IO().CreateFile(resolvConfLocal, resolvCfg); err != nil {
		return fmt.Errorf("failed to create alternate resolv.conf: %w", err)
	}
	if _, err := p.Proc().Run("sudo", "mount", "--bind", resolvConfLocal, "/etc/resolv.conf"); err != nil {
		return fmt.Errorf("failed to bind mount /etc/resolv.conf: %w", err)
	}
	log.Printf("🔒 /etc/resolv.conf mounted to redirect DNS queries to locals dns first")
	return nil
}

func launchWeb(p platform.Platform, cfg *Config) error {
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
	pid, err := p.Proc().Launch("sudo", "nohup", cfg.LocalsBin, "web",
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
