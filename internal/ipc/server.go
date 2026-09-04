package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/dispatcher"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

const DefaultSocketPath = "/run/cockpit-slurm/bridge.sock"

const socketFileMode = 0660

var ErrSocketInUse = errors.New("unix socket is already in use")

// Server provides a Unix-domain socket listener for cockpit-slurm.
type Server struct {
	socketPath string
	listener   net.Listener
	dispatcher *dispatcher.MessageDispatcher
	connections connection.ConnectionManager

	wg sync.WaitGroup

	mu     sync.Mutex
	closed bool
	active map[net.Conn]struct{}
}

// NewServer creates an IPC server.
func NewServer(socketPath string) *Server {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}

	return &Server{
		socketPath: socketPath,
		dispatcher: dispatcher.NewDispatcher(),
		connections: connection.NewConnectionManager(),
	}
}

// SocketPath returns the Unix socket path.
func (s *Server) SocketPath() string {
	return s.socketPath
}

// Listen creates and binds the Unix-domain socket.
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return errors.New("IPC server is already listening")
	}

	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	if err := removeStaleSocket(s.socketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %q: %w", s.socketPath, err)
	}

	if err := os.Chmod(s.socketPath, socketFileMode); err != nil {
		_ = listener.Close()
		_ = os.Remove(s.socketPath)
		return fmt.Errorf("set socket permissions: %w", err)
	}

	s.listener = listener
	s.closed = false
	s.active = make(map[net.Conn]struct{})

	return nil
}

// Serve accepts connections until the server is closed.
func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	listener := s.listener
	if listener == nil {
		s.mu.Unlock()
		return errors.New("IPC server is not listening")
	}
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if s.isClosed() {
				return nil
			}

			return fmt.Errorf("accept connection: %w", err)
		}

		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return nil
		}
		s.active[conn] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()

		go func(conn net.Conn) {
			defer s.wg.Done()
			defer s.untrackConn(conn)
			defer conn.Close()

			connState, err := connection.NewConnection(conn)
			if err != nil {
				_ = conn.Close()
				return
			}
			if err := s.connections.Register(connState); err != nil {
				_ = conn.Close()
				return
			}
			defer s.connections.Unregister(connState.ID)
			defer connState.Close()

			dec := protocol.NewDecoder(conn)
			enc := protocol.NewEncoder(conn)

			for {
				if connState.Context.Err() != nil {
					return
				}

				msg, err := dec.Decode()
				if err != nil {
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
						return
					}
					payload, jsonErr := json.Marshal(map[string]string{
						"code":    "INVALID_MESSAGE",
						"message": err.Error(),
					})
					if jsonErr != nil {
						return
					}
					resp := protocol.NewEnvelope("ERR", protocol.MessageError, payload)
					if writeErr := enc.Encode(resp); writeErr != nil {
						return
					}
					return
				}

				resp := s.dispatcher.Dispatch(connState.Context, msg)
				if err := enc.Encode(resp); err != nil {
					return
				}
			}
		}(conn)
	}
}

// Close stops the server and removes the socket.
func (s *Server) Close() error {
	s.mu.Lock()

	if s.closed {
		s.mu.Unlock()
		return nil
	}

	s.closed = true

	listener := s.listener
	s.listener = nil

	active := make([]net.Conn, 0, len(s.active))
	for conn := range s.active {
		active = append(active, conn)
	}

	s.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}

	for _, conn := range active {
		_ = conn.Close()
	}

	s.connections.CloseAll()

	s.wg.Wait()

	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove socket: %w", err)
	}

	return nil
}

func (s *Server) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

func (s *Server) trackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = conn.Close()
		return
	}
	if s.active == nil {
		s.active = make(map[net.Conn]struct{})
	}
	s.active[conn] = struct{}{}
}

func (s *Server) untrackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		delete(s.active, conn)
	}
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("socket path %q exists and is not a Unix socket", path)
	}

	conn, err := net.DialTimeout("unix", path, 250*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("%w: %s", ErrSocketInUse, path)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Timeout() {
		return fmt.Errorf("cannot determine whether unix socket %q is active: %w", path, err)
	}

	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("cannot determine whether unix socket %q is active: %w", path, err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale socket %q: %w", path, err)
	}

	return nil
}
