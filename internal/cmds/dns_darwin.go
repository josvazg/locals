//go:build darwin

package cmds

import (
	"fmt"
	"locals/internal/platform"
)

func configureDNS(p platform.Platform, cfg *Config) error {
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

func unconfigureDNS(p platform.Platform, state *Config) error {
	if platform.IsIPOnInterface("lo0", state.DNSListen) {
		if _, err := p.Proc().Run("sudo", "ifconfig", "lo0", "-alias", state.DNSListen); err != nil {
			return fmt.Errorf("failed to remove lo0 DNS redirect: %w", err)
		}
	}
	if err := p.IO().RemoveFiles(resolverMacLocalsFile); err != nil {
		return fmt.Errorf("failed to remove locals resolver file %q: %w",
			resolverMacLocalsFile, err)
	}
	return nil
}
