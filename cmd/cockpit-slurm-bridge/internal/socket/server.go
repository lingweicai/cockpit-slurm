package socket

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"os"
	"path/filepath"
	"reflect"
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
	jobState       polledEntityState[models.V0043JobInfo]
	nodeState      polledEntityState[models.V0043Node]
	partitionState polledEntityState[models.V0043PartitionInfo]
}

type entityDelta[T any] struct {
	Added    []T
	Modified []T
	Deleted  []T
}

type polledEntityState[T any] struct {
	Hash       string
	Generation uint64
	Index      map[string]T
}

type jobDelta = entityDelta[models.V0043JobInfo]

type nodeDelta = entityDelta[models.V0043Node]

type partitionDelta = entityDelta[models.V0043PartitionInfo]

func (d entityDelta[T]) empty() bool {
	return len(d.Added) == 0 && len(d.Modified) == 0 && len(d.Deleted) == 0
}





























































type unused struct{}

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
	go s.runJobPollLoop(ctx)
	go s.runNodePollLoop(ctx)
	go s.runPartitionPollLoop(ctx)

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

func (s *Server) runPartitionPollLoop(ctx context.Context) {
	const pollInterval = 15 * time.Second

	_ = s.pollPartitionsOnce(ctx, false)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.pollPartitionsOnce(ctx, true)
		}
	}
}

func (s *Server) runNodePollLoop(ctx context.Context) {
	const pollInterval = 15 * time.Second

	_ = s.pollNodesOnce(ctx, false)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.pollNodesOnce(ctx, true)
		}
	}
}

func (s *Server) runJobPollLoop(ctx context.Context) {
	const pollInterval = 15 * time.Second

	_ = s.pollJobsOnce(ctx, false)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.pollJobsOnce(ctx, true)
		}
	}
}

