package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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

// Register adds a connection and enqueues its authoritative snapshot before
// allowing any later cache commit to publish to the new stream.
func (r *NodeStreamRegistry) Register(conn net.Conn) *NodeStream {
	if r == nil || conn == nil {
		return nil
	}

	var stream *NodeStream
	r.cache.WithSnapshotBoundary(func(snapshot resource.Snapshot) {
		envelope, err := r.snapshotEnvelope(snapshot)
		if err != nil {
			log.Printf("encode Node stream snapshot: %v", err)
			_ = conn.Close()
			return
		}

		r.mu.Lock()
		stream = newNodeStream(conn, r, r.queueSize)
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

	if stream != nil {
		stream.start()
	}
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
	conn     net.Conn
	registry *NodeStreamRegistry
	outbound chan protocol.Envelope
	done     chan struct{}
	close    sync.Once
}

func newNodeStream(conn net.Conn, registry *NodeStreamRegistry, queueSize int) *NodeStream {
	return &NodeStream{
		conn:     conn,
		registry: registry,
		outbound: make(chan protocol.Envelope, queueSize),
		done:     make(chan struct{}),
	}
}

func (s *NodeStream) enqueue(envelope protocol.Envelope) bool {
	select {
	case <-s.done:
		return false
	case s.outbound <- envelope:
		return true
	default:
		return false
	}
}

func (s *NodeStream) start() {
	go s.writeLoop()
	go s.readLoop()
}

func (s *NodeStream) writeLoop() {
	encoder := protocol.NewEncoder(s.conn)
	for {
		select {
		case <-s.done:
			return
		case envelope := <-s.outbound:
			if err := encoder.Encode(envelope); err != nil {
				select {
				case <-s.done:
				default:
					log.Printf("write Node stream: %v", err)
				}
				s.Close()
				return
			}
		}
	}
}

func (s *NodeStream) readLoop() {
	var buf [1]byte
	for {
		n, err := s.conn.Read(buf[:])
		if n > 0 {
			log.Printf("Node stream received unexpected client data")
			s.Close()
			return
		}
		if err != nil {
			if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
				log.Printf("read Node stream: %v", err)
			}
			s.Close()
			return
		}
	}
}

func (s *NodeStream) Close() {
	s.close.Do(func() {
		close(s.done)
		_ = s.conn.Close()
		s.registry.Unregister(s)
	})
}

func (s *NodeStream) Done() <-chan struct{} {
	return s.done
}

func (s *NodeStream) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		s.Close()
		return ctx.Err()
	}
}
