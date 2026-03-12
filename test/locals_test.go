package test

import (
	"context"
	"locals/test/files"
	"log"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocals(t *testing.T) {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	testInactive(ctx, t)
	testOn(ctx, t)
	testActive(ctx, t)
}

func testInactive(ctx context.Context, t *testing.T) {
	t.Helper()
	testCmd(ctx, t, "inactive.out", "status")
}

func testOn(ctx context.Context, t *testing.T) {
	t.Helper()
	testCmd(ctx, t, "on.out", "on")
}

func testActive(ctx context.Context, t *testing.T) {
	t.Helper()
	testCmd(ctx, t, "active.out", "status")
}

func testCmd(ctx context.Context, t *testing.T, wantFile string, args ... string) {
	t.Helper()

	got, err := runLocals(ctx, args...)
	log.Print(string(got))
	require.NoError(t, err)
	want, err0 := files.FS.ReadFile(wantFile)
	require.NoError(t, err0)
	assert.Regexp(t, regexp.MustCompile(string(want)), string(got))
}

func runLocals(ctx context.Context, args ...string) ([]byte, error) {
	allArgs := append([]string{"run", ".."}, args...)
	cmd := exec.CommandContext(ctx, "go", allArgs...)
	return cmd.CombinedOutput()
}
