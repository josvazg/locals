package dnsctl

import (
	"fmt"
	"locals/internal/cfg"
	"locals/internal/platform"
	"path/filepath"
)

const (
	PatchedResolvConfPath = "resolv.patched.conf"
)

type DNSStatus struct {
	Active bool
	Status string
}

func (s *DNSStatus) String() string {
	icon := "🔓"
	if !s.Active {
		icon = "⚠️"
	}
	return fmt.Sprintf("%s %s", icon, s.Status)
}

type DNSController interface {
	Grab() error
	Release() error
}

type osDNSController struct {
	p   platform.Platform
	cfg *cfg.Config
}

func NewDNSController(p platform.Platform, cfg *cfg.Config) DNSController {
	return &osDNSController{p: p, cfg: cfg}
}

func dnsConfigFile(localsDir string) string {
	return filepath.Join(localsDir, PatchedResolvConfPath)
}
