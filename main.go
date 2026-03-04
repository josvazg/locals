package main

import (
	"log"
	"os"

	"locals/api/locals"
	"locals/internal/cmds"
)

func main() {
	if err := cmds.Run(locals.RealOSPlatform(), os.Args[1:]); err != nil {
		log.Printf("locals failed: %v", err)
	}
}
