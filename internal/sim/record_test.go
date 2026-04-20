package sim_test

import (
	"context"
	"fmt"
	"locals/internal/cmds"
	"locals/internal/platform"
	"locals/internal/sim"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecorder(t *testing.T) {
	if !enabled(os.Getenv("RECORDING_TEST")) {
		t.Skip("Set RECORDING_TEST to true to run this test")
	}
	ctx := context.Background()
	localsBinPath := os.Getenv("LOCALSBINPATH")
	require.NotEmpty(t, localsBinPath, "LOCALSBINPATH env var must be set")
	oldpath := os.Getenv("PATH")
	defer func() {
		os.Setenv("PATH", oldpath)
	}()
	os.Setenv("PATH", fmt.Sprintf("%s:%s", localsBinPath, oldpath))
	p := sim.NewRecorderPlatform(platform.NewOSPlatform())
	require.NoError(t, p.Clear())

	p.Record(sim.OpCommandMarker, []any{"locals status"}, nil)
	require.NoError(t, cmds.Run(ctx, p, []string{"locals", "status"}))

	p.Record(sim.OpCommandMarker, []any{"locals on"}, nil)
	require.NoError(t, cmds.Run(ctx, p, []string{"locals", "on"}))

	p.Record(sim.OpCommandMarker, []any{"locals status"}, nil)
	require.NoError(t, cmds.Run(ctx, p, []string{"locals", "status"}))

	p.Record(sim.OpCommandMarker, []any{"locals off"}, nil)
	require.NoError(t, cmds.Run(ctx, p, []string{"locals", "off"}))

	p.Record(sim.OpCommandMarker, []any{"locals status"}, nil)
	require.NoError(t, cmds.Run(ctx, p, []string{"locals", "status"}))
}

func enabled(value string) bool {
	return value == "1" ||
		strings.ToLower(value) == "true" ||
		strings.ToLower(value) == "yes"
}
