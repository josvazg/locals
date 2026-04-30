//go:build linux

package dnsctl

import (
	"fmt"
	"locals/internal/platform"
	"log"
)

const (
	resolverConf = "/etc/systemd/resolved.conf.d/locals.conf"
)

func (d *osDNSController) Grab() error {
	// use our replacement file as /etc/resolv.conf coul be a symlink,
	// or mounted elsewhere
	resolvConfLocal := platform.DNSConfigFile(d.cfg.LocalsDir)
	resolvConfMounted, err := platform.Find(d.p.FS(), "/proc/self/mountinfo", resolvConfLocal)
	if err != nil {
		return fmt.Errorf("failed to check for mountinfo: %w", err)
	}
	if resolvConfMounted {
		log.Printf("⚠️ /etc/resolv.conf already replaced. Skipping.")
		return nil
	}
	resolvCfg := fmt.Sprintf("nameserver %s\noptions edns0 trust-ad", d.cfg.DNSListen)
	if err := d.p.FS().CreateFile(resolvConfLocal, resolvCfg); err != nil {
		return fmt.Errorf("failed to create alternate resolv.conf: %w", err)
	}
	if _, err := d.p.Run("sudo", "mount", "--bind", resolvConfLocal, "/etc/resolv.conf"); err != nil {
		return fmt.Errorf("failed to bind mount /etc/resolv.conf: %w", err)
	}
	log.Printf("🔒 /etc/resolv.conf mounted to redirect DNS queries to locals dns first")
	return nil
}

func (d *osDNSController) Release() error {
	resolvConfLocal := platform.DNSConfigFile(d.cfg.LocalsDir)
	resolvConfMounted, err := platform.Find(d.p.FS(), "/proc/self/mountinfo", resolvConfLocal)
	if err != nil {
		return fmt.Errorf("failed to check for mountinfo: %w", err)
	}
	if resolvConfMounted {
		if _, err := d.p.Run("sudo", "umount", "/etc/resolv.conf"); err != nil {
			return fmt.Errorf("failed to undo mount bind on /etc/resolv.conf: %w", err)
		}
	} else {
		log.Printf("ℹ️ /etc/resolv.conf was not mounted.")
	}
	if err := d.p.FS().RemoveFiles(resolverConf); err != nil {
		return fmt.Errorf("failed to remove resolved config: %w", err)
	}
	return nil
}
