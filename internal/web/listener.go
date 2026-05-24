package web

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type Endpoint struct {
	URL       *url.URL
	TLSConfig *tls.Config
}

type EndpointResolver func(hostname string) *Endpoint

func NewHybridProxy(hl *HybridListener) *http.Server {
	return &http.Server{
		Handler: reverseProxyHandler(hl.Resolver),
	}
}

func reverseProxyHandler(resolver EndpointResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint := resolver(r.Host)
		if endpoint == nil {
			http.Error(w, "Domain not found in locals", http.StatusNotFound)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(endpoint.URL)

		r.URL.Host = endpoint.URL.Host
		r.URL.Scheme = endpoint.URL.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))

		proxy.ServeHTTP(w, r)
	})
}

// HybridListener intercepts raw TCP connections, extracts the SNI hostname,
// and forks traffic based on whether the target scheme is HTTP or HTTPS.
type HybridListener struct {
	net.Listener
	Resolver EndpointResolver
}

func NewHybridListener(ctx context.Context, addr string, resolver EndpointResolver) (*HybridListener, error) {
	lc := net.ListenConfig{}
	baseLn, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	return &HybridListener{
		Listener: baseLn,
		Resolver: resolver,
	}, nil
}

func (hl *HybridListener) Accept() (net.Conn, error) {
	conn, err := hl.Listener.Accept()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	serverName, err := sniffSNI(conn, &buf)

	reConn := &rewoundConn{
		Reader: io.MultiReader(&buf, conn),
		Conn:   conn,
	}

	if err != nil || serverName == "" {
		reConn.Close()
		return hl.Accept() // Skip unrecognized traffic and await next iteration
	}

	endpoint := hl.Resolver(serverName)
	if endpoint == nil {
		reConn.Close()
		return hl.Accept()
	}

	if endpoint.TLSConfig == nil {
		backendConn, err := net.DialTimeout("tcp", endpoint.URL.Host, 2*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %w", endpoint.URL.Host, err)
		}
		go hl.pipeTCP(reConn, backendConn)

		// Return a closedConn so the HTTP server ignores i and lets the TCP 
		// Pipe take over
		return closedConn{Conn: conn}, nil
	}

	tlsConn := tls.Server(reConn, endpoint.TLSConfig)
	return tlsConn, nil
}

func (hl *HybridListener) pipeTCP(clientConn, backendConn net.Conn) {
	defer clientConn.Close()
	defer backendConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { _, _ = io.Copy(backendConn, clientConn); wg.Done() }()
	go func() { _, _ = io.Copy(clientConn, backendConn); wg.Done() }()
	wg.Wait()
}

type closedConn struct {
	net.Conn
}

// Read instantly tells the server: "This connection is dead!"
func (c closedConn) Read(b []byte) (int, error) {
	return 0, io.EOF
}

// Write instantly rejects any data the server tries to send down a passthrough line
func (c closedConn) Write(b []byte) (int, error) {
	return 0, net.ErrClosed
}

// Close is a no-op since it's already dead
func (c closedConn) Close() error {
	return nil
}
