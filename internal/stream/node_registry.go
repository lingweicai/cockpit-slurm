package stream

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

const DefaultNodeStreamQueueSize = 32

type nodeSnapshotPayload struct {
	Resource   string          `json:"resource"`
	Event      string          `json:"event"`
	Generation uint64          `json:"generation"`
	Nodes      []resource.Node `json:"nodes"`
}

// NodeStreamRegistry fans committed Node changes out to connected streams.
type NodeStreamRegistry struct {
	cache      *resource.NodeCache
	queueSize  int
	mu         sync.Mutex
	streams    map[*NodeStream]struct{}
	messageSeq atomic.Uint64
}

func NewNodeStreamRegistry(cache *resource.NodeCache, queueSize int) (*NodeStreamRegistry, error) {
	if cache == nil {
		return nil, errors.New("node cache is required")
	}
	if queueSize <= 0 {
		return nil, fmt.Errorf("node stream queue size must be positive: %d", queueSize)
	}
	return &NodeStreamRegistry{
		cache:     cache,
		queueSize: queueSize,
		streams:   make(map[*NodeStream]struct{}),
	}, nil
}

// QueueSize is the bounded outbound capacity required for each registered stream.
func (r *NodeStreamRegistry) QueueSize() int {
	if r == nil {
		return 0
	}
	return r.queueSize
}

// Register subscribes a sender and enqueues its authoritative snapshot before
// allowing any later cache commit to publish to the new stream.
func (r *NodeStreamRegistry) Register(enqueue func(protocol.Envelope) bool, closeConnection func()) *NodeStream {
	if r == nil || enqueue == nil {
		if closeConnection != nil {
			closeConnection()
		}
		return nil
	}

	var stream *NodeStream
	r.cache.WithSnapshotBoundary(func(snapshot resource.Snapshot) {
		envelope, err := r.snapshotEnvelope(snapshot)
		if err != nil {
			log.Printf("encode Node stream snapshot: %v", err)
			if closeConnection != nil {
				closeConnection()
			}
			return
		}

		r.mu.Lock()
		stream = newNodeStream(r, enqueue, closeConnection)
		r.streams[stream] = struct{}{}
		if !stream.enqueue(envelope) {
			delete(r.streams, stream)
			r.mu.Unlock()
			stream.Close()
			stream = nil
			return
		}
		r.mu.Unlock()
	})
	return stream
}

func (r *NodeStreamRegistry) Publish(batch resource.NodeChangeBatch) {
	if r == nil {
		return
	}
	if batch.Resource != "node" || batch.Event != "changes" || batch.Generation == 0 || len(batch.Changes) == 0 {
		log.Printf("reject invalid Node change batch at generation %d", batch.Generation)
		return
	}

	envelope, err := r.eventEnvelope(batch)
	if err != nil {
		log.Printf("encode Node change batch at generation %d: %v", batch.Generation, err)
		return
	}

	var slow []*NodeStream
	r.mu.Lock()
	for stream := range r.streams {
		if !stream.enqueue(envelope) {
			delete(r.streams, stream)
			slow = append(slow, stream)
		}
	}
	r.mu.Unlock()

	for _, stream := range slow {
		stream.Close()
	}
}

func (r *NodeStreamRegistry) Unregister(stream *NodeStream) {
	if r == nil || stream == nil {
		return
	}
	r.mu.Lock()
	delete(r.streams, stream)
	r.mu.Unlock()
}

func (r *NodeStreamRegistry) CloseAll() {
	if r == nil {
		return
	}
	r.mu.Lock()
	streams := make([]*NodeStream, 0, len(r.streams))
	for stream := range r.streams {
		delete(r.streams, stream)
		streams = append(streams, stream)
	}
	r.mu.Unlock()
	for _, stream := range streams {
		stream.Close()
	}
}

func (r *NodeStreamRegistry) ActiveCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.streams)
}

func (r *NodeStreamRegistry) snapshotEnvelope(snapshot resource.Snapshot) (protocol.Envelope, error) {
	payload, err := json.Marshal(nodeSnapshotPayload{
		Resource:   "node",
		Event:      "snapshot",
		Generation: snapshot.Generation,
		Nodes:      snapshot.Nodes,
	})
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("marshal snapshot payload: %w", err)
	}
	return protocol.NewEnvelope(r.nextMessageID(), protocol.MessageEvent, payload), nil
}

func (r *NodeStreamRegistry) eventEnvelope(batch resource.NodeChangeBatch) (protocol.Envelope, error) {
	payload, err := json.Marshal(batch)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("marshal change payload: %w", err)
	}
	return protocol.NewEnvelope(r.nextMessageID(), protocol.MessageEvent, payload), nil
}

func (r *NodeStreamRegistry) nextMessageID() string {
	return fmt.Sprintf("NODE-%012d", r.messageSeq.Add(1))
}

type NodeStream struct {
	registry        *NodeStreamRegistry
	enqueueOutbound func(protocol.Envelope) bool
	closeConnection func()
	done            chan struct{}
	close           sync.Once
}

func newNodeStream(registry *NodeStreamRegistry, enqueue func(protocol.Envelope) bool, closeConnection func()) *NodeStream {
	return &NodeStream{
		registry:        registry,
		enqueueOutbound: enqueue,
		closeConnection: closeConnection,
		done:            make(chan struct{}),
	}
}

func (s *NodeStream) enqueue(envelope protocol.Envelope) bool {
	select {
	case <-s.done:
		return false
	default:
	}
	return s.enqueueOutbound(envelope)
}

func (s *NodeStream) Close() {
	s.close.Do(func() {
		close(s.done)
		s.registry.Unregister(s)
		if s.closeConnection != nil {
			s.closeConnection()
		}
	})
}

func (s *NodeStream) Unregister() {
	s.close.Do(func() {
		close(s.done)
		s.registry.Unregister(s)
	})
}

func (s *NodeStream) Done() <-chan struct{} {
	return s.done
}
