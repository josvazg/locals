package cmds

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"locals/api/locals"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func listCmd(p *locals.Platform, localsDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active service mappings",
		Run: func(cmd *cobra.Command, args []string) {
			webDir := filepath.Join(localsDir, locals.WebDir)
			files, err := p.IO.ReadDir(webDir)
			if err != nil {
				log.Fatal(err)
			}

			// Setup Tabwriter for clean columns
			w := tabwriter.NewWriter(p.Stdout, 0, 0, 3, ' ', 0)
			if len(files) == 0 {
				fmt.Fprintln(w, "No service mappings set")
				w.Flush()
				return
			}
			fmt.Fprintln(w, "DOMAIN\tTARGET\tSTATUS")
			fmt.Fprintln(w, "------\t------\t------")

			dnsOn := isProcessAlive(filepath.Join(localsDir, "dns.pid"))
			webOn := isProcessAlive(filepath.Join(localsDir, "web.pid"))
			status := "Inactive"
			if dnsOn && webOn {
				status = "Active"
			} else if dnsOn {
				status = "Web Off"
			} else if webOn {
				status = "DNS Off"
			}

			for _, f := range files {
				if filepath.Ext(f.Name()) == ".json" {
					// Parse YAML to find target
					content, _ := p.IO.ReadFile(filepath.Join(webDir, f.Name()))
					var conf WebConfig
					yaml.Unmarshal(content, &conf)

					// Extract info (assuming 1 router/service per file for simplicity)
					domain := strings.TrimSuffix(f.Name(), ".json")
					target := conf.Endpoint

					fmt.Fprintf(w, "%s\t%s\t%s\n", domain, target, status)
				}
			}
			w.Flush()
		},
	}
}
