package platform

import (
	"io"
	"net"
)

// Platform defines everything the tool needs from the operating system.
// Swap these out to run the app in tests or with recorded behavior.
type Platform interface {
	Stdout() io.Writer
	Stderr() io.Writer
	Stdin() io.Reader
	Env(name string) string
	HomeDir() (string, error)
	FS() Filesystem
	Run(cmd string, args ...string) (string, error)
	CheckDNSSetup(configDir string) *DNSStatus
}

func IsIPOnInterface(ifaceName, targetIP string) bool {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		// addr is in CIDR format like "127.1.2.3/32"
		ip, _, _ := net.ParseCIDR(addr.String())
		if ip != nil && ip.String() == targetIP {
			return true
		}
	}
	return false
}
