//go:build darwin

package dnsctl

import (
	_ "embed"
	"errors"
	"fmt"
	"locals/internal/platform"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	resolverDir = "/etc/resolver"

	resolverLocalsFile = resolverDir + "/locals"
)

//go:embed locals
var LocalsResolverContents string

func (d *osDNSController) Prepare() error {
	if hasAlias(d.cfg.DNSListen) {
		return nil
	}
	_, err := d.p.Run("sudo", "ifconfig", "lo0", "alias",
		d.cfg.DNSListen, "netmask", "255.255.255.255")
	if err != nil {
		return fmt.Errorf("failed to set lo0 DNS redirect: %w", err)
	}
	return nil
}

func (d *osDNSController) Grab() error {
	fileMatches, err := fileMatches(d.p)
	if err != nil {
		return fmt.Errorf("failed to match resolver: %w", err)
	}
	if fileMatches {
		return nil
	}
	tmpLocalsResolverFile := filepath.Join(d.cfg.TempDir, "locals")
	if err := d.p.FS().CreateFile(tmpLocalsResolverFile, resolverConfig(d.cfg.DNSListen)); err != nil {
		return fmt.Errorf("failed to write resolver config: %w", err)
	}
	if _, err := d.p.Run("sudo", "cp", tmpLocalsResolverFile, resolverDir); err != nil {
		return fmt.Errorf("failed to copy resolver config to %s: %w", resolverLocalsFile, err)
	}
	if _, err := d.p.Run("sudo", "chmod", "o+r", resolverLocalsFile); err != nil {
		return fmt.Errorf("failed to make resolver config readable by all: %w", err)
	}
	return nil
}

func (d *osDNSController) Release() error {
	if hasAlias(d.cfg.DNSListen) {
		if _, err := d.p.Run("sudo", "ifconfig", "lo0", "-alias", d.cfg.DNSListen); err != nil {
			return fmt.Errorf("failed to remove lo0 DNS redirect: %w", err)
		}
	}
	fileMatches, err := fileMatches(d.p)
	if err != nil {
		return fmt.Errorf("failed to match resolver: %w", err)
	}
	if !fileMatches {
		return nil
	}
	if _, err := d.p.Run("sudo", "rm", resolverLocalsFile); err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			return nil
		}
		return fmt.Errorf("failed to remove locals resolver file %q: %w",
			resolverLocalsFile, err)
	}
	return nil
}

func Status(p platform.Platform, dnsListen string) (*DNSStatus, error) {
	fileMatches, err := fileMatches(p)
	if err != nil {
		return nil, fmt.Errorf("failed to check resolver file matches: %w", err)
	}
	hasAlias := hasAlias(dnsListen)

	dnsMode := ""
	switch {
	case fileMatches && hasAlias:
		dnsMode = "RESOLVER REDIRECT ACTIVE"
	case !fileMatches && !hasAlias:
		dnsMode = "INACTIVE"
	case !hasAlias:
		dnsMode = "MISSING DNS IP ALIAS"
	case !fileMatches:
		dnsMode = "MISSING DNS CONFIG FILE"
	}
	return &DNSStatus{Active: fileMatches && hasAlias, Status: dnsMode}, nil
}

func fileMatches(p platform.Platform) (bool, error) {
	r, err := regexp.Compile(resolverConfig(".*"))
	if err != nil {
		return false, fmt.Errorf("failed to compile resolver contents pattern: %w", err)
	}
	activeResolverConfig, err := p.FS().ReadFile(resolverLocalsFile)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read resolver config: %w", err)
	}
	return r.Match(activeResolverConfig), nil
}

func hasAlias(dnsListen string) bool {
	return platform.IsIPOnInterface("lo0", dnsListen)
}

func resolverConfig(dnsListen string) string {
	return fmt.Sprintf(LocalsResolverContents, dnsListen)
}
