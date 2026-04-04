//go:build mage
// +build mage

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		"./internal/render/testdata/darwin/*.sh",
		"./internal/render/testdata/linux/*.sh",
	})...)
}

// Test runs all unit tests in the project.
func Test() error {
	mg.Deps(Build)

	fmt.Println("Running tests...")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working dir")
	}
	env := map[string]string{
		"LOCALSBIN": filepath.Join(cwd, "bin", "locals"),
	}
	err = runVEnv(env, "go", "test", "-v", "./...")
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
		"debian/14",
		"ubuntu/25.04",
		"fedora/43",
		"nixos/25.11",
	}
	var errs error
	for _, image := range images {
		imgRef := fmt.Sprintf("images:%s", image)
		if err := sh.RunV("./test/incus.sh", imgRef, "mage", "-v", "test"); err != nil {
			errs = errors.Join(errs, fmt.Errorf("image %s failed: %w", image, err))
		}
	}
	if errs == nil {
		fmt.Println("✅ All distro tests PASSED")
	}
	return errs
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
