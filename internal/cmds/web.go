package cmds

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
	"locals/internal/platform"
	"log"
	"math/big"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

const (
	DefaultConfigDir = "web"

	// Localhost only (IPv4 loopback; matches *.locals A records). Using 127.0.0.1 avoids :443 binding all interfaces.
	ListenAddr = "127.0.0.1:443"
)

var (
	ErrNotFound = errors.New("not found")
)

type WebConfig struct {
	URL      string `json:"url"`      // e.g., myservice.locals
	Endpoint string `json:"endpoint"` // e.g., localhost:8080
	CertFile string `json:"cert"`     // path to pem
	KeyFile  string `json:"key"`      // path to key
}

type proxyStore struct {
	m      sync.RWMutex
	routes map[string]*url.URL
	certs  map[string]*tls.Certificate
}

type ProxyStore interface {
	AddEndpoint(host string, url *url.URL, cert *tls.Certificate)
	Endpoint(host string) (*url.URL, error)
	Cert(host string) (*tls.Certificate, error)
	ListHosts() []string
	DeleteEndpoint(host string)
}

func (s *proxyStore) AddEndpoint(host string, endpoint *url.URL, cert *tls.Certificate) {
	s.m.Lock()
	defer s.m.Unlock()
	s.routes[host] = endpoint
	s.certs[host] = cert
}

func (s *proxyStore) Endpoint(host string) (*url.URL, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	endpoint, ok := s.routes[host]
	if !ok {
		return nil, ErrNotFound
	}
	return endpoint, nil
}

func (s *proxyStore) Cert(host string) (*tls.Certificate, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	cert, ok := s.certs[host]
	if !ok {
		return nil, ErrNotFound
	}
	return cert, nil

}

func (s *proxyStore) ListHosts() []string {
	s.m.RLock()
	defer s.m.RUnlock()
	hosts := []string{}
	for h := range s.certs {
		hosts = append(hosts, h)
	}
	return hosts
}

func (s *proxyStore) DeleteEndpoint(host string) {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.routes, host)
	delete(s.certs, host)
}

func webCmd(ctx context.Context, p platform.Platform, cfgDir string) *cobra.Command {
	var logFile string
	cmd := &cobra.Command{
		Use:   "web [configDir]",
		Short: "Run the locals Web reverse proxy service",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if logFile != "" {
				if err := setupLog(p, logFile); err != nil {
					return fmt.Errorf("failed to setup log file: %w", err)
				}
			}
			webDir := DefaultConfigDir
			if len(args) > 0 {
				webDir = args[0]
			}
			cmd.SilenceUsage = true
			return runWeb(ctx, p, ensureAbsolutePath(webDir, cfgDir))
		},
	}
	cmd.Flags().StringVarP(&logFile, "log", "", "", "file to log to")
	return cmd
}

func runWeb(ctx context.Context, p platform.Platform, webDir string) error {
	store := &proxyStore{
		routes: make(map[string]*url.URL),
		certs:  make(map[string]*tls.Certificate),
	}

	if err := loadConfig(p, store, webDir); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	server := &http.Server{
		Addr:    ListenAddr,
		Handler: http.HandlerFunc(reverseProxy(store)),
		TLSConfig: &tls.Config{
			GetCertificate: getCertificate(store),
		},
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create config watcher: %w", err)
	}
	defer watcher.Close()
	if err := watcher.Add(webDir); err != nil {
		return fmt.Errorf("failed to watch web config dir %q: %w", webDir, err)
	}
	go detectChangesLoop(ctx, p, store, webDir, watcher)

	log.Printf("🚀 Web Proxy listening on %s", ListenAddr)
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil { // Certs are handled by GetCertificate
			log.Printf("locals web proxy listen failed: %v", err)
		}
		log.Printf("locals web proxy server exits")
	}()
	<-ctx.Done()
	log.Println("shutting down Web Proxy...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("locals web proxy shutdown failed: %v\n", err)
	}
	log.Println("locals web proxy stopped")
	return nil
}

func ensureAbsolutePath(dir, cfgDir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(cfgDir, filepath.Clean(dir))
}

func loadConfig(p platform.Platform, store ProxyStore, webDir string) error {
	log.Printf("loading web configs from %s", webDir)
	files, err := filepath.Glob(filepath.Join(webDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list web JSON files: %w", err)
	}
	hosts := map[string]struct{}{}
	ensureProbeCert(store, hosts)
	for _, f := range files {
		data, err := p.FS().ReadFile(f)
		if err != nil {
			return fmt.Errorf("read web config %s: %w", f, err)
		}
		var cfg WebConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("decode web config %s: %w", f, err)
		}

		target, err := url.Parse(ensureProtocol(cfg.Endpoint))
		if err != nil {
			return fmt.Errorf("parse endpoint for %s (%s): %w", cfg.URL, f, err)
		}

		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("load cert for %s (%s): %w", cfg.URL, f, err)
		}

		store.AddEndpoint(cfg.URL, target, &cert)
		log.Printf("loaded config https://%s -> %s", cfg.URL, cfg.Endpoint)
		hosts[cfg.URL] = struct{}{}
	}
	for _, h := range store.ListHosts() {
		_, ok := hosts[h]
		if !ok {
			log.Printf("unload web config for %q", h)
			store.DeleteEndpoint(h)
		}
	}
	return nil
}

func ensureProbeCert(store ProxyStore, hosts map[string]struct{}) error {
	crt, err := store.Cert("probe")
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("failed to check probe cert: %w", err)
	}
	if crt == nil {
		store.AddEndpoint("probe", nil, newProbeCert())
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

func reverseProxy(store ProxyStore) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		target, err := store.Endpoint(r.Host)
		if err != nil {
			if err == ErrNotFound {
				http.Error(w, "Domain not found in locals", http.StatusNotFound)
				return
			}
			log.Printf("failed to check SNI %q: %v", r.Host, err)
			http.Error(w, "Error checking domain in locals", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		// Update headers so the backend sees the correct host
		r.URL.Host = target.Host
		r.URL.Scheme = target.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))

		proxy.ServeHTTP(w, r)
	}
}

func getCertificate(store ProxyStore) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if cert, err := store.Cert(hello.ServerName); err != nil {
			return nil, fmt.Errorf("no certificate for %s: %w\n%v", hello.ServerName, err, store.ListHosts())
		} else {
			return cert, nil
		}
	}
}

func detectChangesLoop(ctx context.Context, p platform.Platform, store ProxyStore, webDir string, watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Ignore chmod, only care about data changes
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				log.Println("Config changed, reloading routes...")
				if err := loadConfig(p, store, webDir); err != nil {
					log.Printf("reload failed: %v", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		case <-ctx.Done():
			log.Printf("detect changes loop exits")
			return
		}
	}
}
