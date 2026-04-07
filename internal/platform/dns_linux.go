//go:build linux
package platform

import (
	"fmt"
	"os"
	"strings"
)

func checkDNSSetup() *DNSStatus {
	dnsMode := "INACTIVE"
	dnsResolvedConfig, err := os.ReadFile("/etc/systemd/resolved.conf.d/locals.conf")
	active := err == nil && len(dnsResolvedConfig) > 0

	if active {
		dnsMode = "RESOLVED CONFIG ACTIVE"
	} else {
		mounts, err := os.ReadFile("/proc/self/mountinfo")
		if err != nil {
			dnsMode = fmt.Sprintf("failed to check mounts: %v", err)
		}
		if strings.Contains(string(mounts), "/resolv.patched.conf") {
			dnsMode = "BIND-MOUNT ACTIVE"
			active = true
		}
	}
	return &DNSStatus{
		Active: active,
		Status: dnsMode,
	}
}
