package locals

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"locals/internal/env"
)

// --- CONFIGURATION ---
const (
	DirName  = ".config/locals"
	WebDir   = "web"
	CertsDir = "certs"

	DefaultDNSListen = "127.1.2.3"
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
	Process ProcessInfo
	// Execute is a function to run external binaries (mkcert, traefik)
	Execute       func(name string, args ...string) error
	CheckDNSSetup func()*DNSStatus
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

// Returns a real OS platform impementation
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
			cmd.Stdout = os.Stdout // Uncomment to see mkcert output
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
		CheckDNSSetup: func() func()*DNSStatus {
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

type osProcessInfo struct {
}

func (pi *osProcessInfo) IsProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal(0) doesn't send a signal, but performs error checking.
	err = process.Signal(syscall.Signal(0))

	if err == nil {
		// On both Linux and macOS, nil error means the process exists
		// and we have permission to talk to it.
		return true
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		// If we get EPERM (Operation not permitted), the process IS alive.
		// It just means it's owned by another user (like root).
		if errors.Is(err, syscall.EPERM) {
			return true
		}

		// If we get ESRCH (No such process), it's definitely dead.
		if errors.Is(err, syscall.ESRCH) {
			return false
		}
	}

	// On macOS, FindProcess always succeeds, so we rely entirely on
	// the Signal results above. If we get here, the process is likely dead.
	return false
}

func checkLinuxDNSSetup() *DNSStatus {
	active := false
	dnsMode := "INACTIVE"
	mounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		dnsMode = fmt.Sprintf("failed to check mounts: %v", err)
	}
	if strings.Contains(string(mounts), "/stub-resolv.conf ") {
		dnsMode = "BIND-MOUNT ACTIVE"
		active = true
	}
	return &DNSStatus{
		Active: active,
		Status: dnsMode,
	}
}

func checkMacDNSSetup() *DNSStatus {
	dnsMode := "INACTIVE"
	dnsConfig, err := os.ReadFile("/etc/resolver/locals")
	hasFile := err == nil && len(dnsConfig) > 0

	hasAlias := isIPOnInterface("lo0", DefaultDNSListen)

	if hasFile && hasAlias {
		dnsMode = "RESOLVER REDIRECT ACTIVE"
	} else if hasFile && !hasAlias {
		dnsMode = "MISSING DNS IP ALIAS"
	} else if !hasFile && hasAlias {
		dnsMode = "MISSING DNS RESOLVER FILE"
	}
	return &DNSStatus{
		Active: hasFile && hasAlias,
		Status: dnsMode,
	}
}

func isIPOnInterface(ifaceName, targetIP string) bool {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		// addr is in CIDR format like "127.1.2.3/32"
		ip, _, _ := net.ParseCIDR(addr.String())
		if ip != nil && ip.String() == targetIP {
			return true
		}
	}
	return false
}
