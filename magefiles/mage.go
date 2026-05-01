//go:build mage
// +build mage

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Fmt check formats
func Fmt() error {
	return sh.RunV("go", "fmt")
}

// Build compiles the project binary into the bin folder.
func Build() error {
	mg.Deps(Fmt)
	fmt.Println("Building binary...")
	return sh.RunV("go", "build", "-o", "bin/locals", "./main.go")
}

// Shellcheck runs the shell check
func Shellcheck() error {
	return sh.RunV("shellcheck", expandFiles([]string{
		"./test/*.sh",
	})...)
}

// Test runs all unit tests in the project.
func Test() error {
	mg.Deps(Build, Shellcheck)

	fmt.Println("Running tests...")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working dir")
	}
	env := map[string]string{
		"LOCALSBIN": filepath.Join(cwd, "bin", "locals"),
	}
	err = runVEnv(env, "go", "test", "-v", "-timeout", "2m", "./...")
	if err == nil {
		fmt.Println("✅ Tests PASSED")
	}
	return err
}

// TestLinuxDistros runs all incus tests to try out all supported Linux distros
func TestLinuxDistros() error {
	mg.Deps(Build)

	fmt.Println("Running distro tests...")
	images := []string{
		"archlinux",
		"voidlinux",
		"debian/14",
		"ubuntu/noble",
		"fedora/43",
		// "nixos/25.11", // nixos image lacks sudo and seems broken inside
	}
	var errs error
	for _, image := range images {
		imgRef := fmt.Sprintf("images:%s", image)
		if err := TestInIncus(imgRef, "mage -v test"); err != nil {
			errs = errors.Join(errs, fmt.Errorf("image %s failed: %w", image, err))
		}
	}
	if errs == nil {
		fmt.Println("✅ All distro tests PASSED")
	}
	return errs
}

// TestInIncus runs the system container tests in incus
// Tricks used:
//  1. Mount container in tmpfs, to avoid touching the dick on ephemeral tests
//  2. Share the host nix dir on host-nix, distro bind mounts it in nix later
//  3. Pass the nix path from the host as an env var that distro.sh can use
//  4. Mount the host go modcache to avoid rebuilds or downloads
func TestInIncus(image string, cli string) error {
	nodeName := fmt.Sprintf("test-%d", time.Now().Unix())

	// Setup RAM storage
	if err := setupRamPool(); err != nil {
		return fmt.Errorf("failed to setup ram pool: %w", err)
	}

	fmt.Printf("--- Launching %s ---\n", image)
	launchArgs := []string{
		"launch", image, nodeName,
		"--storage", "ram-pool",
		"-c", "security.privileged=true",
		"-c", "security.nesting=true",
		"--ephemeral",
	}
	if err := sh.RunV("incus", launchArgs...); err != nil {
		return err
	}

	// Cleanup on exit
	defer sh.Run("incus", "delete", "-f", nodeName)

	// Wait for systemd
	fmt.Println("Waiting for container...")
	waitForContainer(nodeName)

	// Add Devices
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current dir: %w", err)
	}
	sh.RunV("incus", "config", "device", "add", nodeName, "project-src", "disk", "source="+cwd, "path=/src")
	sh.RunV("incus", "config", "device", "add", nodeName, "host-nix", "disk", "source=/nix", "path=/host-nix", "readonly=true")

	hostNixBin, _ := exec.LookPath("nix")
	hostNixBin, _ = filepath.EvalSymlinks(hostNixBin)
	hostNixPath := filepath.Dir(hostNixBin)

	// wire go mod cache to avoid re-downloads in the container
	modCache, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		return fmt.Errorf("failed to get go modcache location: %w", err)
	}
	cleanModCache := strings.TrimSpace(string(modCache))
	sh.RunV("incus", "config", "device", "add", nodeName, "go-mod-cache", "disk",
		"source="+cleanModCache, "path=/host-go-cache", "readonly=true")

	runArgs := []string{
		"exec", nodeName, "--",
		"env", fmt.Sprintf("HOST_NIX_PATH=%s", hostNixPath),
		"GOMODCACHE=/host-go-cache", // Point to the mount
		"GOPROXY=off",               // Disable network lookups
		"bash", "-c", fmt.Sprintf("/src/test/distro.sh %v", cli),
	}
	return sh.RunV("incus", runArgs...)
}

func setupRamPool() error {
	ramDir := filepath.Join(os.TempDir(), "incus-tmp")
	sh.Run("sudo", "mkdir", "-p", ramDir)

	// Check mountpoint
	if err := sh.Run("mountpoint", "-q", ramDir); err != nil {
		sh.Run("sudo", "mount", "-t", "tmpfs", "-o", "size=6G", "tmpfs", ramDir)
	}

	// Check storage pool
	if err := sh.Run("incus", "storage", "show", "ram-pool"); err != nil {
		return sh.Run("incus", "storage", "create", "ram-pool", "dir", "source="+ramDir)
	}
	return nil
}

func waitForContainer(node string) {
	fmt.Print("Waiting for container boot...")
	for i := 0; i < 20; i++ {
		err := sh.Run("incus", "exec", node, "--", "ls")
		if err == nil {
			fmt.Printf(" Done\n")
			return
		}
		fmt.Print(".")
		time.Sleep(1 * time.Second)
	}
	fmt.Println("Timeout")
}

// Clean removes the build artifacts.
func Clean() error {
	fmt.Println("Cleaning up...")
	return sh.Rm("bin")
}

func runVEnv(env map[string]string, cmd string, args ...string) error {
	var stdout, stderr io.Writer
	if mg.Verbose() {
		stdout = os.Stdout
		stderr = os.Stderr
		fmt.Printf("exec: %s %s", cmd, strings.Join(args, " "))
	}
	_, err := sh.Exec(env, stdout, stderr, cmd, args...)
	return err
}

func expandFiles(globs []string) []string {
	paths := []string{}
	for _, glob := range globs {
		matches, _ := filepath.Glob(glob)
		for _, match := range matches {
			f, _ := os.Stat(match)
			if !f.IsDir() {
				paths = append(paths, match)
			}
		}
	}
	return paths
}
