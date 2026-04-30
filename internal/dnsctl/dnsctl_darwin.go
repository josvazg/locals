//go:build darwin

package dnsctl

import (
	"fmt"
	"locals/internal/platform"
	"path/filepath"
	"strings"
)

const (
	resolverMacConfigDir = "/etc/resolver"

	resolverMacLocalsFile = "/etc/resolver/locals"
)

func (d *osDNSController) Grab() error {
	if !platform.IsIPOnInterface("lo0", d.cfg.DNSListen) {
		_, err := d.p.Proc().Run("sudo", "ifconfig", "lo0", "alias",
			d.cfg.DNSListen, "netmask", "255.255.255.255")
		if err != nil {
			return fmt.Errorf("failed to set lo0 DNS redirect: %w", err)
		}
	}
	resolverCfg := fmt.Sprintf("nameserver %s\nport 53\n", d.cfg.DNSListen)
	tmpLocalsResolverFile := filepath.Join(d.p.FS().TempDir(), "locals-resolver.conf")
	if err := d.p.FS().CreateFile(tmpLocalsResolverFile, resolverCfg); err != nil {
		return fmt.Errorf("failed to write resolver config: %w", err)
	}
	if _, err := d.p.Proc().Run("sudo", "cp", tmpLocalsResolverFile, resolverMacLocalsFile); err != nil {
		return fmt.Errorf("failed to copy resolver config to %s: %w", resolverMacLocalsFile, err)
	}
	if _, err := d.p.Proc().Run("sudo", "chmod", "o+r", resolverMacLocalsFile); err != nil {
		return fmt.Errorf("failed to make resolver config readable by all: %w", err)
	}
	return nil
}

func (d *osDNSController) Release() error {
	if platform.IsIPOnInterface("lo0", d.cfg.DNSListen) {
		if _, err := d.p.Proc().Run("sudo", "ifconfig", "lo0", "-alias", d.cfg.DNSListen); err != nil {
			return fmt.Errorf("failed to remove lo0 DNS redirect: %w", err)
		}
	}
	if _, err := d.p.Proc().Run("sudo", "rm", resolverMacLocalsFile); err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			return nil
		}
		return fmt.Errorf("failed to remove locals resolver file %q: %w",
			resolverMacLocalsFile, err)
	}
	return nil
}
