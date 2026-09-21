package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

// NodeQueryService returns node snapshots from the canonical cache.
type NodeQueryService struct {
	cache *resource.NodeCache
}

func NewNodeQueryService(cache *resource.NodeCache) *NodeQueryService {
	return &NodeQueryService{cache: cache}
}

func (s *NodeQueryService) Query(ctx context.Context, resourceName string) (resource.Snapshot, error) {
	if s == nil || s.cache == nil {
		return resource.Snapshot{}, fmt.Errorf("node query service is not initialized")
	}
	if resourceName == "" {
		return resource.Snapshot{}, fmt.Errorf("resource name is required")
	}
	if resourceName != "nodes" {
		return resource.Snapshot{}, fmt.Errorf("unsupported resource %q", resourceName)
	}
	return s.cache.Snapshot(), nil
}

// QueryHandler validates a query request and returns a protocol response envelope.
type QueryHandler struct {
	service *NodeQueryService
}

func NewQueryHandler(service *NodeQueryService) *QueryHandler {
	return &QueryHandler{service: service}
}

func (h *QueryHandler) Handle(ctx context.Context, msg protocol.Envelope) protocol.Envelope {
	var req struct {
		Resource string `json:"resource"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return protocol.NewEnvelope(msg.MessageID, protocol.MessageError, json.RawMessage(`{"code":"INVALID_MESSAGE","message":"invalid query payload"}`))
	}
	if req.Resource == "" {
		return protocol.NewEnvelope(msg.MessageID, protocol.MessageError, json.RawMessage(`{"code":"INVALID_MESSAGE","message":"missing resource"}`))
	}

	snapshot, err := h.service.Query(ctx, req.Resource)
	if err != nil {
		return protocol.NewEnvelope(msg.MessageID, protocol.MessageError, json.RawMessage(fmt.Sprintf(`{"code":"NOT_FOUND","message":%q}`, err.Error())))
	}

	payload, err := json.Marshal(map[string]any{
		"resource":   snapshot.Resource,
		"generation": snapshot.Generation,
		"nodes":      snapshot.Nodes,
	})
	if err != nil {
		return protocol.NewEnvelope(msg.MessageID, protocol.MessageError, json.RawMessage(`{"code":"INTERNAL_ERROR","message":"failed to encode snapshot"}`))
	}

	return protocol.NewEnvelope(msg.MessageID, protocol.MessageQueryResponse, payload)
}
