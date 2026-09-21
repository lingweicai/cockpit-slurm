package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lingweicai/cockpit-slurm/internal/dispatcher"
	"github.com/lingweicai/cockpit-slurm/internal/ipc"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache := resource.NewNodeCache()
	adapter := resource.NewSlurmNodeAdapter()
	nodes, err := adapter.ListNodes(ctx)
	if err != nil {
		log.Fatalf("load initial Slurm node snapshot: %v", err)
	}
	cache.ReplaceSnapshot(nodes)

	server := ipc.NewServerWithDispatcher("", dispatcher.NewDispatcherWithCache(cache))

	if err := server.Listen(); err != nil {
		log.Fatalf("listen for IPC socket: %v", err)
	}

	if err := server.Serve(ctx); err != nil {
		log.Fatalf("serve IPC socket: %v", err)
	}

	log.Printf("cockpit-slurm bridge stopped")
}
