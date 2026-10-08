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
	synchronizer, err := resource.NewNodeSynchronizer(adapter, cache, nil, resource.DefaultNodeSyncInterval)
	if err != nil {
		log.Fatalf("configure Node synchronization: %v", err)
	}
	if err := synchronizer.Refresh(ctx); err != nil {
		log.Fatalf("load initial Slurm node snapshot: %v", err)
	}

	server := ipc.NewServerWithDispatcher("", dispatcher.NewDispatcherWithCache(cache))

	if err := server.Listen(); err != nil {
		log.Fatalf("listen for IPC socket: %v", err)
	}

	go func() {
		if err := synchronizer.Run(ctx); err != nil {
			log.Printf("Node synchronizer stopped unexpectedly: %v", err)
			stop()
		}
	}()

	if err := server.Serve(ctx); err != nil {
		log.Fatalf("serve IPC socket: %v", err)
	}

	log.Printf("cockpit-slurm bridge stopped")
}
