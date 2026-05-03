package platform

const (
	EnvSystemCA = "SYSTEM_CA"

	DefaultSystemCA = "/etc/ssl/certs/ca-certificates.crt"

	EnvDomain = "DOMAIN"
)

func SystemCA(p Platform) string {
	systemCA := p.Env(EnvSystemCA)
	if systemCA != "" {
		return systemCA
	}
	return DefaultSystemCA
}
