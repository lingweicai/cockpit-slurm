import cockpit from 'cockpit';

import type { BridgeEnvelope } from '../types/bridge';
import type { SlurmJobsDelta } from '../features/jobs/jobsData';
import { fetchEntitySnapshot, subscribeEntityUpdates } from './entityChannel';
import { extractJobsDelta, extractJobsPayload, type JobsCachePayload } from './jobsChannelData';

const _ = cockpit.gettext;

export async function fetchJobs(): Promise<JobsCachePayload> {
    return fetchEntitySnapshot({
        entity: 'job',
        extractPayload: extractJobsPayload,
        closedMessage: _('Jobs channel closed before a response was received.'),
    });
}

export function subscribeJobsUpdates(callback: (event: BridgeEnvelope, delta: SlurmJobsDelta | null) => void) {
    return subscribeEntityUpdates({
        entity: 'job',
        extractDelta: extractJobsDelta,
        callback,
        shouldHandleMessage: (message) => {
            if (!message || typeof message !== 'object') {
                return false;
            }

            const record = message as Record<string, unknown>;
            return record.type === 'jobs.updated' ||
                record.type === 'jobs.event' ||
                (record.type === 'event' && (record.entity === 'job' || record.entity === 'jobs'));
        },
    });
}
