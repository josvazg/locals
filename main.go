package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"locals/api/locals"
	"locals/internal/cmds"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmds.Run(ctx, locals.RealOSPlatform(), os.Args[1:]); err != nil {
		log.Fatalf("locals failed: %v", err)
	}
}
