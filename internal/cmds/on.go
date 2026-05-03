package cmds

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"locals/internal/cfg"
	"locals/internal/dnsctl"
	"locals/internal/mkcert"
	"locals/internal/platform"
	"locals/internal/service"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	probeRetries = 5

	probePause = 1 * time.Second
)

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
			config, err := newConfig(p, binary, localsDir, dryrun)
			if err != nil {
				return fmt.Errorf("failed to setup on config: %w", err)
			}
			return on(p, config, dryrun)
		},
	}
	cmd.Flags().BoolVarP(&dryrun, "dryrun", "", false, "show what start would have done")
	return cmd
}

func newConfig(p platform.Platform, binary, localsDir string, dryrun bool) (*cfg.Config, error) {
	localsBin := ""
	if binary != "" {
		bin, err := localsBinary(p, binary)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path to locals: %w", err)
		}
		localsBin = bin
	}
	tmpDir := os.Getenv(cfg.EnvLocalsTempDir)
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	config := cfg.Config{
		DNSListen: platform.DefaultDNSListen,
		LocalsDir: localsDir,
		LocalsBin: localsBin,
		SystemCA:  platform.SystemCA(p),
		TempDir:   tmpDir,
		Dryrun:    dryrun,
	}
	return &config, nil
}

func on(p platform.Platform, config *cfg.Config, dryrun bool) error {
	qual := ""
	if dryrun {
		qual = "dryrun "
	}
	if err := installMkcert(p); err != nil {
		return fmt.Errorf("failed to %sinstall mkcert: %w", qual, err)
	}
	svctl := service.New(config, p.Stdout())
	dnsServers, err := currentDNSServers(p)
	if err != nil {
		return fmt.Errorf("failed to detect fallback DNS servers: %w", err)
	}
	if err := launchDNS(svctl, config, dnsServers); err != nil {
		return fmt.Errorf("failed to %slaunch embedded DNS server: %w", qual, err)
	}
	if err := dnsctl.NewDNSController(p, config).Grab(); err != nil {
		return fmt.Errorf("failed to %sconfigure system DNS: %w", qual, err)
	}
	if err := launchWeb(svctl, config); err != nil {
		return fmt.Errorf("failed to %slaunch embedded Web server: %w", qual, err)
	}
	if dryrun {
		return nil
	}
	return probeServices(config.TempDir, config.DNSListen, ListenAddr)
}

func localsBinary(p platform.Platform, binary string) (string, error) {
	fullPath := binary
	if !filepath.IsAbs(binary) {
		var err error
		if fullPath, err = filepath.Abs(binary); err != nil {
			return "", fmt.Errorf("failed to expand binary path %q; %w", binary, err)
		}
	}
	if !p.FS().PathExists(fullPath) {
		bin, err := p.Test("sh", "-c", "command -v locals")
		if err != nil {
			return "", fmt.Errorf("failed to infer binary path %q: %w\npath: %s",
				binary, err, os.Getenv("PATH"))
		}
		bin = strings.TrimSpace(bin)
		if !p.FS().PathExists(bin) {
			return "", fmt.Errorf("failed to resolve a working binary path %q at path:\n%s",
				fullPath, os.Getenv("PATH"))
		}
		fullPath = bin
	}
	return fullPath, nil
}

func installMkcert(p platform.Platform) error {
	if err := mkcert.New(p.Stdout()).Install(); err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}
	log.Printf("For CLI usage run:\nsource <(locals env)")
	return nil
}

func launchDNS(svctl service.Control, config *cfg.Config, dnsServers []string) error {
	fallbacks := strings.Join(dnsServers, ",")
	pid, err := svctl.Launch("dns",
		config.LocalsBin, "dns", config.DNSListen, fallbacks,
		"--log", filepath.Join(config.TempDir, "locals-dns.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded DNS server: %w", err)
	}
	log.Printf("✅ locals DNS started on %s (PID: %d)", config.DNSListen, pid)
	return nil
}

func currentDNSServers(p platform.Platform) ([]string, error) {
	resolvConfCfg, err := p.FS().ReadFile("/etc/resolv.conf")
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
		if p.FS().PathExists(path) {
			return true
		}
	}
	return false
}

func launchWeb(svctl service.Control, config *cfg.Config) error {
	webCfg := filepath.Join(config.LocalsDir, "web")
	pid, err := svctl.Launch("web",
		config.LocalsBin, "web",
		webCfg, "--log", filepath.Join(config.TempDir, "locals-web.log"))
	if err != nil {
		return fmt.Errorf("failed to launch embedded web server: %w", err)
	}
	log.Printf("✅ locals web started on %s (PID: %d)", config.DNSListen, pid)
	return nil
}

func readPIDFromFile(p platform.Platform, pidFile string) int {
	data, err := p.FS().ReadFile(pidFile)
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

func probeServices(tmpDir string, dnsListen, webListen string) error {
	return errors.Join(
		retry(func() error { return probeDNS(tmpDir, dnsListen) }, probeRetries, probePause),
		retry(func() error { return probeWeb(tmpDir, webListen) }, probeRetries, probePause),
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

func probeDNS(tmpDir string, dnsListen string) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("udp", completeAddress(dnsListen, 53))
	if err != nil {
		return fmt.Errorf("probe failed, check failure at %s/locals-dns.log: %w", tmpDir, err)
	}
	defer conn.Close()
	return nil
}

func probeWeb(tmpDir string, webListen string) error {
	dialer := &net.Dialer{
		Timeout: 2 * time.Second,
	}
	conf := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "probe",
	}
	address := completeAddress(webListen, 443)
	conn, err := tls.DialWithDialer(dialer, "tcp", address, conf)
	if err != nil {
		return fmt.Errorf("probe failed, check failure at %s/locals-web.log: %w", tmpDir, err)
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
