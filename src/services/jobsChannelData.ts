import type { BridgeEnvelope } from '../types/bridge';
import type { SlurmJob } from '../types/slurm-api';
import type { SlurmJobsDelta } from '../features/jobs/jobsData';

export type JobsCachePayload = {
    jobs: SlurmJob[];
};

export function normalizeJobsDelta(value: unknown): SlurmJobsDelta | null {
    if (!value || typeof value !== 'object') {
        return null;
    }

    const record = value as Record<string, unknown>;
    const added = Array.isArray(record.added) ? record.added as SlurmJob[] : [];
    const modified = Array.isArray(record.modified) ? record.modified as SlurmJob[] : [];
    const deleted = Array.isArray(record.deleted) ? record.deleted as SlurmJob[] : [];

    if (added.length === 0 && modified.length === 0 && deleted.length === 0) {
        return null;
    }

    return { added, modified, deleted };
}

export function extractJobsDelta(message: BridgeEnvelope): SlurmJobsDelta | null {
    if (!message || typeof message !== 'object') {
        return null;
    }

    const record = message as Record<string, unknown>;
    if (record.type !== 'event' || (record.entity !== 'job' && record.entity !== 'jobs')) {
        return null;
    }

    return normalizeJobsDelta(record.data ?? record.payload ?? record);
}

export function normalizeJobsPayload(value: unknown): JobsCachePayload | null {
    if (Array.isArray(value)) {
        return { jobs: value as SlurmJob[] };
    }

    if (!value || typeof value !== 'object') {
        return null;
    }

    const record = value as Record<string, unknown>;
    const jobs = Array.isArray(record.jobs)
        ? record.jobs as SlurmJob[]
        : Array.isArray(record.items)
            ? record.items as SlurmJob[]
            : null;
    if (!jobs) {
        return null;
    }

    return { jobs };
}

export function extractJobsPayload(message: BridgeEnvelope): JobsCachePayload | null {
    if (message && typeof message === 'object') {
        const record = message as Record<string, unknown>;

        if (record.type === 'jobs.response' || record.type === 'jobs.snapshot') {
            return normalizeJobsPayload(record.data ?? record.payload ?? record);
        }

        if (record.type === 'snapshot' && (record.entity === 'job' || record.entity === 'jobs')) {
            return normalizeJobsPayload(record.payload ?? record.data ?? record);
        }

        if (record.type === 'event' && (record.entity === 'job' || record.entity === 'jobs')) {
            return normalizeJobsPayload(record.payload ?? record.data ?? record);
        }
    }

    return normalizeJobsPayload(message);
}
