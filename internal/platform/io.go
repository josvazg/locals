package platform

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type FilesHandler interface {
	ReadFile(filename string) ([]byte, error)
	ReadDir(dirname string) ([]fs.DirEntry, error)
	CreateFile(dryrun bool, filename, content string) error
	CreateDir(dryrun bool, filename string) error
	RemoveFiles(dryrun bool, filenames ... string) error
	ListFiles(globs ... string) ([]string, error)
}

type osFilesHandler struct{}

func OSFileHandler() *osFilesHandler {
	return &osFilesHandler{}
}

func (osf *osFilesHandler) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (osf *osFilesHandler) CreateFile(dryrun bool, filename, content string) error {
	dir := filepath.Dir(filename)
	if !osf.PathExists(dir) {
		log.Printf("Creating missing directory: %s", dir)
		if err := osf.CreateDir(dryrun, dir); err != nil {
			return fmt.Errorf("failed to create missing dir: %w", err)
		}
	}
	if dryrun {
		log.Printf("DRYRUN: would have created file %q as:\n------\n%s\n------", filename, content)
		return nil
	}
	return os.WriteFile(filename, ([]byte)(content), 0600)
}

func (osf *osFilesHandler) WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

func (osf *osFilesHandler) ReadDir(dirname string) ([]fs.DirEntry, error) {
	return os.ReadDir(dirname)
}

func (osf *osFilesHandler) CreateDir(dryrun bool, dir string) error {
	if dryrun {
		log.Printf("DRYRUN: would have created dir %q", dir)
		return nil
	}
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

func (osf *osFilesHandler) RemoveFiles(dryrun bool, filenames ...string) error {
	for _, filename := range filenames {
		if err := osf.removeFile(dryrun, filename); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil // if it was already gone, we are done, we are good
			}
			return fmt.Errorf("failed to remove file %s: %w", filename, err)
		}
	}
	return nil
}

func (osf *osFilesHandler) removeFile(dryrun bool, filename string) error {
	if strings.HasSuffix(strings.TrimSpace(filename), "/") {
		return fmt.Errorf("refusing unsafe removal of possible dir %q", filename)
	}
	if dryrun {
		log.Printf("DRYRUN: would have removed file %s", filename)
		return nil	
	}
	return os.Remove(filename)
}

func (osf *osFilesHandler) ListFiles(globs ... string) ([]string, error) {
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
