package env

import (
	"fmt"
	"os"
)

const (
	EnvSystemCA = "SYSTEM_CA"

	DefaultSystemCA = "/etc/ssl/certs/ca-certificates.crt"

	EnvDomain = "DOMAIN"
)

var DNSMasqConfs = []string{
	"/etc/dnsmasq-conf.conf",
	"/etc/dnsmasq.conf",
}

type EnvFunc func(string) string

func (e EnvFunc) SystemCA() string {
	systemCA := e(EnvSystemCA)
	if systemCA != "" {
		return systemCA
	}
	return DefaultSystemCA
}

func (EnvFunc) DetectDNSMasqConf() (string, error) {
	for _, dnsMasqConf := range DNSMasqConfs {
		if f, err := os.Open(dnsMasqConf); err == nil {
			return dnsMasqConf, f.Close()
		}
	}
	return "", fmt.Errorf("no dnsmasq config file found")
}
