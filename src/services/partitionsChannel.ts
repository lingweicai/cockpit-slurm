import cockpit from 'cockpit';

import type { BridgeEnvelope } from '../types/bridge';
import type { SlurmPartitionsDelta } from '../features/partitions/partitionsData';

import { fetchEntitySnapshot, subscribeEntityUpdates } from './entityChannel';
import { extractPartitionsDelta, extractPartitionsPayload, type PartitionsCachePayload } from './partitionsChannelData';

const _ = cockpit.gettext;

export async function fetchPartitions(): Promise<PartitionsCachePayload> {
    return fetchEntitySnapshot({
        entity: 'partition',
        extractPayload: extractPartitionsPayload,
        closedMessage: _('Partitions channel closed before a response was received.'),
    });
}

export function subscribePartitionUpdates(callback: (event: BridgeEnvelope, delta: SlurmPartitionsDelta | null) => void) {
    return subscribeEntityUpdates({
        entity: 'partition',
        extractDelta: extractPartitionsDelta,
        callback,
    });
}
