// +build mage	

package main

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

// Build compiles the project binary into the bin folder.
func Build() error {
	fmt.Println("Building binary...")
	return sh.RunV("go", "build", "-o", "bin/locals", "./main.go")
}

// Test runs all unit tests in the project.
func Test() error {
	fmt.Println("Running tests...")
	return sh.RunV("go", "test", "-v", "./...")
}

// Clean removes the build artifacts.
func Clean() error {
	fmt.Println("Cleaning up...")
	return sh.Rm("bin")
}
