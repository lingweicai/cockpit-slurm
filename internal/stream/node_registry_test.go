package stream

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

func TestNodeStreamSendsSnapshotBeforeChanges(t *testing.T) {
	cache := resource.NewNodeCache()
	if _, changed := cache.ReplaceSnapshot([]resource.Node{
		resource.NewNode("node001", 0),
		resource.NewNode("node002", 0),
	}); !changed {
		t.Fatal("initial snapshot did not change cache")
	}
	registry := newTestRegistry(t, cache, 8)
	sink := registerTestSink(t, registry)

	snapshot := receiveNodeMessage(t, sink.outbound)
	assertNodeEvent(t, snapshot, "snapshot", 1)
	var snapshotPayload struct {
		Nodes []resource.Node `json:"nodes"`
	}
	if err := json.Unmarshal(snapshot.Payload, &snapshotPayload); err != nil {
		t.Fatalf("decode snapshot payload: %v", err)
	}
	if len(snapshotPayload.Nodes) != 2 {
		t.Fatalf("snapshot nodes = %#v, want node001 and node002", snapshotPayload.Nodes)
	}
	identities := map[string]bool{}
	for _, node := range snapshotPayload.Nodes {
		identities[node.Identity()] = true
	}
	if !identities["node001"] || !identities["node002"] {
		t.Fatalf("snapshot identities = %#v, want node001 and node002", identities)
	}

	updated := resource.NewNode("node001", 0)
	updated.Status.State = "DOWN"
	added := resource.NewNode("node003", 0)
	cache.ReplaceSnapshotAndPublish([]resource.Node{updated, added}, registry)

	changes := receiveNodeMessage(t, sink.outbound)
	assertNodeEvent(t, changes, "changes", 2)
	var changePayload struct {
		Changes []resource.NodeChange `json:"changes"`
	}
	if err := json.Unmarshal(changes.Payload, &changePayload); err != nil {
		t.Fatalf("decode changes payload: %v", err)
	}
	if len(changePayload.Changes) != 3 {
		t.Fatalf("changes count = %d, want complete batch of 3", len(changePayload.Changes))
	}
	if changePayload.Changes[0].Kind != resource.NodeUpdated ||
		changePayload.Changes[1].Kind != resource.NodeRemoved ||
		changePayload.Changes[2].Kind != resource.NodeAdded {
		t.Fatalf("changes = %#v, want update, removal, and addition in one batch", changePayload.Changes)
	}
}

func TestNodeStreamRegistryFansOutAndCleansDisconnectedClient(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 0)})
	registry := newTestRegistry(t, cache, 8)
	sinkA := registerTestSink(t, registry)
	sinkB := registerTestSink(t, registry)
	assertNodeEvent(t, receiveNodeMessage(t, sinkA.outbound), "snapshot", 1)
	assertNodeEvent(t, receiveNodeMessage(t, sinkB.outbound), "snapshot", 1)

	sinkA.nodeStream.Unregister()
	waitFor(t, func() bool { return registry.ActiveCount() == 1 })

	updated := resource.NewNode("node001", 0)
	updated.Status.State = "IDLE"
	cache.ReplaceSnapshotAndPublish([]resource.Node{updated}, registry)
	assertNodeEvent(t, receiveNodeMessage(t, sinkB.outbound), "changes", 2)
	if got := registry.ActiveCount(); got != 1 {
		t.Fatalf("active streams = %d, want only connected client", got)
	}

	sinkB.nodeStream.Unregister()
	waitFor(t, func() bool { return registry.ActiveCount() == 0 })
}

func TestNodeStreamRegistrationAndCommitHaveNoSnapshotGap(t *testing.T) {
	for i := 0; i < 50; i++ {
		cache := resource.NewNodeCache()
		cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 0)})
		registry := newTestRegistry(t, cache, 4)
		outbound := make(chan protocol.Envelope, 4)
		var nodeStream *NodeStream

		var register sync.WaitGroup
		register.Add(1)
		go func() {
			defer register.Done()
			nodeStream = registry.Register(func(message protocol.Envelope) bool {
				select {
				case outbound <- message:
					return true
				default:
					return false
				}
			}, func() {})
		}()
		updated := resource.NewNode("node001", 0)
		updated.Spec.RealMemory = 1
		cache.ReplaceSnapshotAndPublish([]resource.Node{updated}, registry)
		register.Wait()

		first := receiveNodeMessage(t, outbound)
		if first.Payload == nil {
			t.Fatal("first stream message has no payload")
		}
		var payload struct {
			Event      string `json:"event"`
			Generation uint64 `json:"generation"`
		}
		if err := json.Unmarshal(first.Payload, &payload); err != nil {
			t.Fatalf("decode first message payload: %v", err)
		}
		switch {
		case payload.Event == "snapshot" && payload.Generation == 1:
			second := receiveNodeMessage(t, outbound)
			assertNodeEvent(t, second, "changes", 2)
		case payload.Event == "snapshot" && payload.Generation == 2:
		default:
			t.Fatalf("unexpected baseline: event=%q generation=%d", payload.Event, payload.Generation)
		}
		if nodeStream != nil {
			nodeStream.Unregister()
		}
		registry.CloseAll()
	}
}

