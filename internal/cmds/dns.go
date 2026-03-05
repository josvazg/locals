package cmds

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"locals/api/locals"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/spf13/cobra"
)

const (
	resolvConf = "/etc/resolv.conf"

	domain = "locals"

	domainIP = "127.0.0.1"
)

var domainSuffix = fmt.Sprintf(".%s.", domain)

func dnsCmd(ctx context.Context, p *locals.Platform) *cobra.Command {
	return &cobra.Command{
		Use:   "dns address [fallbacks]",
		Short: "Run the locals DNS service",
		Long: "Runs a simple local DNS service, including other servers as fallback.\n" +
			"E.g:\n" +
			"$ dns 127.1.2.3 1.1.1.1,4.4.4.4,8.8.8.8,9.9.9.9",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			listen := args[0]
			fallbacks, err := fallbacks(p, args, listen)
			if err != nil {
				return err
			}
			return runDNS(ctx, listen, fallbacks)
		},
	}
}

func runDNS(ctx context.Context, listenAddr string, fallbacks []string) error {
	handler := dns.HandlerFunc(handlerWithFallbacks(fallbacks))

	server := &dns.Server{
		Addr:    ensurePort(listenAddr),
		Net:     "udp",
		Handler: handler,
	}

	log.Printf("📡 DNS listening on %s (Fallbacks: %v)", listenAddr, fallbacks)
	exitCtx, cancel := context.WithCancel(ctx)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Printf("locals dns failed: %v", err)
		}
		log.Printf("locals dns exits")
		cancel()
	}()
	<-exitCtx.Done()
	log.Println("shutting down server...")
	err := server.Shutdown()
	if err != nil {
		log.Printf("dns shutdown Error: %v\n", err)
	}
	return nil
}

func handlerWithFallbacks(fallbacks []string) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true

		for _, q := range r.Question {
			if strings.HasSuffix(strings.ToLower(q.Name), domainSuffix) {
				if q.Qtype == dns.TypeA {
					rr, _ := dns.NewRR(fmt.Sprintf("%s 60 IN A %s", q.Name, domainIP))
					m.Answer = append(m.Answer, rr)
				}
				w.WriteMsg(m)
				return
			}
		}
		c := new(dns.Client)
		for _, addr := range fallbacks {
			in, _, err := c.Exchange(r, ensurePort(addr))
			if err == nil {
				w.WriteMsg(in)
				return
			}
		}
		dns.HandleFailed(w, r)
	}
}

func ensurePort(addr string) string {
	if !strings.Contains(addr, ":") {
		return net.JoinHostPort(addr, "53")
	}
	return addr
}

func fallbacks(p *locals.Platform, args []string, listen string) ([]string, error) {
	if len(args) > 1 {
		return strings.Split(args[1], ","), nil
	}
	allNameservers, err := nameservers(p, resolvConf)
	if err != nil {
		return nil, fmt.Errorf("failed to detect nameservers: %w", err)
	}
	return removeIfFound(allNameservers, listen), nil
}

func nameservers(p *locals.Platform, path string) ([]string, error) {
	contents, err := p.IO.ReadFile(path)
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
