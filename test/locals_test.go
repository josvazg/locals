package test

import (
	"context"
	"fmt"
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
)

func TestLocals(t *testing.T) {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	servers := startTestServers(t, NumberOfTestServers)
	defer stopTestServers(t, servers)
	testSudo(ctx, t)
	testInactive(ctx, t)
	testOn(ctx, t)
	testActive(ctx, t)
	testAdds(ctx, t, servers)
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

func testOn(ctx context.Context, t *testing.T) {
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
		parts := strings.Split(endpoint, ":")
		port := parts[len(parts)-1]
		url := fmt.Sprintf("service-%s.locals", port)
		serviceList = fmt.Sprintf("%s  🔗 %s -> %s\n", serviceList, url, endpoint)
		testCmd(ctx, t, addContent, "add", url, endpoint)
	}		
	added := loadFile(t, "active.out")
	patchedAdded := strings.Replace(added, `  \(none\)`, serviceList[:len(serviceList)-1], 1)
	log.Printf("patchedAdded:\n%s", patchedAdded)
	testCmd(ctx, t, patchedAdded, "status")
}

func testRemovals(ctx context.Context, t *testing.T, servers []*httptest.Server) {
	t.Helper()
	addContent := loadFile(t, "rm.out")
	for _, server := range sortByPort(servers) {
		endpoint := server.Listener.Addr().String()
		parts := strings.Split(endpoint, ":")
		port := parts[len(parts)-1]
		url := fmt.Sprintf("service-%s.locals", port)
		testCmd(ctx, t, addContent, "rm", url)
	}		
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

	got, err := runLocals(ctx, t, args...)
	log.Print(string(got))
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(string(want)), string(got))
}

func runLocals(ctx context.Context, t *testing.T, args ...string) ([]byte, error) {
	allArgs := append([]string{"run", ".."}, args...)
	cmd := exec.CommandContext(ctx, "go", allArgs...)
	cmd.Env = testEnv(t)
	return cmd.CombinedOutput()
}

func testEnv(t *testing.T) []string {
	env := os.Environ()
	newPath := "PATH=" + binDir(t) + string(os.PathListSeparator) + os.Getenv("PATH")
	env = append(env, newPath)
	return env
}

func binDir(t *testing.T) string {
	pwd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Clean(filepath.Join(pwd, "..", "bin"))
}
