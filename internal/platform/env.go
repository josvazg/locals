package platform

const (
	EnvSystemCA = "SYSTEM_CA"

	DefaultSystemCA = "/etc/ssl/certs/ca-certificates.crt"

	EnvDomain = "DOMAIN"

	// EnvLocalsConfigDir overrides the default ~/.config/locals state directory.
	EnvLocalsConfigDir = "LOCALS_CONFIG_DIR"
)

type EnvFunc func(string) string

func (e EnvFunc) SystemCA() string {
	systemCA := e(EnvSystemCA)
	if systemCA != "" {
		return systemCA
	}
	return DefaultSystemCA
}
