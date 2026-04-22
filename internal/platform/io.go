package platform

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type FilesHandler interface {
	ReadFile(filename string) ([]byte, error)
	AppendTo(filename string) (io.WriteCloser, error)
	ReadDir(dirname string) ([]fs.DirEntry, error)
	CreateFile(filename, content string) error
	CreateDir(filename string) error
	RemoveFiles(filenames ...string) error
	ListFiles(globs ...string) ([]string, error)
	PathExists(path string) bool
	TempDir() string
}

type osFilesHandler struct{}

func newOSFilesHandler() *osFilesHandler {
	return &osFilesHandler{}
}

func (osf *osFilesHandler) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (osf *osFilesHandler) AppendTo(filename string) (io.WriteCloser, error) {
	return os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
} 

func (osf *osFilesHandler) CreateFile(filename, content string) error {
	dir := filepath.Dir(filename)
	if !osf.PathExists(dir) {
		log.Printf("Creating missing directory: %s", dir)
		if err := osf.CreateDir(dir); err != nil {
			return fmt.Errorf("failed to create missing dir: %w", err)
		}
	}
	return os.WriteFile(filename, ([]byte)(content), 0600)
}

func (osf *osFilesHandler) WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

func (osf *osFilesHandler) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return os.ReadDir(dirname)
}

func (osf *osFilesHandler) CreateDir(dir string) error {
	return osf.MkdirAll(dir, 0750)
}

func (osf *osFilesHandler) MkdirAll(dirname string, perm fs.FileMode) error {
	return os.MkdirAll(dirname, perm)
}

func (osf *osFilesHandler) PathExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func (osf *osFilesHandler) Stat(filename string) (os.FileInfo, error) {
	return os.Stat(filename)
}

func (osf *osFilesHandler) RemoveFiles(filenames ...string) error {
	for _, filename := range filenames {
		if err := osf.removeFile(filename); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil // if it was already gone, we are done, we are good
			}
			return fmt.Errorf("failed to remove file %s: %w", filename, err)
		}
	}
	return nil
}

func (osf *osFilesHandler) removeFile(filename string) error {
	if strings.HasSuffix(strings.TrimSpace(filename), "/") {
		return fmt.Errorf("refusing unsafe removal of possible dir %q", filename)
	}
	return os.Remove(filename)
}

func (osf *osFilesHandler) ListFiles(globs ...string) ([]string, error) {
	paths := []string{}
	for _, glob := range globs {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, fmt.Errorf("failed to list files from glob expression %q: %w", globs, err)
		}
		for _, match := range matches {
			f, _ := os.Stat(match)
			if !f.IsDir() {
				paths = append(paths, match)
			}
		}
	}
	return paths, nil
}

func (osf *osFilesHandler) Remove(filename string) error {
	return os.Remove(filename)
}

func (_ *osFilesHandler) TempDir() string {
	return os.TempDir()
}

func Find(io FilesHandler, filename, pattern string) (bool, error) {
	contents, err := io.ReadFile(filename)
	if err != nil {
		return false, fmt.Errorf("failed to read file %q: %v", filename, contents)
	}
	return strings.Contains(string(contents), pattern), nil
}

func DNSConfigFile(localsDir string) string {
	return filepath.Join(localsDir, "resolv.patched.conf")
}
