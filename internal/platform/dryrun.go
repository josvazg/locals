package platform

import (
	"fmt"
	"log"
	"path/filepath"
)

type dryrunPlatform struct {
	Platform
}

func NewDryrunPlatform(p Platform) Platform {
	return &dryrunPlatform{p}
}

func (dp *dryrunPlatform) FS() Filesystem {
	return &dryrunIO{Filesystem: dp.Platform.FS()}
}

func (dp *dryrunPlatform) Proc() Proc {
	return &dryrunProc{Proc: dp.Platform.Proc()}
}

type dryrunProc struct {
	Proc
}

func (_ *dryrunProc) Run(cmd string, args ...string) (string, error) {
	return dryRun(cmd, args...)
}

func (_ *dryrunProc) Launch(cmd string, args ...string) (int, error) {
	return dryrunLaunch(cmd, args...)
}

func dryRun(cmd string, args ...string) (string, error) {
	log.Printf("DRYRUN: %s", fullCmd(cmd, args...))
	return "", nil
}

func dryrunLaunch(cmd string, args ...string) (int, error) {
	log.Printf("DRYRUN LAUNCH: %s", fullCmd(cmd, args...))
	return -1, nil
}

type dryrunIO struct {
	Filesystem
}

func (dio *dryrunIO) CreateFile(filename, content string) error {
	dir := filepath.Dir(filename)
	if !dio.PathExists(dir) {
		log.Printf("Creating missing directory: %s", dir)
		if err := dio.CreateDir(dir); err != nil {
			return fmt.Errorf("failed to create missing dir: %w", err)
		}
	}
	log.Printf("DRYRUN: would have created file %q as:\n------\n%s\n------", filename, content)
	return nil
}

func (dio *dryrunIO) CreateDir(dir string) error {
	log.Printf("DRYRUN: would have created dir %q", dir)
	return nil
}

func (dio *dryrunIO) RemoveFiles(filenames ...string) error {
	for _, filename := range filenames {
		log.Printf("DRYRUN: would have removed file %q", filename)
	}
	return nil
}
