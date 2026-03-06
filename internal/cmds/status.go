package cmds

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strings"

	"locals/api/locals"

	"github.com/spf13/cobra"
)

func statusCmd(p *locals.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show locals status",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return status(p, localsDir)
		},
	}
}

func status(p *locals.Platform, configDir string) error {
	fmt.Println("----------- 📍 Locals Status -----------")

	dnsMode := "🔓 INACTIVE"
	if runtime.GOOS == "linux" {
		mounts, err := p.IO.ReadFile("/proc/mounts")
		if err != nil {
			return fmt.Errorf("failed to check mounts: %w", err)
		}
		if strings.Contains(string(mounts), " /etc/resolv.conf ") {
			dnsMode = "🔒 BIND-MOUNT ACTIVE"
		}
	} else {
		dnsConfig, err := p.IO.ReadFile("/etc/resolver/locals")
        hasFile := err == nil && len(dnsConfig) > 0
        
        hasAlias := isIPOnInterface("lo0", DefaultDNSListen)

        if hasFile && hasAlias {
            dnsMode = "🔒 RESOLVER REDIRECT ACTIVE"
        } else if hasFile && !hasAlias {
            dnsMode = "⚠️ MISSING DNS IP ALIAS"        
		} else if !hasFile && hasAlias {
            dnsMode = "⚠️ MISSING DNS RESOLVER FILE"
        }
	}
	fmt.Printf("DNS System:  %s\n", dnsMode)

	if isProcessAlive(p, filepath.Join(configDir, "dns.pid")) {
		fmt.Println("DNS Service: 🟢 RUNNING")
	} else {
		fmt.Println("DNS Service: 🔴 OFF")
	}

	if isProcessAlive(p, filepath.Join(configDir, "web.pid")) {
		fmt.Println("Web Proxy:   🟢 RUNNING")
		fmt.Println("\nActive web entrypoints:")
	} else {
		fmt.Println("Web Proxy:   🔴 OFF")
		fmt.Println("\nConfigured web entrypoints:")
	}

	rulesDir := filepath.Join(configDir, "web")
	files, _ := p.IO.ReadDir(rulesDir)

	count := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			count++
			name := strings.TrimSuffix(f.Name(), ".json")
			target := "unknown"
			if content, err := p.IO.ReadFile(filepath.Join(rulesDir, f.Name())); err == nil {
				webConfig := WebConfig{}
				if err := json.Unmarshal(content, &webConfig); err == nil {
					name = webConfig.URL
					target = webConfig.Endpoint
				}
			}
			fmt.Printf("  🔗 %-15s -> %s\n", name, target)
		}
	}

	if count == 0 {
		fmt.Println("  (none)")
	}
	fmt.Println("----------------------------------------")
	return nil
}

func isProcessAlive(p *locals.Platform, pidPath string) bool {
	data, err := p.IO.ReadFile(pidPath)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}
	return p.Process.IsProcessAlive(pid)
}

func isIPOnInterface(ifaceName, targetIP string) bool {
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
