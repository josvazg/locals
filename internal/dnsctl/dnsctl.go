package dnsctl

import (
	"locals/internal/cfg"
	"locals/internal/platform"
)

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
