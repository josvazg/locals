package cmds

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"locals/api/locals"
	"log"
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

	ListenAddr = ":443"
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
	for h := range s.routes {
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

func webCmd(ctx context.Context, p *locals.Platform, cfgDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "web [configDir]",
		Short: "Run the locals Web reverse proxy service",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			webDir := DefaultConfigDir
			if len(args) > 0 {
				webDir = args[0]
			}
			return runWeb(ctx, p, ensureAbsolutePath(webDir, cfgDir))
		},
	}
}

func runWeb(ctx context.Context, p *locals.Platform, webDir string) error {
	store := &proxyStore{
		routes: make(map[string]*url.URL),
		certs:  make(map[string]*tls.Certificate),
	}

	if err := loadConfig(p, store, webDir); err != nil {
		return fmt.Errorf("failed to load config: %s", err)
	}

	server := &http.Server{
		Addr:    ListenAddr,
		Handler: http.HandlerFunc(reverseProxy(store)),
		TLSConfig: &tls.Config{
			GetCertificate: getCertificate(store),
		},
	}

	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()
	watcher.Add(webDir)
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

func loadConfig(p *locals.Platform, store ProxyStore, webDir string) error {
	log.Printf("loading web configs from %s", webDir)
	files, _ := filepath.Glob(filepath.Join(webDir, "*.json"))
	hosts := map[string]struct{}{}
	for _, f := range files {
		data, _ := p.IO.ReadFile(f)
		var cfg WebConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to load web config: %w", err)
		}

		target, _ := url.Parse(ensureProtocol(cfg.Endpoint))

		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("⚠️  Failed to load cert for %s: %v", cfg.URL, err)
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
			return nil, fmt.Errorf("no certificate for %s: %w", hello.ServerName, err)
		} else {
			return cert, nil
		}
	}
}

func detectChangesLoop(ctx context.Context, p *locals.Platform, store ProxyStore, webDir string, watcher *fsnotify.Watcher) {
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
