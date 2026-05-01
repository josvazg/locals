package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type Server struct {
	addr      string
	domain    string
	domainIP  string
	fallbacks []string
	l         *log.Logger
}

func New(addr, domain, domainIP string, fallbacks []string, logger *log.Logger) *Server {
	return &Server{
		addr:      addr,
		domain:    domain,
		domainIP:  domainIP,
		fallbacks: fallbacks,
		l:         logger,
	}
}

func (s *Server) Run(ctx context.Context) error {
	handler := dns.HandlerFunc(handlerWithFallbacks(s.domain, s.domainIP, s.fallbacks))

	server := &dns.Server{
		Addr:    ensurePort(s.addr),
		Net:     "udp",
		Handler: handler,
	}

	s.l.Printf("📡 DNS listening on %s (Fallbacks: %v)", s.addr, s.fallbacks)
	exitCtx, cancel := context.WithCancel(ctx)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			s.l.Printf("locals dns failed: %v", err)
		}
		s.l.Printf("locals dns exits")
		cancel()
	}()
	<-exitCtx.Done()
	s.l.Println("shutting down server...")
	err := server.Shutdown()
	if err != nil {
		s.l.Printf("dns shutdown Error: %v\n", err)
	}
	return nil
}

func handlerWithFallbacks(domain, domainIP string, fallbacks []string) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true

		domainSuffix := fmt.Sprintf(".%s.", domain)
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
