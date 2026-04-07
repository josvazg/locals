package platform

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// Configuration paths and defaults for locals state under the user's config dir.
const (
	DirName  = ".config/locals"
	WebDir   = "web"
	CertsDir = "certs"

	DefaultDNSListen = "127.1.2.3"
)

// Platform defines everything the tool needs from the operating system.
// Swap these out to run the app in tests or with recorded behavior.
type Platform struct {
	Stdout        io.Writer
	Stderr        io.Writer
	Stdin         io.Reader
	Env           EnvFunc
	HomeDir       func() (string, error)
	IO            FilesHandler
	Process       ProcessInfo
	Execute       func(name string, args ...string) error
	CheckDNSSetup func() *DNSStatus
}

// RealOSPlatform returns a platform implementation backed by the real OS.
func RealOSPlatform() *Platform {
	return &Platform{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Env:     os.Getenv,
		HomeDir: os.UserHomeDir,
		IO:      &osFileshandler{},
		Process: &osProcessInfo{},
		Execute: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
		CheckDNSSetup: checkDNSSetup,
	}
}

type ProcessInfo interface {
	IsProcessAlive(pid int) bool
}

type osProcessInfo struct{}

func (pi *osProcessInfo) IsProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))

	if err == nil {
		return true
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if errors.Is(err, syscall.EPERM) {
			return true
		}
		if errors.Is(err, syscall.ESRCH) {
			return false
		}
	}
	return false
}
