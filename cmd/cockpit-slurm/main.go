package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/dispatcher"
	"github.com/lingweicai/cockpit-slurm/internal/ipc"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache := resource.NewNodeCache()
	adapter := resource.NewSlurmNodeAdapter()
	synchronizer, err := resource.NewNodeSynchronizer(adapter, cache, 5*time.Second, nil)
	if err != nil {
		log.Fatalf("create Node synchronizer: %v", err)
	}
	if _, err := synchronizer.Refresh(ctx); err != nil {
		log.Fatalf("load initial Slurm node snapshot: %v", err)
	}
	go synchronizer.RunPeriodic(ctx, func(err error) {
		log.Printf("refresh Slurm node snapshot: %v", err)
	})

	server := ipc.NewServerWithDispatcher("", dispatcher.NewDispatcherWithCache(cache))

	if err := server.Listen(); err != nil {
		log.Fatalf("listen for IPC socket: %v", err)
	}

	if err := server.Serve(ctx); err != nil {
		log.Fatalf("serve IPC socket: %v", err)
	}

	log.Printf("cockpit-slurm bridge stopped")
}
