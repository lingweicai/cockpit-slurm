package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
)

const defaultBridgeSocketPath = "/run/cockpit-slurm/bridge.sock"

// bridgeSocketDefaults returns the bridge socket path for the helper.
//
// For local development, set COCKPIT_SLURM_BRIDGE_SOCKET_PATH to the same path
// used by the bridge service, for example:
//
//	export COCKPIT_SLURM_BRIDGE_SOCKET_PATH="$XDG_RUNTIME_DIR/cockpit-slurm/bridge.sock"
//
// This makes the helper connect to the same development socket path as the bridge.
func bridgeSocketDefaults() string {
	if path := os.Getenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH"); path != "" {
		return path
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "cockpit-slurm", "bridge.sock")
	}
	return defaultBridgeSocketPath
}

func main() {
	socketPath := bridgeSocketDefaults()
	flag.StringVar(&socketPath, "socket", socketPath, "path to bridge socket")
	flag.StringVar(&socketPath, "s", socketPath, "path to bridge socket (shorthand)")
	flag.Parse()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatalf("connect to bridge socket %q: %v", socketPath, err)
	}
	defer conn.Close()

	go func() {
		if _, err := io.Copy(conn, os.Stdin); err != nil {
			log.Fatalf("copy stdin to socket: %v", err)
		}
	}()

	if _, err := io.Copy(os.Stdout, conn); err != nil {
		log.Fatalf("copy socket to stdout: %v", err)
	}
}
