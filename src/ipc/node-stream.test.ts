import { applyNodeStreamMessage, type NodeStreamState } from './node-stream.ts';
import { type NodeChangePayload, type NodeSnapshotPayload, type NodeStreamEnvelope, type NodeStreamNode } from './types.ts';

function assertEqual<T>(actual: T, expected: T, message: string): void {
    if (actual !== expected) {
        throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
    }
}

function assertThrows(callback: () => void, expectedMessage: string): void {
    try {
        callback();
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        if (message.includes(expectedMessage)) {
            return;
        }
        throw new Error(`expected error containing ${expectedMessage}, got ${message}`);
    }
    throw new Error(`expected error containing ${expectedMessage}`);
}

function makeNode(name: string, state = 'IDLE'): NodeStreamNode {
    return {
        metadata: {
            name,
            kind: 'Node',
            generation: 1,
            observedAt: '2026-10-08T00:00:00Z',
            source: 'cockpit-slurm',
        },
        spec: { nodeName: name },
        status: { state, stateFlags: [] },
    };
}

function message<TPayload extends NodeSnapshotPayload | NodeChangePayload>(payload: TPayload): NodeStreamEnvelope<TPayload> {
    return {
        protocol: 'cockpit-slurm',
        version: '1.0',
        messageId: 'NODE-1',
        type: 'event',
        payload,
    };
}

const emptyState: NodeStreamState = {
    resource: 'nodes',
    generation: 0,
    nodes: [],
    hasSnapshot: false,
};

const snapshot = message({
    resource: 'node',
    event: 'snapshot',
    generation: 10,
    nodes: [makeNode('node001'), makeNode('node002')],
});

const baseline = applyNodeStreamMessage(emptyState, snapshot);
assertEqual(baseline.generation, 10, 'snapshot should set generation');
assertEqual(baseline.nodes.length, 2, 'snapshot should replace client state');
assertEqual(baseline.hasSnapshot, true, 'snapshot should establish stream baseline');

const changed = applyNodeStreamMessage(baseline, message({
    resource: 'node',
    event: 'changes',
    generation: 11,
    changes: [
        { kind: 'updated', node: makeNode('node001', 'DOWN') },
        { kind: 'removed', node: makeNode('node002') },
        { kind: 'added', node: makeNode('node003') },
    ],
}));
assertEqual(changed.generation, 11, 'complete change batch should advance generation');
assertEqual(changed.nodes.length, 2, 'batch should apply update, remove, and add');
assertEqual(changed.nodes[0].status.state, 'DOWN', 'updated node should be replaced');
assertEqual(changed.nodes[1].spec.nodeName, 'node003', 'added node should be inserted');
assertEqual(baseline.nodes.length, 2, 'applying a batch should not mutate previous state');

assertThrows(
    () => applyNodeStreamMessage(baseline, message({
        resource: 'node',
        event: 'changes',
        generation: 13,
        changes: [{ kind: 'added', node: makeNode('node004') }],
    })),
    'generation gap',
);
assertThrows(
    () => applyNodeStreamMessage(baseline, message({
        resource: 'node',
        event: 'changes',
        generation: 10,
        changes: [{ kind: 'updated', node: makeNode('node001') }],
    })),
    'generation gap',
);
assertThrows(
    () => applyNodeStreamMessage(baseline, snapshot),
    'unexpected Node snapshot',
);
assertThrows(
    () => applyNodeStreamMessage(emptyState, message({
        resource: 'node',
        event: 'changes',
        generation: 1,
        changes: [{ kind: 'updated', node: makeNode('node001') }],
    })),
    'before initial snapshot',
);

const invalidBatch = message({
    resource: 'node',
    event: 'changes',
    generation: 11,
    changes: [
        { kind: 'updated', node: makeNode('node001', 'DOWN') },
        { kind: 'updated', node: makeNode('missing-node', 'DOWN') },
    ],
});
assertThrows(() => applyNodeStreamMessage(baseline, invalidBatch), 'updated missing Node');
assertEqual(baseline.nodes[0].status.state, 'IDLE', 'failed batch must not mutate the baseline');
assertEqual(baseline.generation, 10, 'failed batch must not advance generation');

const reconnectBaseline = applyNodeStreamMessage({ ...baseline, hasSnapshot: false }, message({
    resource: 'node',
    event: 'snapshot',
    generation: 20,
    nodes: [makeNode('node009')],
}));
assertEqual(reconnectBaseline.generation, 20, 'fresh connection snapshot should reset generation');
assertEqual(reconnectBaseline.nodes.length, 1, 'fresh connection snapshot should replace stale Nodes');
assertEqual(reconnectBaseline.nodes[0].spec.nodeName, 'node009', 'fresh snapshot should be authoritative');

console.log('Node stream state checks passed');