func TestNodeStreamSerializesConcurrentPublications(t *testing.T) {
	cache := resource.NewNodeCache()
	registry := newTestRegistry(t, cache, 128)
	sink := registerTestSink(t, registry)
	assertNodeEvent(t, receiveNodeMessage(t, sink.outbound), "snapshot", 0)

	const publications = 64
	var writers sync.WaitGroup
	for i := 0; i < publications; i++ {
		writers.Add(1)
		go func(generation uint64) {
			defer writers.Done()
			registry.Publish(resource.NodeChangeBatch{
				Resource:   "node",
				Event:      "changes",
				Generation: generation + 1,
				Changes: []resource.NodeChange{{
					Kind: resource.NodeAdded,
					Node: resource.NewNode("node001", generation+1),
				}},
			})
		}(uint64(i))
	}
	writers.Wait()

	for i := 0; i < publications; i++ {
		message := receiveNodeMessage(t, sink.outbound)
		if message.Type != protocol.MessageEvent {
			t.Fatalf("message type = %q, want event", message.Type)
		}
	}
}

func TestNodeStreamQueueOverflowClosesOnlySlowStream(t *testing.T) {
	cache := resource.NewNodeCache()
	registry := newTestRegistry(t, cache, 1)
	sink := registerTestSink(t, registry)

	for generation := uint64(1); generation <= 3; generation++ {
		registry.Publish(resource.NodeChangeBatch{
			Resource:   "node",
			Event:      "changes",
			Generation: generation,
			Changes: []resource.NodeChange{{
				Kind: resource.NodeAdded,
				Node: resource.NewNode("node001", generation),
			}},
		})
	}

	waitFor(t, func() bool { return registry.ActiveCount() == 0 })
	select {
	case <-sink.nodeStream.Done():
	case <-time.After(time.Second):
		t.Fatal("slow stream was not closed after queue overflow")
	}
	select {
	case <-sink.closed:
	case <-time.After(time.Second):
		t.Fatal("connection close callback was not invoked after queue overflow")
	}
}

func newTestRegistry(t *testing.T, cache *resource.NodeCache, queueSize int) *NodeStreamRegistry {
	t.Helper()
	registry, err := NewNodeStreamRegistry(cache, queueSize)
	if err != nil {
		t.Fatalf("NewNodeStreamRegistry returned error: %v", err)
	}
	return registry
}

type testSink struct {
	nodeStream *NodeStream
	outbound   chan protocol.Envelope
	closed     chan struct{}
}

func registerTestSink(t *testing.T, registry *NodeStreamRegistry) *testSink {
	t.Helper()
	sink := &testSink{
		outbound: make(chan protocol.Envelope, registry.QueueSize()),
		closed:   make(chan struct{}),
	}
	sink.nodeStream = registry.Register(func(message protocol.Envelope) bool {
		select {
		case sink.outbound <- message:
			return true
		default:
			return false
		}
	}, func() { close(sink.closed) })
	if sink.nodeStream == nil {
		t.Fatal("Register returned nil stream")
	}
	t.Cleanup(sink.nodeStream.Unregister)
	return sink
}

func receiveNodeMessage(t *testing.T, outbound <-chan protocol.Envelope) protocol.Envelope {
	t.Helper()
	select {
	case envelope := <-outbound:
		return envelope
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream message")
		return protocol.Envelope{}
	}
}

func assertNodeEvent(t *testing.T, envelope protocol.Envelope, event string, generation uint64) {
	t.Helper()
	if envelope.Type != protocol.MessageEvent {
		t.Fatalf("message type = %q, want event", envelope.Type)
	}
	var payload struct {
		Resource   string `json:"resource"`
		Event      string `json:"event"`
		Generation uint64 `json:"generation"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("decode Node event payload: %v", err)
	}
	if payload.Resource != "node" || payload.Event != event || payload.Generation != generation {
		t.Fatalf("Node payload = %#v, want node/%s generation %d", payload, event, generation)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.After(time.Second)
	for !condition() {
		select {
		case <-deadline:
			t.Fatal("condition not satisfied before timeout")
		case <-time.After(time.Millisecond):
		}
	}
}
