import cockpit from 'cockpit';

import type { BridgeEnvelope } from '../types/bridge';
import type { SlurmNodesDelta } from '../features/nodes/nodesData';
import { fetchEntitySnapshot, subscribeEntityUpdates } from './entityChannel';
import { extractNodesDelta, extractNodesPayload, type NodesCachePayload } from './nodesChannelData';

const _ = cockpit.gettext;

export async function fetchNodes(): Promise<NodesCachePayload> {
    return fetchEntitySnapshot({
        entity: 'node',
        extractPayload: extractNodesPayload,
        closedMessage: _('Nodes channel closed before a response was received.'),
    });
}

export function subscribeNodeUpdates(callback: (event: BridgeEnvelope, delta: SlurmNodesDelta | null) => void) {
    return subscribeEntityUpdates({
        entity: 'node',
        extractDelta: extractNodesDelta,
        callback,
    });
}
