package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lingweicai/cockpit-slurm/internal/ipc"
)

func main() {
	server := ipc.NewServer(ipc.DefaultSocketPath)

	if err := server.Listen(); err != nil {
		log.Fatalf("listen for IPC socket: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Serve(ctx); err != nil {
		log.Fatalf("serve IPC socket: %v", err)
	}

	log.Printf("cockpit-slurm bridge stopped")
}
