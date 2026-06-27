package socket

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	bridgeaccount "github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/account"
	bridgeentity "github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/entity"
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
	conn          net.Conn
	mu            sync.Mutex
	writeMu       sync.Mutex
	id            int
	connectionID  string
	subscriptions map[string]uint64
	done          chan struct{}
}

type request struct {
	RequestID  string          `json:"request_id,omitempty"`
	Type       string          `json:"type"`
	Entity     string          `json:"entity,omitempty"`
	ID         string          `json:"id,omitempty"`
	Generation uint64          `json:"generation,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type response struct {
	Type         string            `json:"type"`
	RequestID    string            `json:"request_id,omitempty"`
	Entity       string            `json:"entity,omitempty"`
	ConnectionID string            `json:"connection_id,omitempty"`
	Success      bool              `json:"success,omitempty"`
	Status       string            `json:"status,omitempty"`
	Generation   uint64            `json:"generation,omitempty"`
	Items        []*models.Account `json:"items,omitempty"`
	Added        []*models.Account `json:"added,omitempty"`
	Modified     []*models.Account `json:"modified,omitempty"`
	Deleted      []*models.Account `json:"deleted,omitempty"`
	Data         interface{}       `json:"data,omitempty"`
	Error        string            `json:"error,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

type accountRequestPayload struct {
	Name         string  `json:"name,omitempty"`
	AccountName  string  `json:"account_name,omitempty"`
	Description  *string `json:"description,omitempty"`
	Organization *string `json:"organization,omitempty"`
}

type accountSnapshot struct {
	Entity     string            `json:"entity"`
	Generation uint64            `json:"generation"`
	Items      []*models.Account `json:"items"`
	Count      int               `json:"count"`
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
	client := &client{
		conn:          conn,
		connectionID:  newConnectionID(),
		subscriptions: map[string]uint64{"account": s.currentGeneration()},
		done:          make(chan struct{}),
	}
	s.addClient(client)
	defer s.removeClient(client)

	go client.writeLoop()

	client.send(response{
		Type:         "connection.ready",
		ConnectionID: client.connectionID,
		Success:      true,
		Timestamp:    time.Now().UTC(),
	})
	client.send(snapshotResponse("", s.currentSnapshot()))

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			client.send(response{
				Type:      "error",
				Error:     fmt.Sprintf("invalid request: %v", err),
				Timestamp: time.Now().UTC(),
			})
			continue
		}
		s.handleRequest(ctx, client, req)
	}

	conn.Close()
	close(client.done)
}

func (s *Server) handleRequest(ctx context.Context, c *client, req request) {
	switch req.Type {
	case "list":
		if err := s.ensureAccountEntity(req.Entity); err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		c.send(snapshotResponse(req.RequestID, s.currentSnapshot()))
	case "get":
		if err := s.ensureAccountEntity(req.Entity); err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		account, ok := s.manager.Cache().Get(req.ID)
		if !ok {
			c.send(requestError(req.Entity, req.RequestID, fmt.Errorf("account %q not found", req.ID)))
			return
		}
		c.send(response{
			Type:      "object",
			RequestID: req.RequestID,
			Entity:    "account",
			Success:   true,
			Data:      account,
			Timestamp: time.Now().UTC(),
		})
	case "subscribe":
		if err := s.ensureAccountEntity(req.Entity); err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		c.mu.Lock()
		c.subscriptions["account"] = req.Generation
		c.mu.Unlock()
		c.send(response{
			Type:       "subscribe.response",
			RequestID:  req.RequestID,
			Entity:     "account",
			Success:    true,
			Generation: s.currentGeneration(),
			Timestamp:  time.Now().UTC(),
		})
		if req.Generation < s.currentGeneration() {
			c.send(snapshotResponse("", s.currentSnapshot()))
		}
	case "unsubscribe":
		if err := s.ensureAccountEntity(req.Entity); err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		c.mu.Lock()
		delete(c.subscriptions, "account")
		c.mu.Unlock()
		c.send(response{
			Type:      "unsubscribe.response",
			RequestID: req.RequestID,
			Entity:    "account",
			Success:   true,
			Timestamp: time.Now().UTC(),
		})
	case "create":
		payload, err := decodeAccountPayload(req.Payload)
		if err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		result, err := s.manager.AddAccount(ctx, req.RequestID, models.AccountCreateSpec{
			Name:         payload.accountName(),
			Description:  payload.stringValue(payload.Description),
			Organization: payload.stringValue(payload.Organization),
		})
		c.send(accountResultResponse(req.RequestID, result, err))
	case "update":
		payload, err := decodeAccountPayload(req.Payload)
		if err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		accountName := payload.accountName()
		if accountName == "" {
			accountName = req.ID
		}
		result, err := s.manager.ModifyAccount(ctx, req.RequestID, accountName, models.AccountUpdateSpec{
			Description:  payload.Description,
			Organization: payload.Organization,
		})
		c.send(accountResultResponse(req.RequestID, result, err))
	case "delete":
		payload, err := decodeAccountPayload(req.Payload)
		if err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		accountName := payload.accountName()
		if accountName == "" {
			accountName = req.ID
		}
		result, err := s.manager.DeleteAccount(ctx, req.RequestID, accountName)
		c.send(accountResultResponse(req.RequestID, result, err))
	default:
		c.send(requestError(req.Entity, req.RequestID, fmt.Errorf("unknown request type %q", req.Type)))
	}
}

