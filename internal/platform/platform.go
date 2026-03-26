package platform

import "net"

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
