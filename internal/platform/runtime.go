package platform

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
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
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Env    EnvFunc
	HomeDir func() (string, error)
	IO       FilesHandler
	Process  ProcessInfo
	Execute  func(name string, args ...string) error
	CheckDNSSetup func() *DNSStatus
}

type DNSStatus struct {
	Active bool
	Status string
}

func (s *DNSStatus) String() string {
	icon := "🔓"
	if !s.Active {
		icon = "⚠️"
	}
	return fmt.Sprintf("%s %s", icon, s.Status)
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
		CheckDNSSetup: func() func() *DNSStatus {
			if runtime.GOOS == "darwin" {
				return checkMacDNSSetup
			}
			return checkLinuxDNSSetup
		}(),
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

func checkLinuxDNSSetup() *DNSStatus {
	dnsMode := "INACTIVE"
	dnsResolvedConfig, err := os.ReadFile("/etc/systemd/resolved.conf.d/locals.conf")
	active := err == nil && len(dnsResolvedConfig) > 0

	if active {
		dnsMode = "RESOLVED CONFIG ACTIVE"
	} else {
		mounts, err := os.ReadFile("/proc/self/mountinfo")
		if err != nil {
			dnsMode = fmt.Sprintf("failed to check mounts: %v", err)
		}
		if strings.Contains(string(mounts), "/resolv.patched.conf") {
			dnsMode = "BIND-MOUNT ACTIVE"
			active = true
		}
	}
	return &DNSStatus{
		Active: active,
		Status: dnsMode,
	}
}

func checkMacDNSSetup() *DNSStatus {
	dnsMode := ""
	dnsConfig, err := os.ReadFile("/etc/resolver/locals")
	hasFile := err == nil && len(dnsConfig) > 0
	hasAlias := IsIPOnInterface("lo0", DefaultDNSListen)

	switch {
	case hasFile && hasAlias:
		dnsMode = "RESOLVER REDIRECT ACTIVE"
	case !hasFile && !hasAlias:
		dnsMode = "INACTIVE"
	case !hasAlias:
		dnsMode = "MISSING DNS IP ALIAS"
	case !hasFile:
		dnsMode = "MISSING DNS CONFIG FILE"
	}
	return &DNSStatus{
		Active: hasFile && hasAlias,
		Status: dnsMode,
	}
}
