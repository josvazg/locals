package sim

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"locals/internal/platform"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Constants for operation names to avoid magic strings
const (
	OpCommandMarker = "Command"

	OpReadFile    = "ReadFile"
	OpCreateFile  = "CreateFile"
	OpListFiles   = "ListFiles"
	OpReadDir     = "ReadDir"
	OpCreateDir   = "CreateDir"
	OpRemoveFiles = "RemoveFiles"
	OpPathExists  = "PathExists"
	OpTempDir     = "TempDir"
	OpRun         = "Run"
	OpLaunch      = "Launch"

	OpProcRun            = "ProcRun"
	OpProcLaunch         = "ProcLaunch"
	OpProcIsProcessAlive = "ProcIsProcessAlive"
)

type recorderPlatform struct {
	distro       string
	version      string
	lastSequence int
	platform.Platform
}

type recordedEvent struct {
	Operation string `json:"op"`
	Inputs    []any  `json:"in"`
	Outputs   []any  `json:"out"`
}

func (rec *recorderPlatform) Clear() error {
	dir := filepath.Join("..", "testdata", "recorded", rec.distro, rec.version)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("recorder: mkdir failed: %w", err)
	}

	file := filepath.Join(dir, "session.jsonl")
	return os.Truncate(file, 0)
}

// Record handles the serialization and persistence.
// It returns an error if the recording itself fails.
func (rec *recorderPlatform) Record(op string, in []any, out []any) error {
	// 1. Sanitize Outputs: Convert error types to strings for JSON
	for i, v := range out {
		if err, ok := v.(error); ok {
			if err == nil {
				out[i] = nil
			} else {
				out[i] = err.Error()
			}
		}
	}

	event := recordedEvent{
		Operation: op,
		Inputs:    in,
		Outputs:   out,
	}

	dir := filepath.Join("..", "testdata", "recorded", rec.distro, rec.version)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("recorder: mkdir failed: %w", err)
	}

	file := filepath.Join(dir, "session.jsonl")
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("recorder: marshal failed: %w", err)
	}

	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("recorder: open file failed: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(eventJSON, '\n')); err != nil {
		return fmt.Errorf("recorder: write failed: %w", err)
	}
	return nil
}

func NewRecorderPlatform(p platform.Platform) *recorderPlatform {
	return &recorderPlatform{
		Platform: p,
		distro:   GetDistro(p),
		version:  GetVersion(p),
	}
}

func (rec *recorderPlatform) IO() platform.FilesHandler {
	return &recorderIO{rp: rec, FilesHandler: rec.Platform.IO()}
}

func (rec *recorderPlatform) Proc() platform.Proc {
	return &recorderProc{rp: rec, Proc: rec.Platform.Proc()}
}

type recorderIO struct {
	rp *recorderPlatform
	platform.FilesHandler
}

func (rci *recorderIO) ReadFile(filename string) ([]byte, error) {
	data, err := rci.FilesHandler.ReadFile(filename)
	if recErr := rci.rp.Record(
		OpReadFile, []any{filename}, []any{string(data), err},
	); recErr != nil {
		return nil, recErr
	}
	return data, err
}

func (rci *recorderIO) CreateFile(filename, content string) error {
	err := rci.FilesHandler.CreateFile(filename, content)
	if recErr := rci.rp.Record(
		OpCreateFile, []any{filename, content}, []any{err},
	); recErr != nil {
		return recErr
	}
	return err
}

func (rci *recorderIO) ReadDir(dirname string) ([]fs.DirEntry, error) {
	entries, err := rci.FilesHandler.ReadDir(dirname)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	if recErr := rci.rp.Record(
		OpReadDir, []any{dirname}, []any{names, err},
	); recErr != nil {
		return nil, recErr
	}
	return entries, err
}

func (rci *recorderIO) ListFiles(globs ...string) ([]string, error) {
	files, err := rci.FilesHandler.ListFiles(globs...)
	if recErr := rci.rp.Record(
		OpListFiles, []any{globs}, []any{files, err},
	); recErr != nil {
		return nil, recErr
	}
	return files, err
}

func (rci *recorderIO) CreateDir(dirname string) error {
	err := rci.FilesHandler.CreateDir(dirname)
	if recErr := rci.rp.Record(
		OpCreateDir, []any{dirname}, []any{err},
	); recErr != nil {
		return recErr
	}
	return err
}

func (rci *recorderIO) RemoveFiles(filenames ...string) error {
	err := rci.FilesHandler.RemoveFiles(filenames...)
	if recErr := rci.rp.Record(
		OpRemoveFiles, []any{filenames}, []any{err},
	); recErr != nil {
		return recErr
	}
	return err
}

func (rci *recorderIO) PathExists(path string) bool {
	exists := rci.FilesHandler.PathExists(path)
	// PathExists doesn't return an error in the interface,
	// so we log recording failures to Stderr as a last resort.
	if recErr := rci.rp.Record(
		OpPathExists, []any{path}, []any{exists},
	); recErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", recErr)
	}
	return exists
}

func (rci *recorderIO) TempDir() string {
	dir := rci.FilesHandler.TempDir()
	if recErr := rci.rp.Record(
		OpTempDir, nil, []any{dir},
	); recErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", recErr)
	}
	return dir
}

type recorderProc struct {
	rp *recorderPlatform
	platform.Proc
}

func (rcp *recorderProc) Run(cmd string, args ...string) (string, error) {
	stdout, err := rcp.Proc.Run(cmd, args...)

	if recErr := rcp.rp.Record(
		OpProcRun, []any{cmd, args}, []any{stdout, err},
	); recErr != nil {
		return "", recErr
	}

	return stdout, err
}

func (rcp *recorderProc) Launch(cmd string, args ...string) (int, error) {
	pid, err := rcp.Proc.Launch(cmd, args...)

	if recErr := rcp.rp.Record(
		OpProcLaunch, []any{cmd, args}, []any{pid, err},
	); recErr != nil {
		return 0, recErr
	}

	return pid, err
}

func (rcp *recorderProc) IsProcessAlive(pid int) bool {
	alive := rcp.Proc.IsProcessAlive(pid)

	if recErr := rcp.rp.Record(
		OpProcIsProcessAlive, []any{pid}, []any{alive},
	); recErr != nil {
		panic(recErr)
	}

	return alive
}

// GetDistro returns the name of the OS distro
func GetDistro(p platform.Platform) string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "linux":
		data, err := p.IO().ReadFile("/etc/os-release")
		if err == nil {
			id := parseOSRelease(data, "ID")
			if id != "" {
				return strings.ToLower(id)
			}
		}
		return "linux-generic"
	default:
		return runtime.GOOS
	}
}

// GetVersion returns a "snapshot-stable" version string for the current OS.
func GetVersion(p platform.Platform) string {
	osName := runtime.GOOS

	switch osName {
	case "darwin":
		v, err := p.Proc().Run("sw_vers", "-productVersion")
		if err != nil {
			return "unknown"
		}
		return strings.TrimSpace(v)

	case "linux":
		data, err := p.IO().ReadFile("/etc/os-release")
		if err != nil {
			return "linux-generic"
		}
		versionID := parseOSRelease(data, "VERSION_ID")
		if versionID == "" {
			kv, _ := p.Proc().Run("uname", "-r")
			return strings.TrimSpace(kv)
		}
		return versionID

	default:
		return "unsupported"
	}
}

// Helpers to keep the logic clean

func parseOSRelease(data []byte, key string) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key+"=") {
			val := strings.TrimPrefix(line, key+"=")
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}
