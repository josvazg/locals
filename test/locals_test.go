package test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"locals/internal/cfg"
	"locals/internal/mkcert"
	"locals/test/files"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	NumberOfTestServers = 3

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
	for range n {
		server := newTestServer(t)
		log.Printf("started server at %s", server.Listener.Addr().String())
		servers = append(servers, server)
	}
	return servers
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	addr := ":?"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from server at %s", addr)
	}))
	addr = server.Listener.Addr().String()
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
	for _, server := range sortByPort(servers) {
		endpoint := server.Listener.Addr().String()
		url := serviceURL(portFrom(endpoint))
		serviceList = fmt.Sprintf("%s  🔗 %s -> %s\n", serviceList, url, endpoint)
		testCmd(ctx, t, addContent, "add", url, endpoint)
	}
	added := loadFile(t, "active.out")
	patchedAdded := strings.Replace(added, `  \(none\)`, serviceList[:len(serviceList)-1], 1)
	testCmd(ctx, t, patchedAdded, "status")
}

func testServers(t *testing.T, servers []*httptest.Server) {
	t.Helper()
	client := testClient(t)
	defer client.CloseIdleConnections()
	for _, server := range servers {
		endpoint := server.Listener.Addr().String()
		url := fmt.Sprintf("https://%s", serviceURL(portFrom(endpoint)))
		res, err := client.Get(url)
		if err != nil {
			require.NoError(t, err)
		}
		greeting, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			require.NoError(t, err)
		}
		want := fmt.Sprintf("Hello from server at %s", server.Listener.Addr())
		assert.Equal(t, want, string(greeting))
	}
}

func testClient(t *testing.T) *http.Client {
	caPath := filepath.Join(mkcertCARoot(t), "rootCA.pem")
	caCert, err := os.ReadFile(caPath)
	require.NoError(t, err, "failed to read mkcert CA file")

	certPool, err := x509.SystemCertPool()
	if err != nil {
		certPool = x509.NewCertPool()
	}
	certPool.AppendCertsFromPEM(caCert)

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
	for _, server := range sortByPort(servers) {
		endpoint := server.Listener.Addr().String()
		url := fmt.Sprintf("service-%s.locals", portFrom(endpoint))
		testCmd(ctx, t, addContent, "rm", url)
	}
}

func portFrom(addr string) string {
	parts := strings.Split(addr, ":")
	return parts[len(parts)-1]
}

func serviceURL(port string) string {
	return fmt.Sprintf("service-%s.locals", port)
}

func sortByPort(servers []*httptest.Server) []*httptest.Server {
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Listener.Addr().String() < servers[j].Listener.Addr().String()
	})
	return servers
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
