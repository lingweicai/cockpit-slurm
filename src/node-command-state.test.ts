import { IpcCommandError } from './ipc/command-service.ts';
import {
    availableNodeActions,
    commandFailureStatus,
    commandStatusText,
    isCommandActive,
    nodeCommandBlockReason,
    type NodeCommandAvailability,
} from './node-command-state.ts';
import { type NodeCapabilities, type NodeStreamNode } from './ipc/types.ts';

function assertEqual<T>(actual: T, expected: T, message: string): void {
    if (actual !== expected) {
        throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
    }
}

function assertActions(actual: string[], expected: string[], message: string): void {
    assertEqual(actual.join(','), expected.join(','), message);
}

function makeNode(state: string | undefined = 'IDLE', stateFlags: string[] = []): NodeStreamNode {
    return {
        metadata: {
            name: 'c21',
            kind: 'Node',
            generation: 1,
            observedAt: '2026-10-25T00:00:00Z',
            source: 'cockpit-slurm',
        },
        spec: { nodeName: 'c21' },
        status: { state, stateFlags },
    };
}

const adminCapabilities: NodeCapabilities = { 'nodes.drain': true, 'nodes.resume': true };
const idleAvailability: Omit<NodeCommandAvailability, 'action'> = {
    node: makeNode(),
    capabilities: adminCapabilities,
    capabilitiesLoading: false,
    connected: true,
};

assertActions(availableNodeActions({ ...idleAvailability, capabilities: null }), [], 'missing capabilities hide actions');
assertActions(availableNodeActions({ ...idleAvailability, capabilitiesLoading: true }), [], 'loading capabilities hide actions');
assertActions(availableNodeActions({ ...idleAvailability, connected: false }), [], 'disconnected stream hides actions');
assertActions(availableNodeActions({ ...idleAvailability, node: makeNode('') }), [], 'missing observed state hides actions');
assertEqual(
    nodeCommandBlockReason({ ...idleAvailability, action: 'DRAIN', node: undefined }),
    'The latest Node state is unavailable',
    'submission guard rejects a missing Node snapshot',
);

assertActions(availableNodeActions(idleAvailability), ['DRAIN'], 'admin can drain an observed idle node');
assertActions(
    availableNodeActions({ ...idleAvailability, capabilities: { 'nodes.drain': false, 'nodes.resume': false } }),
    [],
    'non-admin capabilities hide mutation actions',
);
assertActions(
    availableNodeActions({ ...idleAvailability, node: makeNode('IDLE', ['DRAIN']) }),
    ['RESUME'],
    'composite IDLE+DRAIN state exposes resume only',
);
assertActions(
    availableNodeActions({ ...idleAvailability, node: makeNode('ALLOCATED') }),
    [],
    'inapplicable node state hides actions',
);

for (const status of ['pending', 'running', 'unknown'] as const) {
    const command = { action: 'DRAIN' as const, status, commandId: `cmd-${status}` };
    assertEqual(isCommandActive(status), true, `${status} is considered active`);
    assertActions(availableNodeActions({ ...idleAvailability, command }), [], `${status} blocks duplicate actions`);
    assertEqual(
        nodeCommandBlockReason({ ...idleAvailability, action: 'DRAIN', command }) !== null,
        true,
        `${status} submission is rejected`,
    );
}

assertEqual(isCommandActive('succeeded'), false, 'terminal success is not active');
assertEqual(commandStatusText({ action: 'DRAIN', status: 'pending' }), 'Submitting drain', 'pending status text');
assertEqual(commandStatusText({ action: 'RESUME', status: 'failed' }), 'resume: failed', 'terminal status text');
assertEqual(
    commandFailureStatus(new IpcCommandError({
        code: 'authorization-denied',
        category: 'authorization-error',
        message: 'Not authorized',
        retryable: false,
    })),
    'denied',
    'authorization error remains distinct',
);
assertEqual(
    commandFailureStatus(new IpcCommandError({
        code: 'command-outcome-unknown',
        category: 'timeout',
        message: 'Outcome unknown',
        retryable: false,
    })),
    'unknown',
    'uncertain execution blocks automatic resubmission',
);
assertEqual(commandFailureStatus(new Error('transport disconnected')), 'unknown', 'unclassified transport failure is unknown');

console.log('Node command UI policy checks passed');
