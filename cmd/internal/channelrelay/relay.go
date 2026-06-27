package channelrelay

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const defaultBridgeSocketPath = "/run/cockpit-slurm/bridge.sock"

// DefaultSocketPath returns the preferred bridge socket path for the channel helper.
func DefaultSocketPath() string {
	if path := os.Getenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH"); path != "" {
		return path
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "cockpit-slurm", "bridge.sock")
	}
	if uid := os.Getuid(); uid > 0 {
		return filepath.Join("/run/user", strconv.Itoa(uid), "cockpit-slurm", "bridge.sock")
	}
	return defaultBridgeSocketPath
}

// Run connects stdin/stdout to the bridge unix socket.
func Run(stdin io.Reader, stdout io.Writer, socketPath string) error {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to bridge socket %q: %w", socketPath, err)
	}
	defer conn.Close()

	stdinErrCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, stdin)
		if unixConn, ok := conn.(*net.UnixConn); ok {
			_ = unixConn.CloseWrite()
		}
		stdinErrCh <- err
	}()

	if _, err := io.Copy(stdout, conn); err != nil {
		_ = conn.Close()
		stdinErr := <-stdinErrCh
		if stdinErr != nil && !errors.Is(stdinErr, io.EOF) && !errors.Is(stdinErr, net.ErrClosed) {
			return fmt.Errorf("copy stdin to socket: %w", stdinErr)
		}
		return fmt.Errorf("copy socket to stdout: %w", err)
	}

	_ = conn.Close()
	if stdinErr := <-stdinErrCh; stdinErr != nil && !errors.Is(stdinErr, io.EOF) && !errors.Is(stdinErr, net.ErrClosed) {
		return fmt.Errorf("copy stdin to socket: %w", stdinErr)
	}
	return nil
}
