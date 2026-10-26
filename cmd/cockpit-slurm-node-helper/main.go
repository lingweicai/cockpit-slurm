package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lingweicai/cockpit-slurm/internal/nodehelper"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := nodehelper.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
