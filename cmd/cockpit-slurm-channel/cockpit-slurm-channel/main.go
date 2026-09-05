package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/lingweicai/cockpit-slurm/internal/channel"
	"github.com/lingweicai/cockpit-slurm/internal/ipc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 {
		log.Printf("ignoring extra arguments: %v", os.Args[1:])
	}

	socketPath, err := resolveSocketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid bridge socket path: %v\n", err)
		os.Exit(1)
	}

	bridge, err := channel.Connect(socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to bridge: %v\n", err)
		os.Exit(1)
	}
	defer bridge.Close()

	if err := relay(ctx, os.Stdin, os.Stdout, bridge); err != nil {
		fmt.Fprintf(os.Stderr, "channel relay failed: %v\n", err)
		os.Exit(1)
	}
}

func resolveSocketPath() (string, error) {
	socketPath := strings.TrimSpace(os.Getenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH"))
	if socketPath == "" {
		return ipc.DefaultSocketPath, nil
	}
	if !filepath.IsAbs(socketPath) {
		return "", fmt.Errorf("bridge socket path must be absolute: %q", socketPath)
	}
	return socketPath, nil
}

func relay(ctx context.Context, cockpitIn io.ReadCloser, cockpitOut io.WriteCloser, bridge io.ReadWriteCloser) error {
	ch := channel.New()

	// Keep the relay simple and transport-only. This is intentionally not a
	// business-layer implementation and does not interpret payload contents.
	go func() {
		<-ctx.Done()
		_ = ch.Close()
		_ = bridge.Close()
		_ = cockpitIn.Close()
		_ = cockpitOut.Close()
	}()

	errCh := make(chan error, 2)
	go func() { errCh <- ch.Copy(cockpitOut, bridge) }()
	go func() { errCh <- ch.Copy(bridge, cockpitIn) }()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && err != io.EOF && err != channel.ErrClosed {
			return err
		}
	}
	return nil
}
