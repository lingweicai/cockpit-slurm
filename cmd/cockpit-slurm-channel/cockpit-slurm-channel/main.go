package main

import (
	"flag"
	"log"
	"os"

	"github.com/lingweicai/cockpit-slurm/cmd/internal/channelrelay"
)

func main() {
	socketPath := channelrelay.DefaultSocketPath()
	flag.StringVar(&socketPath, "socket", socketPath, "path to bridge socket")
	flag.StringVar(&socketPath, "s", socketPath, "path to bridge socket (shorthand)")
	flag.Parse()

	if err := channelrelay.Run(os.Stdin, os.Stdout, socketPath); err != nil {
		log.Fatalf("cockpit-slurm-channel: %v", err)
	}
}
