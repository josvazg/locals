package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"locals/internal/cmds"
	"locals/internal/platform"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmds.Run(ctx, platform.NewOSPlatform(), os.Args); err != nil {
		log.Fatalf("locals failed: %v", err)
	}
}
