package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lingweicai/cockpit-slurm/internal/dispatcher"
	"github.com/lingweicai/cockpit-slurm/internal/identity"
	"github.com/lingweicai/cockpit-slurm/internal/ipc"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
	"github.com/lingweicai/cockpit-slurm/internal/stream"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache := resource.NewNodeCache()
	adapter := resource.NewSlurmNodeAdapter()
	streams, err := stream.NewNodeStreamRegistry(cache, stream.DefaultNodeStreamQueueSize)
	if err != nil {
		log.Fatalf("configure Node streams: %v", err)
	}
	synchronizer, err := resource.NewNodeSynchronizer(adapter, cache, streams, resource.DefaultNodeSyncInterval)
	if err != nil {
		log.Fatalf("configure Node synchronization: %v", err)
	}
	if err := synchronizer.Refresh(ctx); err != nil {
		log.Fatalf("load initial Slurm node snapshot: %v", err)
	}

	resolver := identity.NewCapabilityResolver(identity.NewSacctmgrUserLookup())
	messageDispatcher := dispatcher.NewDispatcherWithCacheAndCapabilities(cache, resolver)
	server := ipc.NewServerWithNodeStreamsAndDispatcher("", streams, messageDispatcher)

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
