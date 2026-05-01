package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"locals/internal/platform"
	"locals/internal/service"
	"locals/internal/web"

	"github.com/spf13/cobra"
)

func statusCmd(p platform.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show locals status",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return status(p, localsDir, os.TempDir())
		},
	}
}

func status(p platform.Platform, configDir, tmpDir string) error {
	fmt.Println("----------- 📍 Locals Status -----------")

	dnsStatus := p.CheckDNSSetup(configDir)
	fmt.Printf("DNS System:  %s\n", dnsStatus)
	svctl := service.New(configDir, tmpDir, p.Env("PATH"), p.Stdout())

	if svctl.IsRunning("dns") {
		fmt.Println("DNS Service: 🟢 RUNNING")
	} else {
		fmt.Println("DNS Service: 🔴 OFF")
	}

	if svctl.IsRunning("web") {
		fmt.Println("Web Proxy:   🟢 RUNNING")
		fmt.Println("\nActive web entrypoints:")
	} else {
		fmt.Println("Web Proxy:   🔴 OFF")
		fmt.Println("\nConfigured web entrypoints:")
	}

	rulesDir := filepath.Join(configDir, "web")
	files, _ := p.FS().ReadDir(rulesDir)

	count := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			count++
			name := strings.TrimSuffix(f.Name(), ".json")
			target := "unknown"
			if content, err := p.FS().ReadFile(filepath.Join(rulesDir, f.Name())); err == nil {
				webConfig := web.Config{}
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
