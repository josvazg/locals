package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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
	mounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return fmt.Errorf("failed to check mounts: %w", err)
	}
	if strings.Contains(string(mounts), " /etc/resolv.conf ") {
		dnsMode = "🔒 BIND-MOUNT ACTIVE"
	}
	fmt.Printf("DNS System:  %s\n", dnsMode)

	if isProcessAlive(filepath.Join(configDir, "dns.pid")) {
		fmt.Println("DNS Service: 🟢 RUNNING")
	} else {
		fmt.Println("DNS Service: 🔴 OFF")
	}

	if isProcessAlive(filepath.Join(configDir, "web.pid")) {
		fmt.Println("Web Proxy:   🟢 RUNNING")
		fmt.Println("\nActive web entrypoints:")
	} else {
		fmt.Println("Web Proxy:   🔴 OFF")
		fmt.Println("\nConfigured web entrypoints:")
	}

	rulesDir := filepath.Join(configDir, "web")
	files, _ := os.ReadDir(rulesDir)

	count := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			count++
			name := strings.TrimSuffix(f.Name(), ".json")
			target := "unknown"
			if content, err := os.ReadFile(filepath.Join(rulesDir, f.Name())); err == nil {
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

func isProcessAlive(pidPath string) bool {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 is the "ping" of process management
	return process.Signal(syscall.Signal(0)) == nil
}
