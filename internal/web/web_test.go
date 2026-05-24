package web_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"locals/internal/web" // Update if your go.mod module name is different
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHybridProxy(t *testing.T) {
	echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	httpBackend := httptest.NewServer(echoHandler)
	defer httpBackend.Close()

	httpsBackend := httptest.NewTLSServer(echoHandler)
	defer httpsBackend.Close()

	httpURL, err := url.Parse(httpBackend.URL)
	require.NoError(t, err)
	httpsURL, err := url.Parse(httpsBackend.URL)
	require.NoError(t, err)

	routingTable := map[string]*web.Endpoint{
		"app-http.local": {
			URL:       httpURL,
			TLSConfig: httpsBackend.TLS,
		},
		"app-https.local": {
			URL:       httpsURL,
			TLSConfig: nil,
		},
	}

	resolver := func(hostname string) *web.Endpoint {
		return routingTable[hostname]
	}

	ctx := context.Background()
	hl, err := web.NewHybridListener(ctx, "127.0.0.1:0", resolver)
	proxy := web.NewHybridProxy(hl)
	go func() {
		err := proxy.Serve(hl)
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		require.NoError(t, err)
	}()
	proxyAddress := hl.Addr().String()

	t.Run("HTTPS Reverse Proxy to HTTP Backend path", func(t *testing.T) {
		client := createSNIClient(proxyAddress, "app-http.local")
		assertEchoSequence(t, client, "app-http.local")
	})

	t.Run("TCP Passthrough to Secure Backend path", func(t *testing.T) {
		client := createSNIClient(proxyAddress, "app-https.local")
		assertEchoSequence(t, client, "app-https.local")
	})
	require.NoError(t, proxy.Shutdown(ctx))
}

func assertEchoSequence(t *testing.T, client *http.Client, targetHost string) {
	payload := fmt.Sprintf("hi-payload-%d", rand.IntN(100000))

	reqUrl := fmt.Sprintf("https://%s/", targetHost)
	req, err := http.NewRequest("POST", reqUrl, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Failed to build request payload: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request execution failed on host %s: %v", targetHost, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	responseString := string(bodyBytes)

	assert.Equal(t, payload, responseString,
		"Mismatched echo response.\nSent: %q\nGot:  %q", payload, responseString)
}

func createSNIClient(proxyAddr, targetHost string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         targetHost,
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "tcp", proxyAddr)
			},
		},
	}
}
