package cmds

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"locals/internal/platform"

	"github.com/spf13/cobra"
)

func statusCmd(p platform.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show locals status",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return status(p, localsDir)
		},
	}
}

func status(p platform.Platform, configDir string) error {
	fmt.Println("----------- 📍 Locals Status -----------")

	dnsStatus := p.CheckDNSSetup()
	fmt.Printf("DNS System:  %s\n", dnsStatus)

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
	files, _ := p.IO().ReadDir(rulesDir)

	count := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			count++
			name := strings.TrimSuffix(f.Name(), ".json")
			target := "unknown"
			if content, err := p.IO().ReadFile(filepath.Join(rulesDir, f.Name())); err == nil {
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

func isProcessAlive(p platform.Platform, pidPath string) bool {
	data, err := p.IO().ReadFile(pidPath)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}
	return p.Proc().IsProcessAlive(pid)
}
