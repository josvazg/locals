package cmds

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"locals/api/locals"
	"locals/internal/render"

	"github.com/spf13/cobra"
)

func onCmd(p *locals.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "on",
		Short: "Start the web proxy and grab DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return on(p, localsDir, dryrun)
		},
	}
}

func on(p *locals.Platform, localsDir string, dryrun bool) error {
	state := render.State{
		DNSListen: locals.DefaultDNSListen,
		LocalsDir: localsDir,
		SystemCA:  p.Env.SystemCA(),
	}
	onScriptCode, err := render.On(state)
	if err != nil {
		return fmt.Errorf("failed to render the on script: %w", err)
	}
	onScript := &script{name: filepath.Join(localsDir, "on.sh"), contents: onScriptCode}
	if err := save(p, onScript); err != nil {
		return fmt.Errorf("failed to save on.sh script: %w", err)
	}
	if dryrun {
		return show(onScript)
	}
	if err := runScript(p, onScript); err != nil {
		return fmt.Errorf("failed to run on.sh script: %w", err)
	}
	return probeServices(state.DNSListen, ":443")
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
