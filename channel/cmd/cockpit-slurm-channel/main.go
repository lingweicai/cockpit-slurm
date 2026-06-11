package main

import (
	"io"
	"log"
	"net"
	"os"
)

const bridgeSocketPath = "/run/cockpit-slurm/bridge.sock"

func main() {
	conn, err := net.Dial("unix", bridgeSocketPath)
	if err != nil {
		log.Fatalf("connect to bridge socket %q: %v", bridgeSocketPath, err)
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
