package test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"locals/internal/cfg"
	"locals/internal/mkcert"
	"locals/test/files"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	NumberOfTestServers = 5

	DefaultLocals = "locals"
)

var (
	localsBinary  = DefaultLocals
	testConfigDir string
	testTempDir   string
)

func TestLocals(t *testing.T) {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	servers := startTestServers(t, NumberOfTestServers)
	defer stopTestServers(t, servers)
	testSudo(ctx, t)
	bin, err := filepath.Abs(filepath.Join("..", "bin", DefaultLocals))
	require.NoError(t, err)
	localsBinary = bin

	testConfigDir = t.TempDir()
	testTempDir = t.TempDir()
	os.Setenv("GODEBUG", "netdns=go")

	wasActive := isRealLocalsActive(ctx)
	if wasActive {
		out, err := runRealLocals(ctx, "off")
		require.NoError(t, err, "failed to turn off running locals before test: %s", out)
	}
	// restore runs second (LIFO), after the test-daemon cleanup below
	defer func() {
		if wasActive {
			if out, err := runRealLocals(ctx, "on"); err != nil {
				t.Logf("warning: failed to restore locals after test: %v\n%s", err, out)
			}
		}
	}()
	// stop test daemons on any exit path, including mid-test failures
	defer func() { runLocals(ctx, "off") }() //nolint:errcheck

	testInactive(ctx, t)
	testStart(ctx, t)
	testActive(ctx, t)
	for _, filename := range []string{"locals-dns.log", "locals-web.log"} {
		out, err := os.ReadFile(filepath.Join(testTempDir, filename))
		if err != nil {
			fmt.Printf("failed to read %q: %v", filename, err)
		}
		fmt.Printf("%s:\n%v", filename, string(out))
	}
	testAdds(ctx, t, servers)
	testServers(t, servers)
	testRemovals(ctx, t, servers)
	testOff(ctx, t)
	testInactive(ctx, t)
}

func startTestServers(t *testing.T, n int) []*httptest.Server {
	servers := make([]*httptest.Server, 0, n)
	for i := range n {
		tls := (i%2 == 0)
		servers = append(servers, newTestServer(t, serverName(i), tls))
	}
	return servers
}

func newTestServer(t *testing.T, serverName string, tls bool) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from server %s", serverName)
	})

	if tls {
		server = newCustomTLSServer(t, handler, []string{serverName})
	} else {
		server = httptest.NewServer(handler)
	}
	return server
}

// newCustomTLSServer creates a TLS test server running with a self-signed
// certificate valid for the explicitly provided hostnames/IPs (SANs).
func newCustomTLSServer(t *testing.T, handler http.Handler, allowedNames []string) *httptest.Server {
	t.Helper()

	server := httptest.NewUnstartedServer(handler)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Go Test Server Inc."},
			CommonName:   allowedNames[0], // Primary name
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(1 * time.Hour), // Short-lived for testing
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Categorize the allowed names into DNSNames or IPAddresses
	for _, name := range allowedNames {
		if ip := net.ParseIP(name); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, name)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	serverCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("failed to load x509 key pair: %v", err)
	}

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	server.StartTLS()
	return server
}

func stopTestServers(t *testing.T, servers []*httptest.Server) {
	t.Helper()

	for _, server := range servers {
		log.Printf("shutting down server at %v", server.Listener.Addr())
		server.Close()
	}
}

func testSudo(ctx context.Context, t *testing.T) {
	t.Helper()

	cmd := exec.CommandContext(ctx, "sudo", "-n", "ls")
	cmd.Stdin = nil
	require.NoError(t, cmd.Run(), "please unlock sudo manually access before the test")
}

func testInactive(ctx context.Context, t *testing.T) {
	t.Helper()
	testCmd(ctx, t, loadFile(t, "inactive.out"), "status")
}

func testStart(ctx context.Context, t *testing.T) {
	t.Helper()
	testCmd(ctx, t, loadFile(t, "on.out"), "on")
}

