package cfg

const (
	// EnvLocalsConfigDir overrides the default ~/.config/locals state directory.
	EnvLocalsConfigDir = "LOCALS_CONFIG_DIR"

	// EnvLocalsTempDir overrides the default OS temp dir
	EnvLocalsTempDir = "LOCALS_TEMP_DIR"
)

type Config struct {
	DNSListen string
	LocalsDir string
	SystemCA  string
	LocalsBin string
	TempDir   string
	Dryrun    bool
}
