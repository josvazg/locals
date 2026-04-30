package cmds

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"locals/internal/dns"
	"log"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	resolvConf = "/etc/resolv.conf"

	domain = "locals"

	domainIP = "127.0.0.1"
)

func dnsCmd(ctx context.Context) *cobra.Command {
	var logFile string
	cmd := &cobra.Command{
		Use:   "dns address [fallbacks]",
		Short: "Run the locals DNS service",
		Long: "Runs a simple local DNS service, including other servers as fallback\n" +
			"Example:\n\n" +
			"  locals dns 127.1.2.3 1.1.1.1,4.4.4.4,8.8.8.8,9.9.9.9",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := log.Default()
			if logFile != "" {
				f, err := loggerFile(logFile)
				if err != nil {
					return fmt.Errorf("failed to setup log file: %w", err)
				}
				logger = log.New(f, "", log.LstdFlags)
				defer f.Close()
			}
			listen := args[0]
			fallbacks, err := fallbacks(args, listen)
			if err != nil {
				return err
			}
			cmd.SilenceUsage = true
			return dns.New(listen, domain, domainIP, fallbacks, logger).Run(ctx)
		},
	}
	cmd.Flags().StringVarP(&logFile, "log", "", "", "file to log to")
	return cmd
}

func loggerFile(logFile string) (*os.File, error) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed open logger file: %w", err)
	}
	return f, nil
}

func fallbacks(args []string, listen string) ([]string, error) {
	if len(args) > 1 {
		return strings.Split(args[1], ","), nil
	}
	allNameservers, err := nameservers(resolvConf)
	if err != nil {
		return nil, fmt.Errorf("failed to detect nameservers: %w", err)
	}
	return removeIfFound(allNameservers, listen), nil
}

func nameservers(path string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var nameservers []string
	scanner := bufio.NewScanner(bytes.NewBuffer(contents))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			nameservers = append(nameservers, fields[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nameservers, nil
}

func removeIfFound(nameservers []string, listen string) []string {
	listenIP, _, err := net.SplitHostPort(listen)
	if err != nil {
		listenIP = listen
	}

	result := make([]string, 0, len(nameservers))
	for _, ns := range nameservers {
		if ns != listenIP {
			result = append(result, ns)
		}
	}
	return result
}
