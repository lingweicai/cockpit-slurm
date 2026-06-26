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

	bridgeaccount "github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/account"
	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
)

type Server struct {
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	clients    map[int]*client
	nextID     int
	manager    *bridgeaccount.Manager
	events     <-chan bridgeaccount.Event
}

type client struct {
	conn    net.Conn
	writeMu sync.Mutex
	id      int
	done    chan struct{}
}

type request struct {
	RequestID string          `json:"request_id,omitempty"`
	Entity    string          `json:"entity,omitempty"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type response struct {
	Type      string      `json:"type"`
	RequestID string      `json:"request_id,omitempty"`
	Status    string      `json:"status,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type accountActionPayload struct {
	Name         string  `json:"name,omitempty"`
	AccountName  string  `json:"account_name,omitempty"`
	Description  *string `json:"description,omitempty"`
	Organization *string `json:"organization,omitempty"`
}

func NewServer(socketPath string, manager *bridgeaccount.Manager) *Server {
	return &Server{
		socketPath: socketPath,
		clients:    make(map[int]*client),
		manager:    manager,
		events:     manager.EventChannel(),
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
		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) runEventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-s.events:
			if !ok {
				return
			}
			s.broadcast(evt)
		}
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	client := &client{conn: conn, done: make(chan struct{})}
	s.addClient(client)
	defer s.removeClient(client)

	go client.writeLoop()

	client.send(response{Type: "connection.ready", Timestamp: time.Now().UTC()})
	client.send(response{Type: "account.snapshot", Data: s.currentSnapshot(), Timestamp: time.Now().UTC()})

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			client.send(response{Type: "error", Error: fmt.Sprintf("invalid request: %v", err), Timestamp: time.Now().UTC()})
			continue
		}
		s.handleRequest(ctx, client, req)
	}

	conn.Close()
	close(client.done)
}

func (s *Server) handleRequest(ctx context.Context, c *client, req request) {
	switch req.Action {
	case "get_accounts":
		c.send(response{Type: "account.snapshot", Data: s.currentSnapshot(), Timestamp: time.Now().UTC()})
	case "subscribe":
		c.send(response{Type: "account.subscribed", Timestamp: time.Now().UTC()})
	case "add_account":
		payload, err := decodeAccountPayload(req.Payload)
		if err != nil {
			c.send(response{Type: "request.response", RequestID: req.RequestID, Status: "error", Error: err.Error(), Timestamp: time.Now().UTC()})
			return
		}
		result, err := s.manager.AddAccount(ctx, req.RequestID, models.AccountCreateSpec{
			Name:         payload.accountName(),
			Description:  payload.stringValue(payload.Description),
			Organization: payload.stringValue(payload.Organization),
		})
		c.send(accountResultResponse(req.RequestID, result, err))
	case "modify_account":
		payload, err := decodeAccountPayload(req.Payload)
		if err != nil {
			c.send(response{Type: "request.response", RequestID: req.RequestID, Status: "error", Error: err.Error(), Timestamp: time.Now().UTC()})
			return
		}
		accountName := payload.accountName()
		result, err := s.manager.ModifyAccount(ctx, req.RequestID, accountName, models.AccountUpdateSpec{
			Description:  payload.Description,
			Organization: payload.Organization,
		})
		c.send(accountResultResponse(req.RequestID, result, err))
	case "delete_account":
		payload, err := decodeAccountPayload(req.Payload)
		if err != nil {
			c.send(response{Type: "request.response", RequestID: req.RequestID, Status: "error", Error: err.Error(), Timestamp: time.Now().UTC()})
			return
		}
		result, err := s.manager.DeleteAccount(ctx, req.RequestID, payload.accountName())
		c.send(accountResultResponse(req.RequestID, result, err))
	default:
		c.send(response{Type: "request.response", RequestID: req.RequestID, Status: "error", Error: fmt.Sprintf("unknown action %q", req.Action), Timestamp: time.Now().UTC()})
	}
}

func (s *Server) currentSnapshot() map[string]interface{} {
	accounts, generation := s.manager.Snapshot()
	return map[string]interface{}{
		"entity":     "account",
		"generation": generation,
		"accounts":   accounts,
		"count":      len(accounts),
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

func (s *Server) broadcast(evt bridgeaccount.Event) {
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
	select {
	case <-c.done:
		return
	default:
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	payload = append(payload, '\n')
	_, _ = c.conn.Write(payload)
}

func (c *client) writeLoop() {
	<-c.done
}

func decodeAccountPayload(raw json.RawMessage) (accountActionPayload, error) {
	if len(raw) == 0 {
		return accountActionPayload{}, nil
	}

	var payload accountActionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return accountActionPayload{}, fmt.Errorf("decode account payload: %w", err)
	}
	return payload, nil
}

func (p accountActionPayload) accountName() string {
	if p.Name != "" {
		return p.Name
	}
	return p.AccountName
}

func (p accountActionPayload) stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func accountResultResponse(requestID string, result bridgeaccount.RequestResult, err error) response {
	if err != nil {
		return response{
			Type:      "request.response",
			RequestID: requestID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now().UTC(),
		}
	}

	return response{
		Type:      "request.response",
		RequestID: requestID,
		Status:    result.Status,
		Data:      result,
		Timestamp: time.Now().UTC(),
	}
}
