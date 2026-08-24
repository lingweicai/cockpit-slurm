package ipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

const DefaultSocketPath = "/run/cockpit-slurm/bridge.sock"

const socketFileMode = 0660

// Server provides a Unix-domain socket listener for cockpit-slurm.
type Server struct {
	socketPath string
	listener   net.Listener

	wg sync.WaitGroup

	mu      sync.Mutex
	closed  bool
	cancel  context.CancelFunc
	active  map[net.Conn]struct{}
}

// NewServer creates an IPC server.
func NewServer(socketPath string) *Server {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}

	return &Server{socketPath: socketPath}
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

	return nil
}

// Serve accepts connections until the server is closed.
func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()

	if listener == nil {
		return errors.New("IPC server is not listening")
	}

	serveCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.active = make(map[net.Conn]struct{})
	s.mu.Unlock()

	go func() {
		<-serveCtx.Done()
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

		s.trackConn(conn)
		s.wg.Add(1)

		go func(conn net.Conn) {
			defer s.wg.Done()
			defer s.untrackConn(conn)
			defer conn.Close()

			// Phase 1B intentionally does not process application messages.
			// Phase 1C will attach the protocol layer here.
			<-serveCtx.Done()
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

	if s.cancel != nil {
		s.cancel()
	}

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

	return os.Remove(path)
}