func (s *Server) ensureAccountEntity(entity string) error {
	if entity == "" || entity == "account" {
		return nil
	}
	return fmt.Errorf("unsupported entity %q", entity)
}

func (s *Server) currentSnapshot() accountSnapshot {
	accounts, generation := s.manager.Snapshot()
	return accountSnapshot{
		Entity:     "account",
		Generation: generation,
		Items:      accounts,
		Count:      len(accounts),
	}
}

func (s *Server) currentGeneration() uint64 {
	_, generation := s.manager.Snapshot()
	return generation
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
	payload, ok := evt.Data.(bridgeentity.EntityEvent[models.Account])
	if !ok || payload.Entity != "account" {
		return
	}

	s.mu.Lock()
	clients := make([]*client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	for _, c := range clients {
		if !c.isSubscribed("account", payload.Generation) {
			continue
		}
		c.send(response{
			Type:       "event",
			Entity:     "account",
			Generation: payload.Generation,
			Added:      payload.Added,
			Modified:   payload.Modified,
			Deleted:    payload.Deleted,
			Timestamp:  evt.Timestamp,
		})
		c.setSubscriptionGeneration("account", payload.Generation)
	}
}

func (c *client) isSubscribed(entity string, generation uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	seen, ok := c.subscriptions[entity]
	return ok && generation > seen
}

func (c *client) setSubscriptionGeneration(entity string, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subscriptions == nil {
		c.subscriptions = make(map[string]uint64)
	}
	c.subscriptions[entity] = generation
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

func decodeAccountPayload(raw json.RawMessage) (accountRequestPayload, error) {
	if len(raw) == 0 {
		return accountRequestPayload{}, nil
	}

	var payload accountRequestPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return accountRequestPayload{}, fmt.Errorf("decode account payload: %w", err)
	}
	return payload, nil
}

func (p accountRequestPayload) accountName() string {
	if p.Name != "" {
		return p.Name
	}
	return p.AccountName
}

func (p accountRequestPayload) stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func snapshotResponse(requestID string, snapshot accountSnapshot) response {
	return response{
		Type:       "snapshot",
		RequestID:  requestID,
		Entity:     snapshot.Entity,
		Success:    true,
		Generation: snapshot.Generation,
		Items:      snapshot.Items,
		Timestamp:  time.Now().UTC(),
	}
}

func requestError(entity, requestID string, err error) response {
	return response{
		Type:      "error",
		RequestID: requestID,
		Entity:    entity,
		Success:   false,
		Error:     err.Error(),
		Timestamp: time.Now().UTC(),
	}
}

func accountResultResponse(requestID string, result bridgeaccount.RequestResult, err error) response {
	if err != nil {
		return response{
			Type:      "request.response",
			RequestID: requestID,
			Entity:    "account",
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now().UTC(),
		}
	}

	return response{
		Type:       "request.response",
		RequestID:  requestID,
		Entity:     "account",
		Success:    true,
		Status:     result.Status,
		Generation: result.Generation,
		Data:       result,
		Timestamp:  time.Now().UTC(),
	}
}

func newConnectionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("conn-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
