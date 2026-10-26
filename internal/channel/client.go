package channel

import (
	"fmt"
	"net"
)

func Connect(socketPath string) (net.Conn, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial unix socket %q: %w", socketPath, err)
	}
	return conn, nil
}
