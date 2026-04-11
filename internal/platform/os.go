package platform

import (
	"io"
	"os"
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
type Platform interface {
	Stdout() io.Writer
	Stderr() io.Writer
	Stdin() io.Reader
	Env(name string) string
	HomeDir() (string, error)
	IO() FilesHandler
	Proc() Proc
	Run(name string, args ...string) (string, error)
	CheckDNSSetup() *DNSStatus
}

type osPlatform struct {
}

func (_ *osPlatform) Stdout() io.Writer {
	return os.Stdout
}

func (_ *osPlatform) Stderr() io.Writer {
	return os.Stderr
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

func (_ *osPlatform) IO() FilesHandler {
	return newOSFilesHandler()
}

func (_ *osPlatform) Proc() Proc {
	return &osProc{}
}

func (_ *osPlatform) Run(cmd string, args ...string) (string, error) {
	return run(cmd, args...)
}

func (_ *osPlatform) CheckDNSSetup() *DNSStatus {
	return checkDNSSetup()
}

// NewOSPlatform returns a platform implementation backed by the real OS.
func NewOSPlatform() Platform {
	return &osPlatform{}
}
