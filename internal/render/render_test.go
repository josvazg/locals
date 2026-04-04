package render

import (
	"embed"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/*/*.sh
var testDataFS embed.FS

func TestRenderScripts(t *testing.T) {
	state := State{
		DNSListen: "127.1.2.3",
		LocalsDir: "/home/user/.config/locals",
		SystemCA:  "/etc/ssl/certs/ca-certificates.crt",
	}

	tests := []struct {
		name       string
		renderFn   func(State) ([]byte, error)
		goldenFile string
	}{
		{"On Script", On, "on.sh"},
		{"Off Script", Off, "off.sh"},
		{"Add Script", Add, "add.sh"},
		{"Remove Script", Remove, "rm.sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.renderFn(state)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}

			goldenFile := filepath.Join("testdata", runtime.GOOS, tt.goldenFile)
			want, err := testDataFS.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("could not read golden file %s: %v", tt.goldenFile, err)
			}

			assert.Equal(t, string(want), string(got))
		})
	}
}
