//go:build linux

package cmds

import (
	"fmt"
	"locals/internal/platform"
	"log"
)

func configureDNS(p platform.Platform, cfg *Config) error {
	// use our replacement file as /etc/resolv.conf coul be a symlink,
	// or mounted elsewhere
	resolvConfLocal := platform.DNSConfigFile(cfg.LocalsDir)
	resolvConfMounted, err := platform.Find(p.FS(), "/proc/self/mountinfo", resolvConfLocal)
	if err != nil {
		return fmt.Errorf("failed to check for mountinfo: %w", err)
	}
	if resolvConfMounted {
		log.Printf("⚠️ /etc/resolv.conf already replaced. Skipping.")
		return nil
	}
	resolvCfg := fmt.Sprintf("nameserver %s\noptions edns0 trust-ad", cfg.DNSListen)
	if err := p.FS().CreateFile(resolvConfLocal, resolvCfg); err != nil {
		return fmt.Errorf("failed to create alternate resolv.conf: %w", err)
	}
	if _, err := p.Proc().Run("sudo", "mount", "--bind", resolvConfLocal, "/etc/resolv.conf"); err != nil {
		return fmt.Errorf("failed to bind mount /etc/resolv.conf: %w", err)
	}
	log.Printf("🔒 /etc/resolv.conf mounted to redirect DNS queries to locals dns first")
	return nil
}

func unconfigureDNS(p platform.Platform, cfg *Config) error {
	resolvConfLocal := platform.DNSConfigFile(cfg.LocalsDir)
	resolvConfMounted, err := platform.Find(p.FS(), "/proc/self/mountinfo", resolvConfLocal)
	if err != nil {
		return fmt.Errorf("failed to check for mountinfo: %w", err)
	}
	if resolvConfMounted {
		if _, err := p.Proc().Run("sudo", "umount", "/etc/resolv.conf"); err != nil {
			return fmt.Errorf("failed to undo mount bind on /etc/resolv.conf: %w", err)
		}
	} else {
		log.Printf("ℹ️ /etc/resolv.conf was not mounted.")
	}
	if err := p.FS().RemoveFiles(resolverConf); err != nil {
		return fmt.Errorf("failed to remove resolved config: %w", err)
	}
	return nil
}
