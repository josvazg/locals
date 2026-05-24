package web

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	ErrNotFound = errors.New("not found")
)

type Server struct {
	addr  string
	dir   string
	l     *log.Logger
	store ProxyStore
}

type Config struct {
	URL      string `json:"url"`      // e.g., myservice.locals
	Endpoint string `json:"endpoint"` // e.g., localhost:8080
	CertFile string `json:"cert"`     // path to pem
	KeyFile  string `json:"key"`      // path to key
}

func New(addr, dir string, logger *log.Logger) *Server {
	return &Server{
		addr: addr,
		dir:  dir,
		l:    logger,
		store: NewProxyStore(),
	}
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	hl, err := NewHybridListener(ctx, s.addr, s.store.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to start loistener at %s: %w", s.addr, err)
	}
	proxy := NewHybridProxy(hl)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create config watcher: %w", err)
	}
	defer watcher.Close()
	if err := watcher.Add(s.dir); err != nil {
		return fmt.Errorf("failed to watch web config dir %q: %w", s.dir, err)
	}
	go s.detectChangesLoop(ctx, watcher)

	s.l.Printf("🚀 Web Proxy listening on %s", s.addr)
	go func() {
		if err := proxy.Serve(hl); err != nil { // Certs are handled by GetCertificate
			s.l.Printf("locals web proxy listen failed: %v", err)
		}
		s.l.Printf("locals web proxy server exits")
	}()
	<-ctx.Done()
	s.l.Println("shutting down Web Proxy...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := proxy.Shutdown(shutdownCtx); err != nil {
		s.l.Printf("locals web proxy shutdown failed: %v\n", err)
	}
	s.l.Println("locals web proxy stopped")
	return nil
}

func (s *Server) loadConfig() error {
	s.l.Printf("loading web configs from %s", s.dir)
	files, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list web JSON files: %w", err)
	}
	hosts := map[string]struct{}{}
	ensureProbeCert(s.store, hosts)
	for _, filename := range files {
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read web config %s: %w", filename, err)
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("decode web config %s: %w", filename, err)
		}

		target, err := url.Parse(ensureProtocol(cfg.Endpoint))
		if err != nil {
			return fmt.Errorf("parse endpoint for %s (%s): %w", cfg.URL, filename, err)
		}
		if cfg.CertFile == "" && cfg.KeyFile == "" {
			s.store.AddTCPEndpoint(cfg.URL, target)
			s.l.Printf("loaded config tcp://%s -> %s", cfg.URL, cfg.Endpoint)
		} else {
			cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
			if err != nil {
				return fmt.Errorf("load cert for %s (%s): %w", cfg.URL, filename, err)
			}
			s.store.AddTLSEndpoint(cfg.URL, target, &cert)
			s.l.Printf("loaded config https://%s -> %s", cfg.URL, cfg.Endpoint)
		}
		hosts[cfg.URL] = struct{}{}
	}
	for _, h := range s.store.ListHosts() {
		_, ok := hosts[h]
		if !ok {
			s.l.Printf("unload web config for %q", h)
			s.store.DeleteEndpoint(h)
		}
	}
	return nil
}

func ensureProbeCert(store ProxyStore, hosts map[string]struct{}) error {
	endpoint := store.Endpoint("probe")
	if endpoint == nil {
		store.AddTLSEndpoint("probe", nil, newProbeCert())
	}
	hosts["probe"] = struct{}{}
	return nil
}

func newProbeCert() *tls.Certificate {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil
	}
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"locals-internal"},
			CommonName:   "probe",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365), // 1 year
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"probe"},
	}
	derBytes, err := x509.CreateCertificate(
		rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil
	}
	return &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}
}

func ensureProtocol(addr string) string {
	if !strings.HasPrefix(addr, "http") {
		return "http://" + addr
	}
	return addr
}

func (s *Server) getCertificate() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		endpoint := s.store.Endpoint(hello.ServerName)
		if endpoint == nil {
			return nil, fmt.Errorf("no endpoint for %s: %v", hello.ServerName, s.store.ListHosts())
		}
		if endpoint.TLSConfig == nil || len(endpoint.TLSConfig.Certificates) == 0 {
			return nil, fmt.Errorf("no cert for %s", hello.ServerName)
		}
		cert := endpoint.TLSConfig.Certificates[0]
		return &cert, nil
	}
}

func (s *Server) detectChangesLoop(ctx context.Context, watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Ignore chmod, only care about data changes
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				s.l.Println("Config changed, reloading routes...")
				if err := s.loadConfig(); err != nil {
					s.l.Printf("reload failed: %v", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.l.Println("error:", err)
		case <-ctx.Done():
			s.l.Printf("detect changes loop exits")
			return
		}
	}
}
