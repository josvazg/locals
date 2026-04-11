package platform

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

type dryrunPlatform struct {
	Platform
}

func NewDryrunPlatform(p Platform) Platform {
	return &dryrunPlatform{p}
}

func (dp *dryrunPlatform) IO() FilesHandler {
	return &dryrunIO{FilesHandler: dp.Platform.IO()}
}

func (_ *dryrunPlatform) Proc() Proc {
	return &dryrunProc{}
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
	cli := strings.Join(append([]string{cmd}, args...), " ")
	log.Printf("DRYRUN: %s", cli)
	return "", nil
}

func dryrunLaunch(cmd string, args ...string) (int, error) {
	cli := strings.Join(append([]string{cmd}, args...), " ")
	log.Printf("DRYRUN %s", cli)
	return -1, nil
}

type dryrunIO struct {
	FilesHandler
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