func testActive(ctx context.Context, t *testing.T) {
	t.Helper()
	testCmd(ctx, t, loadFile(t, "active.out"), "status")
}

func testAdds(ctx context.Context, t *testing.T, servers []*httptest.Server) {
	t.Helper()
	serviceList := ""
	addContent := loadFile(t, "add.out")
	for i, server := range servers {
		endpoint := server.Listener.Addr().String()
		url := serverName(i)
		serviceList = fmt.Sprintf("%s  🔗 %s -> %s\n", serviceList, url, endpoint)
		testCmd(ctx, t, addContent, "add", url, endpoint)
	}
	added := loadFile(t, "active.out")
	patchedAdded := strings.Replace(added, `  \(none\)`, serviceList[:len(serviceList)-1], 1)
	testCmd(ctx, t, patchedAdded, "status")
}

func testServers(t *testing.T, servers []*httptest.Server) {
	t.Helper()
	client := testClient(t, servers)
	defer client.CloseIdleConnections()
	for i := range len(servers) {
		url := fmt.Sprintf("https://%s", serverName(i))
		res, err := client.Get(url)
		if err != nil {
			require.NoError(t, err)
		}
		greeting, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			require.NoError(t, err)
		}
		want := fmt.Sprintf("Hello from server %s", serverName(i))
		assert.Equal(t, want, string(greeting))
	}
}

func testClient(t *testing.T, servers []*httptest.Server) *http.Client {
	caPath := filepath.Join(mkcertCARoot(t), "rootCA.pem")
	caCert, err := os.ReadFile(caPath)
	require.NoError(t, err, "failed to read mkcert CA file")

	certPool, err := x509.SystemCertPool()
	if err != nil {
		certPool = x509.NewCertPool()
	}
	certPool.AppendCertsFromPEM(caCert)
	for _, server := range servers {
		if server.TLS == nil {
			continue
		}
		for _, chain := range server.TLS.Certificates {
			if chain.Certificate == nil {
				continue
			}
			for _, crt := range chain.Certificate {
				cert, err := x509.ParseCertificate(crt)
				require.NoError(t, err)
				certPool.AddCert(cert)
			}
		}
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}
}

func testRemovals(ctx context.Context, t *testing.T, servers []*httptest.Server) {
	t.Helper()
	addContent := loadFile(t, "rm.out")
	for i := range servers {
		url := serverName(i)
		testCmd(ctx, t, addContent, "rm", url)
	}
}

func testOff(ctx context.Context, t *testing.T) {
	t.Helper()
	testCmd(ctx, t, loadFile(t, "off.out"), "off")
}

func loadFile(t *testing.T, filename string) string {
	content, err := files.FS.ReadFile(filename)
	require.NoError(t, err)
	return string(content)
}

func testCmd(ctx context.Context, t *testing.T, want string, args ...string) {
	t.Helper()

	got, err := runLocals(ctx, args...)
	require.NoError(t, err, "failed to test command: %v\n got %v",
		strings.Join(args, " "), string(got))
	assert.Regexp(t, regexp.MustCompile(string(want)), string(got))
}

func runLocals(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, localsBinary, args...)
	cmd.Env = append(os.Environ(),
		cfg.EnvLocalsConfigDir+"="+testConfigDir,
		cfg.EnvLocalsTempDir+"="+testTempDir,
	)
	return cmd.CombinedOutput()
}

func runRealLocals(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, localsBinary, args...)
	return cmd.CombinedOutput()
}

func isRealLocalsActive(ctx context.Context) bool {
	out, err := runRealLocals(ctx, "status")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "RUNNING")
}

func mkcertCARoot(t *testing.T) string {
	t.Helper()

	caroot, err := mkcert.New(os.Stdout).CARoot()
	require.NoError(t, err)
	return strings.TrimSpace(caroot)
}

func envOrDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		value = defaultValue
	}
	return value
}

func serverName(i int) string {
	return fmt.Sprintf("service-%c.locals", 'a'+byte(i))
}
