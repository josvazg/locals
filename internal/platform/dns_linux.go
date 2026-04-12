//go:build linux

package platform

import (
	"fmt"
)

func checkDNSSetup(io FilesHandler, configDir string) *DNSStatus {
	dnsMode := "INACTIVE"
	active := false
	found, err := Find(io, "/proc/self/mountinfo", DNSConfigFile(configDir))
	if err != nil {
		dnsMode = fmt.Sprintf("failed to check mounts: %v", err)
	} else if found {
		dnsMode = "BIND-MOUNT ACTIVE"
		active = true
	}
	return &DNSStatus{
		Active: active,
		Status: dnsMode,
	}
}
