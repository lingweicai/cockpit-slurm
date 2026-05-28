package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lingweicai/cockpit-slurm/bridge/internal/models"
	"github.com/lingweicai/cockpit-slurm/bridge/internal/slurm"
)

type Server struct {
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	clients    map[int]*client
	nextID     int
	eventCh    <-chan slurm.Event
	cache      *models.SinfoCache
}

type client struct {
	conn    net.Conn
	writeMu sync.Mutex
	id      int
	done    chan struct{}
}

type request struct {
	Action    string `json:"action"`
	Subscribe bool   `json:"subscribe,omitempty"`
}

type response struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

func NewServer(socketPath string, eventCh <-chan slurm.Event, cache *models.SinfoCache) *Server {
	return &Server{
		socketPath: socketPath,
		clients:    make(map[int]*client),
		eventCh:    eventCh,
		cache:      cache,
	}
}

func (s *Server) Start(ctx context.Context) error {
	if err := os.RemoveAll(s.socketPath); err != nil {
		return fmt.Errorf("remove old socket: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen unix socket: %w", err)
	}
	if err := os.Chmod(s.socketPath, 0666); err != nil {
		listener.Close()
		return fmt.Errorf("set socket permissions: %w", err)
	}
	s.listener = listener

	go s.runEventLoop(ctx)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept connection: %w", err)
			}
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) runEventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-s.eventCh:
			if !ok {
				return
			}
			s.broadcast(evt)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	client := &client{conn: conn, id: s.nextID, done: make(chan struct{})}
	s.addClient(client)
	defer s.removeClient(client)

	client.send(response{Type: "connection.ready", Timestamp: time.Now().UTC()})
	client.send(response{Type: "sinfo.response", Data: s.currentCache(), Timestamp: time.Now().UTC()})

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		buf := scanner.Bytes()
		var req request
		if err := json.Unmarshal(buf, &req); err != nil {
			client.send(response{Type: "error", Error: fmt.Sprintf("invalid request: %v", err), Timestamp: time.Now().UTC()})
			continue
		}
		s.handleRequest(client, req)
	}

	conn.Close()
	close(client.done)
}

func (s *Server) handleRequest(c *client, req request) {
	switch req.Action {
	case "get_sinfo":
		c.send(response{Type: "sinfo.response", Data: s.currentCache(), Timestamp: time.Now().UTC()})
	case "subscribe":
		c.send(response{Type: "sinfo.subscribed", Data: nil, Timestamp: time.Now().UTC()})
	default:
		c.send(response{Type: "error", Error: fmt.Sprintf("unknown action %q", req.Action), Timestamp: time.Now().UTC()})
	}
}

func (s *Server) currentCache() map[string]interface{} {
	rows, updatedAt := s.cache.Get()
	return map[string]interface{}{
		"rows":       rows,
		"updated_at": updatedAt,
	}
}

func (s *Server) addClient(c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.id = s.nextID
	s.nextID++
	s.clients[c.id] = c
}

func (s *Server) removeClient(c *client) {
	s.mu.Lock()
	delete(s.clients, c.id)
	s.mu.Unlock()
	c.conn.Close()
}

func (s *Server) broadcast(evt slurm.Event) {
	s.mu.Lock()
	clients := make([]*client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	for _, c := range clients {
		c.send(response{Type: evt.Type, Data: evt.Data, Timestamp: evt.Timestamp})
	}
}

func (c *client) send(msg response) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	payload = append(payload, '\n')
	c.conn.Write(payload)
}
