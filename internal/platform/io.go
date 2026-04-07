package platform

import (
	"io/fs"
	"os"
)

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
