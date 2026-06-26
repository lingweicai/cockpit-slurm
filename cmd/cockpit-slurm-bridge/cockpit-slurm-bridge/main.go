package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/account"
	"github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/socket"
	"github.com/lingweicai/cockpit-slurm/cmd/internal/provider"
)

const (
	defaultSocketPath   = "/run/cockpit-slurm/bridge.sock"
	defaultPollInterval = 30 * time.Second
)

func bridgeSocketDefaults() string {
	if path := os.Getenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH"); path != "" {
		return path
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "cockpit-slurm", "bridge.sock")
	}
	if uid := os.Getuid(); uid > 0 {
		return filepath.Join("/run/user", strconv.Itoa(uid), "cockpit-slurm", "bridge.sock")
	}
	return defaultSocketPath
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	socketPath := bridgeSocketDefaults()
	flag.StringVar(&socketPath, "socket", socketPath, "path to bridge socket")
	flag.StringVar(&socketPath, "s", socketPath, "path to bridge socket (shorthand)")
	flag.Parse()

	accountManager := account.NewManager(provider.NewSacctmgrAccountProvider())
	count, err := accountManager.LoadInitialCache(ctx)
	if err != nil {
		log.Fatalf("load initial account cache: %v", err)
	}
	log.Printf("Loaded %d accounts into cache", count)
	log.Printf("Generation: %d", accountManager.Cache().Generation())

	server := socket.NewServer(socketPath, accountManager)

	go func() {
		if err := accountManager.Run(ctx, defaultPollInterval); err != nil {
			log.Printf("account poller exited: %v", err)
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
