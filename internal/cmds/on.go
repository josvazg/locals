package cmds

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"locals/internal/platform"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"
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

func onCmd(p platform.Platform, localsDir, binary string) *cobra.Command {
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
			return on(p, binary, localsDir, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func on(p platform.Platform, binary, localsDir string, dryrun bool) error {
	localsBin, err := localsBinary(p, binary)
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
	return probeServices(p, state.DNSListen, ListenAddr)
}

func localsBinary(p platform.Platform, binary string) (string, error) {
	log.Printf("evaluating locals path: binary=%q", binary)
	if p.IO().PathExists(binary) {
		log.Printf("render binary: %q", binary)
		return binary, nil
	}
	binPath, err := p.Proc().LookPath("locals")
	if err != nil {
		return "", fmt.Errorf("failed to find locals binary path: %w", err)
	}
	log.Printf("resolved as binPath=%q", binPath)
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
	if pid := readPIDFromFile(p, pidFile); pid >= 0 {
		if p.Proc().IsProcessAlive(pid) {
			fmt.Printf("⚠️ locals dns is already running (PID: %d). Skipping start.\n", pid)
			return nil
		}
		fmt.Println("🔄 Cleaning up stale PID file from previous crash...")
		if err := p.IO().RemoveFiles(pidFile); err != nil {
			return fmt.Errorf("failed to remove PID file %q: %w", pidFile, err)
		}
	}
	dnsServers, err := currentDNSServers(p)
	if err != nil {
		return fmt.Errorf("failed to detect fallback DNS servers: %w", err)
	}
	fallbacks := strings.Join(dnsServers, ",")
	pid, err := p.Proc().Launch("sudo", "env", fmt.Sprintf("PATH=%s", p.Env("PATH")),
		"nohup", cfg.LocalsBin,
		"dns", cfg.DNSListen, fallbacks,
		"--log", filepath.Join(p.IO().TempDir(), "locals-dns.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded DNS server: %w", err)
	}
	if err := p.IO().CreateFile(pidFile, fmt.Sprintf("%d", pid)); err != nil {
		return fmt.Errorf("failed to write the embedded DNS server PID file: %w", err)
	}
	log.Printf("✅ locals DNS started on %s (PID: %d)", cfg.DNSListen, pid)
	return nil
}

func currentDNSServers(p platform.Platform) ([]string, error) {
	resolvConfCfg, err := p.IO().ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/resolv.conf: %w", err)
	}
	var servers []string
	scanner := bufio.NewScanner(bytes.NewReader(resolvConfCfg))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			servers = append(servers, fields[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning /etc/resolv.conf: %w", err)
	}
	return servers, nil
}

func isUsingSystemdResolved(p platform.Platform) bool {
	paths := []string{
		"/lib/systemd/system/systemd-resolved.service",
		"/usr/lib/systemd/system/systemd-resolved.service",
	}
	for _, path := range paths {
		if p.IO().PathExists(path) {
			return true
		}
	}
	return false
}

func launchWeb(p platform.Platform, cfg *Config) error {
	pidFile := filepath.Join(cfg.LocalsDir, "web.pid")
	if pid := readPIDFromFile(p, pidFile); pid >= 0 {
		if p.Proc().IsProcessAlive(pid) {
			fmt.Printf("⚠️ locals web is already running (PID: %d). Skipping start.\n", pid)
			return nil
		}
		fmt.Println("🔄 Cleaning up stale PID file from previous crash...")
		if err := p.IO().RemoveFiles(pidFile); err != nil {
			return fmt.Errorf("failed to remove PID file %q: %w", pidFile, err)
		}
	}
	webCfg := filepath.Join(cfg.LocalsDir, "web")
	pid, err := p.Proc().Launch("sudo", "env", fmt.Sprintf("PATH=%s", p.Env("PATH")),
		"nohup", cfg.LocalsBin, "web",
		webCfg, "--log", filepath.Join(p.IO().TempDir(), "locals-web.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded web server: %w", err)
	}
	if err := p.IO().CreateFile(pidFile, fmt.Sprintf("%d", pid)); err != nil {
		return fmt.Errorf("failed to write the embedded web server PID file: %w", err)
	}
	log.Printf("✅ locals web started on %s (PID: %d)", cfg.DNSListen, pid)
	return nil
}

func readPIDFromFile(p platform.Platform, pidFile string) int {
	data, err := p.IO().ReadFile(pidFile)
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

func probeServices(p platform.Platform, dnsListen, webListen string) error {
	return errors.Join(
		retry(func() error { return probeDNS(p, dnsListen) }, probeRetries, probePause),
		retry(func() error { return probeWeb(p, webListen) }, probeRetries, probePause),
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

func probeDNS(p platform.Platform, dnsListen string) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("udp", completeAddress(dnsListen, 53))
	if err != nil {
		return fmt.Errorf("probe failed, check failure at %s/locals-dns.log: %w", p.IO().TempDir(), err)
	}
	defer conn.Close()
	return nil
}

func probeWeb(p platform.Platform, webListen string) error {
	dialer := &net.Dialer{
		Timeout: 2 * time.Second,
	}
	conf := &tls.Config{
		InsecureSkipVerify: true,
		ServerName: "probe",
	}
	address := completeAddress(webListen, 443)
	conn, err := tls.DialWithDialer(dialer, "tcp", address, conf)
	if err != nil {
		return fmt.Errorf("probe failed, check failure at %s/locals-web.log: %w", p.IO().TempDir(), err)
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
