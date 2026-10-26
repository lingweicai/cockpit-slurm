package channel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

var ErrClosed = errors.New("channel closed")

// Channel is the transport-level adapter between Cockpit and the bridge.
// It deliberately does not contain Slurm business logic, resource models,
// or authorization decisions.
type Channel struct {
	mu     sync.Mutex
	closed bool
	conn   net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

func New() *Channel {
	ctx, cancel := context.WithCancel(context.Background())
	return &Channel{ctx: ctx, cancel: cancel}
}

func (c *Channel) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *Channel) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.conn
	cancel := c.cancel
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
	return nil
}

func (c *Channel) SetConn(conn net.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
}

func (c *Channel) Run(cockpit io.ReadWriteCloser, bridge io.ReadWriteCloser) error {
	if cockpit == nil || bridge == nil {
		return fmt.Errorf("%w: nil transport", ErrClosed)
	}
	if conn, ok := bridge.(net.Conn); ok {
		c.SetConn(conn)
	}
	defer func() { _ = cockpit.Close(); _ = bridge.Close(); _ = c.Close() }()

	errCh := make(chan error, 2)
	go func() {
		errCh <- c.copy(bridge, cockpit)
	}()
	go func() {
		errCh <- c.copy(cockpit, bridge)
	}()

	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, ErrClosed) && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.ErrClosedPipe) {
			_ = c.Close()
			_ = cockpit.Close()
			_ = bridge.Close()
			return err
		}
		if i == 0 {
			_ = c.Close()
			_ = cockpit.Close()
			_ = bridge.Close()
		}
	}
	return nil
}

func (c *Channel) Copy(dst io.Writer, src io.Reader) error {
	return c.copy(dst, src)
}

func (c *Channel) copy(dst io.Writer, src io.Reader) error {
	if c.isClosed() {
		return ErrClosed
	}

	buf := make([]byte, 32*1024)
	for {
		if c.isClosed() {
			return ErrClosed
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				if errors.Is(werr, net.ErrClosed) || errors.Is(werr, ErrClosed) || errors.Is(werr, io.ErrClosedPipe) {
					return ErrClosed
				}
				return fmt.Errorf("write forwarded data: %w", werr)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, net.ErrClosed) || errors.Is(err, ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
				return ErrClosed
			}
			return err
		}
	}
}
