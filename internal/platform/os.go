package platform

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Configuration paths and defaults for locals state under the user's config dir.
const (
	DirName  = ".config/locals"
	WebDir   = "web"
	CertsDir = "certs"

	DefaultDNSListen = "127.1.2.3"
)

type osPlatform struct {
	stdout io.Writer
	stderr io.Writer
}

func (osp *osPlatform) Stdout() io.Writer {
	return osp.stdout
}

func (osp *osPlatform) Stderr() io.Writer {
	return osp.stderr
}

func (_ *osPlatform) Stdin() io.Reader {
	return os.Stdin
}

func (_ *osPlatform) Env(name string) string {
	return os.Getenv(name)
}

func (_ *osPlatform) HomeDir() (string, error) {
	return os.UserHomeDir()
}

func (_ *osPlatform) FS() Filesystem {
	return newOSFilesystem()
}

func (_ *osPlatform) Run(cmd string, args ...string) (string, error) {
	return run(cmd, args...)
}

func ( *osPlatform) Test(cmd string, args ...string) (string, error) {
	return run(cmd, args...)
}

func run(cmd string, args ...string) (string, error) {
	fullCmd := fullCmd(cmd, args...)
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %s (%v): %w\n%s", 
		fullCmd, append([]string{cmd}, args...), err, string(out))
	}
	return string(out), err
}

// NewOSPlatform returns a platform implementation backed by the real OS.
func NewOSPlatform() Platform {
	return &osPlatform{stdout: os.Stdout, stderr: os.Stderr}
}
