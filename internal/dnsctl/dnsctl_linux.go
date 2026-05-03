//go:build linux

package dnsctl

import (
	_ "embed"
	"fmt"
	"locals/internal/platform"
	"log"
	"regexp"
)

const (
	resolvConfPath = "/etc/resolv.conf"
)

//go:embed resolv.conf
var LocalsResolvConfContents string

func (d *osDNSController) Grab() error {
	active, err := active(d.p)
	if err != nil {
		return fmt.Errorf("failed to check resolv.conf: %w", err)
	}
	if active {
		log.Printf("⚠️ %s already replaced. Skipping.", resolvConfPath)
		return nil
	}

	resolvConfLocal := dnsConfigFile(d.cfg.LocalsDir)
	if err := d.p.FS().CreateFile(resolvConfLocal, resolveConfig(d.cfg.DNSListen)); err != nil {
		return fmt.Errorf("failed to create alternate resolv.conf: %w", err)
	}
	if _, err := d.p.Run("sudo", "mount", "--bind", resolvConfLocal, resolvConfPath); err != nil {
		return fmt.Errorf("failed to bind mount %s: %w", resolvConfPath, err)
	}
	log.Printf("🔒 %s mounted to redirect DNS queries to locals dns first", resolvConfPath)
	return nil
}

func (d *osDNSController) Release() error {
	active, err := active(d.p)
	if err != nil {
		return fmt.Errorf("failed to check resolv.conf: %w", err)
	}
	if active {
		if _, err := d.p.Run("sudo", "umount", resolvConfPath); err != nil {
			return fmt.Errorf("failed to undo mount bind on /etc/resolv.conf: %w", err)
		}
	} else {
		log.Printf("ℹ️ /etc/resolv.conf was not mounted.")
	}
	return nil
}

func active(p platform.Platform) (bool, error) {
	r, err := regexp.Compile(resolveConfig(".*"))
	if err != nil {
		return false, fmt.Errorf("failed to compile resolv.conf contents pattern: %w", err)
	}
	activeResolvConf, err := p.FS().ReadFile(resolvConfPath)
	if err != nil {
		return false, fmt.Errorf("failed to read resolv.conf: %w", err)
	}
	return r.Match(activeResolvConf), nil
}

func Status(p platform.Platform, _ string) (*DNSStatus, error) {
	dnsMode := "INACTIVE"
	active, err := active(p)
	if err != nil {
		return nil, fmt.Errorf("failed to check resolv.conf: %w", err)
	}
	if active {
		dnsMode = "BIND-MOUNT ACTIVE"
	}
	return &DNSStatus{Active: active, Status: dnsMode}, nil
}

func resolveConfig(dnsListen string) string {
	return fmt.Sprintf(LocalsResolvConfContents, dnsListen)
}
