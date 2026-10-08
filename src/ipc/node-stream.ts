import { validateNodeStreamMessage } from './framing.ts';
import { type NodeStreamMessage, type NodeStreamNode } from './types.ts';

export interface NodeStreamState {
  resource: 'nodes';
  generation: number;
  nodes: NodeStreamNode[];
  hasSnapshot: boolean;
}

export function applyNodeStreamMessage(state: NodeStreamState, value: unknown): NodeStreamState {
    if (!validateNodeStreamMessage(value)) {
        throw new Error('invalid Node stream message');
    }

    const message: NodeStreamMessage = value;
    const payload = message.payload;
    if (payload.event === 'snapshot') {
        if (state.hasSnapshot) {
            throw new Error('unexpected Node snapshot after stream baseline');
        }
        return {
            resource: 'nodes',
            generation: payload.generation,
            nodes: payload.nodes,
            hasSnapshot: true,
        };
    }

    if (!state.hasSnapshot) {
        throw new Error('Node changes received before initial snapshot');
    }
    if (payload.generation !== state.generation + 1) {
        throw new Error(`Node stream generation gap: received ${payload.generation} after ${state.generation}`);
    }

    const nodesByIdentity = new Map(state.nodes.map(node => [node.spec.nodeName, node]));
    for (const change of payload.changes) {
        const identity = change.node.spec.nodeName;
        const exists = nodesByIdentity.has(identity);

        switch (change.kind) {
        case 'added':
            if (exists) {
                throw new Error(`Node stream added existing Node ${identity}`);
            }
            nodesByIdentity.set(identity, change.node);
            break;
        case 'updated':
            if (!exists) {
                throw new Error(`Node stream updated missing Node ${identity}`);
            }
            nodesByIdentity.set(identity, change.node);
            break;
        case 'removed':
            if (!exists) {
                throw new Error(`Node stream removed missing Node ${identity}`);
            }
            nodesByIdentity.delete(identity);
            break;
        }
    }

    return {
        resource: 'nodes',
        generation: payload.generation,
        nodes: [...nodesByIdentity.values()],
        hasSnapshot: true,
    };
}
