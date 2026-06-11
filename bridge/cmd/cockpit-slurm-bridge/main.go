package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lingweicai/cockpit-slurm/bridge/internal/slurm"
	"github.com/lingweicai/cockpit-slurm/bridge/internal/socket"
)

const (
	defaultSocketPath   = "/run/cockpit-slurm/bridge.sock"
	defaultPollInterval = 10 * time.Second
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	service := slurm.NewSinfoService(defaultPollInterval)
	server := socket.NewServer(defaultSocketPath, service.EventChannel(), service.Cache())

	go func() {
		if err := service.Run(ctx); err != nil {
			log.Printf("sinfo service exited: %v", err)
			cancel()
		}
	}()

	go func() {
		if err := server.Start(ctx); err != nil {
			log.Printf("socket server exited: %v", err)
			cancel()
		}
	}()

	log.Println("cockpit-slurm-bridge started")
	<-ctx.Done()
	log.Println("shutting down cockpit-slurm-bridge")
}
