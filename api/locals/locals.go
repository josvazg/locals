package locals

import (
	"io"
	"io/fs"
	"os"
	"os/exec"

	"locals/internal/env"
)

// --- CONFIGURATION ---
const (
	DirName  = ".config/locals"
	WebDir   = "web"
	CertsDir = "certs"
	LogsDir  = "logs"
)

const (
	ENV_VAR_LOCALS_CONFIG_DIR = "LOCALS_CONFIG_DIR"
)

// Platform defines everything the tool needs from the Operating System.
// By swapping these out, we can test the entire app in memory.
type Platform struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	// Env is a function that retrieves environment variables
	Env env.EnvFunc
	// HomeDir is a function to find the user's home without hardcoding os.UserHomeDir
	HomeDir func() (string, error)
	IO      FilesHandler
	// Execute is a function to run external binaries (mkcert, traefik)
	Execute func(name string, args ...string) error
}

// Returns a real OS platform impementation
func RealOSPlatform() *Platform {
	return &Platform{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Env:     os.Getenv,
		HomeDir: os.UserHomeDir,
		IO:      &osFileshandler{},
		Execute: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			cmd.Stdout = os.Stdout // Uncomment to see mkcert output
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
	}
}

type FilesHandler interface {
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm fs.FileMode) error
	ReadDir(dirname string) ([]fs.DirEntry, error)
	MkdirAll(dirname string, perm fs.FileMode) error
	Stat(filename string) (os.FileInfo, error)
	Remove(filename string) error
}

type osFileshandler struct{}

func OSFileHandler() *osFileshandler {
	return &osFileshandler{}
}

func (osf *osFileshandler) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (osf *osFileshandler) WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

func (osf *osFileshandler) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return os.ReadDir(dirname)
}

func (osf *osFileshandler) MkdirAll(dirname string, perm fs.FileMode) error {
	return os.MkdirAll(dirname, perm)
}

func (osf *osFileshandler) Stat(filename string) (os.FileInfo, error) {
	return os.Stat(filename)
}

func (osf *osFileshandler) Remove(filename string) error {
	return os.Remove(filename)
}
