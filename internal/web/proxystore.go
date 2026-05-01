package web

import (
	"crypto/tls"
	"net/url"
	"sync"
)

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
