package web

import (
	"crypto/tls"
	"net/url"
	"sync"
)

type proxyStore struct {
	m         sync.RWMutex
	endpoints map[string]*Endpoint
}

type ProxyStore interface {
	AddTLSEndpoint(host string, url *url.URL, cert *tls.Certificate)
	AddTCPEndpoint(host string, url *url.URL)
	ListHosts() []string
	Endpoint(host string) *Endpoint
	DeleteEndpoint(host string)
}

var TCPPipeNoCert = tls.Certificate{} // explicitly marks an entry as a tcp pipe

func NewProxyStore() ProxyStore {
	return &proxyStore{endpoints: make(map[string]*Endpoint)}
}

func (s *proxyStore) AddTLSEndpoint(host string, url *url.URL, cert *tls.Certificate) {
	s.m.Lock()
	defer s.m.Unlock()
	s.endpoints[host] = &Endpoint{
		URL: url,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*cert},
			MinVersion:   tls.VersionTLS12,
		},
	}
}

func (s *proxyStore) AddTCPEndpoint(host string, url *url.URL) {
	s.m.Lock()
	defer s.m.Unlock()
	s.endpoints[host] = &Endpoint{URL: url}
}

func (s *proxyStore) Endpoint(host string) *Endpoint {
	s.m.RLock()
	defer s.m.RUnlock()
	return s.endpoints[host]
}

func (s *proxyStore) ListHosts() []string {
	s.m.RLock()
	defer s.m.RUnlock()
	hosts := []string{}
	for h := range s.endpoints {
		hosts = append(hosts, h)
	}
	return hosts
}

func (s *proxyStore) DeleteEndpoint(host string) {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.endpoints, host)
}