func (s *Server) pollJobsOnce(ctx context.Context, broadcast bool) error {
	jobsResp, hash, err := fetchJobs(ctx)
	if err != nil {
		return err
	}

	nextIndex := make(map[string]models.V0043JobInfo, len(jobsResp.Jobs))
	for i := range jobsResp.Jobs {
		job := jobsResp.Jobs[i]
		nextIndex[jobIdentity(job)] = job
	}

	s.mu.Lock()
	changed := hash != "" && hash != s.jobHash
	if s.jobHash == "" {
		s.jobHash = hash
		s.jobIndex = nextIndex
		if s.jobGen == 0 {
			s.jobGen = 1
		}
		s.mu.Unlock()
		return nil
	}

	if !changed {
		s.mu.Unlock()
		return nil
	}

	delta := diffJobs(s.jobIndex, nextIndex)
	s.jobHash = hash
	s.jobIndex = nextIndex
	s.jobGen++
	generation := s.jobGen
	clients := make([]*client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	if !broadcast || delta.empty() {
		return nil
	}

	for _, c := range clients {
		if !c.isSubscribed("job", generation) {
			continue
		}

		c.send(response{
			Type:       "event",
			Entity:     "job",
			Generation: generation,
			Success:    true,
			Data: map[string]interface{}{
				"added":    delta.Added,
				"modified": delta.Modified,
				"deleted":  delta.Deleted,
			},
			Timestamp:  time.Now().UTC(),
		})
		c.setSubscriptionGeneration("job", generation)
	}

	return nil
}

func (s *Server) pollNodesOnce(ctx context.Context, broadcast bool) error {
	nodesResp, hash, err := fetchNodes(ctx)
	if err != nil {
		return err
	}

	nextIndex := make(map[string]models.V0043Node, len(nodesResp.Nodes))
	for i := range nodesResp.Nodes {
		node := nodesResp.Nodes[i]
		nextIndex[nodeIdentity(node)] = node
	}

	s.mu.Lock()
	changed := hash != "" && hash != s.nodeHash
	if s.nodeHash == "" {
		s.nodeHash = hash
		s.nodeIndex = nextIndex
		if s.nodeGen == 0 {
			s.nodeGen = 1
		}
		s.mu.Unlock()
		return nil
	}

	if !changed {
		s.mu.Unlock()
		return nil
	}

	delta := diffNodes(s.nodeIndex, nextIndex)
	s.nodeHash = hash
	s.nodeIndex = nextIndex
	s.nodeGen++
	generation := s.nodeGen
	clients := make([]*client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	if !broadcast || delta.empty() {
		return nil
	}

	for _, c := range clients {
		if !c.isSubscribed("node", generation) {
			continue
		}

		c.send(response{
			Type:       "event",
			Entity:     "node",
			Generation: generation,
			Success:    true,
			Data: map[string]interface{}{
				"added":    delta.Added,
				"modified": delta.Modified,
				"deleted":  delta.Deleted,
			},
			Timestamp: time.Now().UTC(),
		})
		c.setSubscriptionGeneration("node", generation)
	}

	return nil
}

func (s *Server) pollPartitionsOnce(ctx context.Context, broadcast bool) error {
	partitionsResp, hash, err := fetchPartitions(ctx)
	if err != nil {
		return err
	}

	nextIndex := make(map[string]models.V0043PartitionInfo, len(partitionsResp.Partitions))
	for i := range partitionsResp.Partitions {
		partition := partitionsResp.Partitions[i]
		nextIndex[partitionIdentity(partition)] = partition
	}

	s.mu.Lock()
	changed := hash != "" && hash != s.partitionHash
	if s.partitionHash == "" {
		s.partitionHash = hash
		s.partitionIndex = nextIndex
		if s.partitionGen == 0 {
			s.partitionGen = 1
		}
		s.mu.Unlock()
		return nil
	}

	if !changed {
		s.mu.Unlock()
		return nil
	}

	delta := diffPartitions(s.partitionIndex, nextIndex)
	s.partitionHash = hash
	s.partitionIndex = nextIndex
	s.partitionGen++
	generation := s.partitionGen
	clients := make([]*client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	if !broadcast || delta.empty() {
		return nil
	}

	for _, c := range clients {
		if !c.isSubscribed("partition", generation) {
			continue
		}

		c.send(response{
			Type:       "event",
			Entity:     "partition",
			Generation: generation,
			Success:    true,
			Data: map[string]interface{}{
				"added":    delta.Added,
				"modified": delta.Modified,
				"deleted":  delta.Deleted,
			},
			Timestamp: time.Now().UTC(),
		})
		c.setSubscriptionGeneration("partition", generation)
	}

	return nil
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
		subscriptions: map[string]uint64{"account": s.currentGeneration("account")},
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
		s.handleListRequest(ctx, c, req)
	case "get":
		s.handleGetRequest(ctx, c, req)
	case "subscribe":
		entity, err := s.normalizeEntity(req.Entity)
		if err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		c.mu.Lock()
		c.subscriptions[entity] = req.Generation
		c.mu.Unlock()
		c.send(response{
			Type:       "subscribe.response",
			RequestID:  req.RequestID,
			Entity:     entity,
			Success:    true,
			Generation: s.currentGeneration(entity),
			Timestamp:  time.Now().UTC(),
		})
		if entity == "account" && req.Generation < s.currentGeneration(entity) {
			c.send(snapshotResponse("", s.currentSnapshot()))
		}
		if entity == "node" && req.Generation < s.currentGeneration(entity) {
			nodesResp, _, err := fetchNodes(ctx)
			if err == nil {
				c.send(response{
					Type:       "snapshot",
					Entity:     "node",
					Success:    true,
					Generation: s.currentGeneration("node"),
					Data: map[string]interface{}{
						"nodes": nodesResp.Nodes,
					},
					Timestamp: time.Now().UTC(),
				})
			}
		}
		if entity == "partition" && req.Generation < s.currentGeneration(entity) {
			partitionsResp, _, err := fetchPartitions(ctx)
			if err == nil {
				c.send(response{
					Type:       "snapshot",
					Entity:     "partition",
					Success:    true,
					Generation: s.currentGeneration("partition"),
					Data: map[string]interface{}{
						"partitions": partitionsResp.Partitions,
					},
					Timestamp: time.Now().UTC(),
				})
			}
		}
	case "unsubscribe":
		entity, err := s.normalizeEntity(req.Entity)
		if err != nil {
			c.send(requestError(req.Entity, req.RequestID, err))
			return
		}
		c.mu.Lock()
		delete(c.subscriptions, entity)
		c.mu.Unlock()
		c.send(response{
			Type:      "unsubscribe.response",
			RequestID: req.RequestID,
			Entity:    entity,
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

func (s *Server) handleListRequest(ctx context.Context, c *client, req request) {
	entity, err := s.normalizeEntity(req.Entity)
	if err != nil {
		c.send(requestError(req.Entity, req.RequestID, err))
		return
	}

	switch entity {
	case "account":
		c.send(snapshotResponse(req.RequestID, s.currentSnapshot()))
	case "job":
		jobsResp, _, err := fetchJobs(ctx)
		if err != nil {
			c.send(requestError(entity, req.RequestID, err))
			return
		}

		c.send(response{
			Type:       "snapshot",
			RequestID:  req.RequestID,
			Entity:     "job",
			Success:    true,
			Generation: s.currentGeneration("job"),
			Data: map[string]interface{}{
				"jobs": jobsResp.Jobs,
			},
			Timestamp: time.Now().UTC(),
		})
	case "node":
		nodesResp, _, err := fetchNodes(ctx)
		if err != nil {
			c.send(requestError(entity, req.RequestID, err))
			return
		}

		c.send(response{
			Type:       "snapshot",
			RequestID:  req.RequestID,
			Entity:     "node",
			Success:    true,
			Generation: s.currentGeneration("node"),
			Data: map[string]interface{}{
				"nodes": nodesResp.Nodes,
			},
			Timestamp: time.Now().UTC(),
		})
	case "partition":
		partitionsResp, _, err := fetchPartitions(ctx)
		if err != nil {
			c.send(requestError(entity, req.RequestID, err))
			return
		}

		c.send(response{
			Type:       "snapshot",
			RequestID:  req.RequestID,
			Entity:     "partition",
			Success:    true,
			Generation: s.currentGeneration("partition"),
			Data: map[string]interface{}{
				"partitions": partitionsResp.Partitions,
			},
			Timestamp: time.Now().UTC(),
		})
	}
}

func (s *Server) handleGetRequest(ctx context.Context, c *client, req request) {
	entity, err := s.normalizeEntity(req.Entity)
	if err != nil {
		c.send(requestError(req.Entity, req.RequestID, err))
		return
	}

	switch entity {
	case "account":
		account, ok := s.manager.Cache().Get(req.ID)
		if !ok {
			c.send(requestError(entity, req.RequestID, fmt.Errorf("account %q not found", req.ID)))
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
	case "job":
		if req.ID == "" {
			c.send(requestError(entity, req.RequestID, fmt.Errorf("job id is required")))
			return
		}

		jobsResp, _, err := fetchJobs(ctx)
		if err != nil {
			c.send(requestError(entity, req.RequestID, err))
			return
		}

		for i := range jobsResp.Jobs {
			job := jobsResp.Jobs[i]
			if job.JobId != nil && fmt.Sprint(*job.JobId) == req.ID {
				c.send(response{
					Type:      "object",
					RequestID: req.RequestID,
					Entity:    "job",
					Success:   true,
					Data:      job,
					Timestamp: time.Now().UTC(),
				})
				return
			}
		}

		c.send(requestError(entity, req.RequestID, fmt.Errorf("job %q not found", req.ID)))
	case "node":
		if req.ID == "" {
			c.send(requestError(entity, req.RequestID, fmt.Errorf("node id is required")))
			return
		}

		nodesResp, _, err := fetchNodes(ctx)
		if err != nil {
			c.send(requestError(entity, req.RequestID, err))
			return
		}

		for i := range nodesResp.Nodes {
			node := nodesResp.Nodes[i]
			if nodeIdentity(node) == req.ID {
				c.send(response{
					Type:      "object",
					RequestID: req.RequestID,
					Entity:    "node",
					Success:   true,
					Data:      node,
					Timestamp: time.Now().UTC(),
				})
				return
			}
		}

		c.send(requestError(entity, req.RequestID, fmt.Errorf("node %q not found", req.ID)))
	case "partition":
		if req.ID == "" {
			c.send(requestError(entity, req.RequestID, fmt.Errorf("partition id is required")))
			return
		}

		partitionsResp, _, err := fetchPartitions(ctx)
		if err != nil {
			c.send(requestError(entity, req.RequestID, err))
			return
		}

		for i := range partitionsResp.Partitions {
			partition := partitionsResp.Partitions[i]
			if partitionIdentity(partition) == req.ID {
				c.send(response{
					Type:      "object",
					RequestID: req.RequestID,
					Entity:    "partition",
					Success:   true,
					Data:      partition,
					Timestamp: time.Now().UTC(),
				})
				return
			}
		}

		c.send(requestError(entity, req.RequestID, fmt.Errorf("partition %q not found", req.ID)))
	}
}

func (s *Server) normalizeEntity(entity string) (string, error) {
	if entity == "" || entity == "account" {
		return "account", nil
	}

	if entity == "job" {
		return "job", nil
	}

	if entity == "node" {
		return "node", nil
	}

	if entity == "partition" {
		return "partition", nil
	}

	return "", fmt.Errorf("unsupported entity %q", entity)
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

func (s *Server) currentGeneration(entity string) uint64 {
	if entity == "job" {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.jobGen
	}

	if entity == "node" {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.nodeGen
	}

	if entity == "partition" {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.partitionGen
	}

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

func fetchJobs(ctx context.Context) (*models.V0043OpenapiJobInfoResp, string, error) {
	cmd := exec.CommandContext(ctx, "squeue", "--json")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("execute squeue --json: %v: %s", err, bytes.TrimSpace(raw))
	}

	var resp models.V0043OpenapiJobInfoResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("parse squeue JSON: %w", err)
	}

	sum := sha256.Sum256(raw)

	return &resp, hex.EncodeToString(sum[:]), nil
}

func fetchNodes(ctx context.Context) (*models.V0043OpenapiNodesResp, string, error) {
	cmd := exec.CommandContext(ctx, "scontrol", "show", "nodes", "--json")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("execute scontrol show nodes --json: %v: %s", err, bytes.TrimSpace(raw))
	}

	var resp models.V0043OpenapiNodesResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("parse scontrol node JSON: %w", err)
	}

	sum := sha256.Sum256(raw)

	return &resp, hex.EncodeToString(sum[:]), nil
}

func fetchPartitions(ctx context.Context) (*models.V0043OpenapiPartitionResp, string, error) {
	cmd := exec.CommandContext(ctx, "scontrol", "show", "partition", "--json")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("execute scontrol show partition --json: %v: %s", err, bytes.TrimSpace(raw))
	}

	var resp models.V0043OpenapiPartitionResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, "", fmt.Errorf("parse scontrol partition JSON: %w", err)
	}

	sum := sha256.Sum256(raw)

	return &resp, hex.EncodeToString(sum[:]), nil
}

func jobIdentity(job models.V0043JobInfo) string {
	if job.JobId != nil {
		return fmt.Sprintf("job:%d", *job.JobId)
	}

	name := ""
	if job.Name != nil {
		name = *job.Name
	}

	user := ""
	if job.UserName != nil {
		user = *job.UserName
	}

	submit := int64(0)
	if job.SubmitTime != nil && job.SubmitTime.Number != nil {
		submit = *job.SubmitTime.Number
	}

	return fmt.Sprintf("job:fallback:%s:%s:%d", name, user, submit)
}

func diffJobs(prev, next map[string]models.V0043JobInfo) jobDelta {
	delta := jobDelta{}

	for key, nextJob := range next {
		prevJob, ok := prev[key]
		if !ok {
			delta.Added = append(delta.Added, nextJob)
			continue
		}

		if !reflect.DeepEqual(prevJob, nextJob) {
			delta.Modified = append(delta.Modified, nextJob)
		}
	}

	for key, prevJob := range prev {
		if _, ok := next[key]; !ok {
			delta.Deleted = append(delta.Deleted, prevJob)
		}
	}

	return delta
}

func nodeIdentity(node models.V0043Node) string {
	if node.Name != nil && *node.Name != "" {
		return *node.Name
	}

	if node.Hostname != nil && *node.Hostname != "" {
		return *node.Hostname
	}

	return "unknown"
}

func diffNodes(prev, next map[string]models.V0043Node) nodeDelta {
	delta := nodeDelta{}

	for key, nextNode := range next {
		prevNode, ok := prev[key]
		if !ok {
			delta.Added = append(delta.Added, nextNode)
			continue
		}

		if !reflect.DeepEqual(prevNode, nextNode) {
			delta.Modified = append(delta.Modified, nextNode)
		}
	}

	for key, prevNode := range prev {
		if _, ok := next[key]; !ok {
			delta.Deleted = append(delta.Deleted, prevNode)
		}
	}

	return delta
}

func partitionIdentity(partition models.V0043PartitionInfo) string {
	if partition.Name != nil && *partition.Name != "" {
		return *partition.Name
	}

	return "unknown"
}

func diffPartitions(prev, next map[string]models.V0043PartitionInfo) partitionDelta {
	delta := partitionDelta{}

	for key, nextPartition := range next {
		prevPartition, ok := prev[key]
		if !ok {
			delta.Added = append(delta.Added, nextPartition)
			continue
		}

		if !reflect.DeepEqual(prevPartition, nextPartition) {
			delta.Modified = append(delta.Modified, nextPartition)
		}
	}

	for key, prevPartition := range prev {
		if _, ok := next[key]; !ok {
			delta.Deleted = append(delta.Deleted, prevPartition)
		}
	}

	return delta
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
