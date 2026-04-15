//go:build darwin

package platform

func checkDNSSetup(io FilesHandler, _ string) *DNSStatus {
	dnsMode := ""
	dnsConfig, err := io.ReadFile("/etc/resolver/locals")
	hasFile := err == nil && len(dnsConfig) > 0

	// Assuming IsIPOnInterface is defined elsewhere in your project
	hasAlias := IsIPOnInterface("lo0", DefaultDNSListen)

	switch {
	case hasFile && hasAlias:
		dnsMode = "RESOLVER REDIRECT ACTIVE"
	case !hasFile && !hasAlias:
		dnsMode = "INACTIVE"
	case !hasAlias:
		dnsMode = "MISSING DNS IP ALIAS"
	case !hasFile:
		dnsMode = "MISSING DNS CONFIG FILE"
	}
	return &DNSStatus{
		Active: hasFile && hasAlias,
		Status: dnsMode,
	}
}
