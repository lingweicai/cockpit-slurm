import type { JobRecord, JobState } from '../../types/job';
import type { SlurmJob, SlurmJobsResponse } from '../../types/slurm-api';

type SlurmJobsPayload = Pick<SlurmJobsResponse, 'jobs'>;

export type SlurmJobsDelta = {
    added?: SlurmJob[];
    modified?: SlurmJob[];
    deleted?: SlurmJob[];
};

function buildHistory(state: JobState, submitTime: string, startTime: string | null, endTime: string | null) {
    const history = [
        { timestamp: submitTime, event: 'submitted', detail: 'Job submitted to Slurm' },
    ];

    if (startTime) {
        history.push({ timestamp: startTime, event: 'started', detail: 'Job started running' });
    }

    if (endTime) {
        history.push({ timestamp: endTime, event: 'finished', detail: state === 'FAILED' ? 'Job failed' : 'Job completed' });
    }

    return history;
}

function normalizeTimestamp(raw: unknown) {
    if (raw && typeof raw === 'object' && 'number' in raw) {
        const unix = Number((raw as { number?: unknown }).number);
        if (Number.isFinite(unix) && unix > 0) {
            return new Date(unix * 1000).toISOString();
        }
    }

    return null;
}

function normalizeInteger(raw: unknown) {
    if (typeof raw === 'number' && Number.isFinite(raw)) {
        return raw;
    }

    if (raw && typeof raw === 'object' && 'number' in raw) {
        const value = Number((raw as { number?: unknown }).number);
        if (Number.isFinite(value)) {
            return value;
        }
    }

    return 0;
}

function mapState(state: SlurmJob['job_state']): JobState {
    const primaryState = Array.isArray(state) ? state[0] : undefined;

    switch (primaryState) {
    case 'RUNNING':
    case 'PENDING':
    case 'FAILED':
    case 'COMPLETED':
    case 'CANCELLED':
        return primaryState;
    default:
        return 'PENDING';
    }
}

function formatRuntime(startTime: string | null, endTime: string | null) {
    if (!startTime) {
        return '00:00:00';
    }

    const startMs = Date.parse(startTime);
    const endMs = endTime ? Date.parse(endTime) : Date.now();
    if (Number.isNaN(startMs) || Number.isNaN(endMs) || endMs <= startMs) {
        return '00:00:00';
    }

    const totalSeconds = Math.floor((endMs - startMs) / 1000);
    const hours = Math.floor(totalSeconds / 3600)
            .toString()
            .padStart(2, '0');
    const minutes = Math.floor((totalSeconds % 3600) / 60)
            .toString()
            .padStart(2, '0');
    const seconds = (totalSeconds % 60).toString().padStart(2, '0');
    return `${hours}:${minutes}:${seconds}`;
}

export function mapSlurmJobToRecord(job: SlurmJob): JobRecord {
    const state = mapState(job.job_state);
    const submitTime = normalizeTimestamp(job.submit_time) ?? new Date(0).toISOString();
    const startTime = normalizeTimestamp(job.start_time);
    const endTime = normalizeTimestamp(job.end_time);
    const jobId = String(job.job_id ?? 'unknown');

    return {
        jobId,
        name: job.name ?? `job-${jobId}`,
        user: job.user_name ?? String(job.user_id ?? 'unknown'),
        account: job.account ?? 'unknown',
        partition: job.partition ?? 'unknown',
        state,
        runtime: formatRuntime(startTime, endTime),
        nodes: normalizeInteger(job.node_count),
        cpus: normalizeInteger(job.cpus),
        submitTime,
        startTime,
        endTime,
        nodeList: job.nodes ?? '',
        qos: job.qos ?? '',
        command: job.command ?? '',
        workDir: job.current_working_directory ?? '',
        stdout: job.standard_output ?? '',
        stderr: job.standard_error ?? '',
        environment: {},
        history: buildHistory(state, submitTime, startTime, endTime),
    };
}

export function mapSlurmJobsResponseToRecords(response: SlurmJobsPayload): JobRecord[] {
    return response.jobs.map((job) => mapSlurmJobToRecord(job));
}

export function resolveJobRows(response?: SlurmJobsPayload | null): JobRecord[] {
    if (!response || !Array.isArray(response.jobs) || response.jobs.length === 0) {
        return JOB_FIXTURES;
    }

    return mapSlurmJobsResponseToRecords(response);
}

function jobIdentity(job: SlurmJob) {
    return String(job.job_id ?? 'unknown');
}

export function applySlurmJobsDelta(
    current: SlurmJobsPayload | null,
    delta: SlurmJobsDelta,
): SlurmJobsPayload | null {
    const added = Array.isArray(delta.added) ? delta.added : [];
    const modified = Array.isArray(delta.modified) ? delta.modified : [];
    const deleted = Array.isArray(delta.deleted) ? delta.deleted : [];

    if (!current || !Array.isArray(current.jobs) || current.jobs.length === 0) {
        if (added.length > 0 || modified.length > 0) {
            return { jobs: [...added, ...modified] };
        }

        return current;
    }

    const nextById = new Map(current.jobs.map((job) => [jobIdentity(job), job]));

    for (const job of added) {
        nextById.set(jobIdentity(job), job);
    }

    for (const job of modified) {
        nextById.set(jobIdentity(job), job);
    }

    for (const job of deleted) {
        nextById.delete(jobIdentity(job));
    }

    return { jobs: Array.from(nextById.values()) };
}

export const JOB_FIXTURES: JobRecord[] = [
    {
        jobId: '4821901',
        name: 'analysis-a',
        user: 'alice',
        account: 'research',
        partition: 'compute',
        state: 'RUNNING',
        runtime: '02:14:33',
        nodes: 4,
        cpus: 128,
        submitTime: '2026-06-28T01:12:00Z',
        startTime: '2026-06-28T01:18:10Z',
        endTime: null,
        nodeList: 'node[001-004]',
        qos: 'normal',
        command: 'python train.py --config config.yml',
        workDir: '/home/alice/project',
        stdout: '/scratch/alice/4821901.out',
        stderr: '/scratch/alice/4821901.err',
        environment: { CUDA_VISIBLE_DEVICES: '0,1,2,3', OMP_NUM_THREADS: '8' },
        history: buildHistory('RUNNING', '2026-06-28T01:12:00Z', '2026-06-28T01:18:10Z', null),
    },
    {
        jobId: '4821902',
        name: 'postprocess-b',
        user: 'bob',
        account: 'research',
        partition: 'gpu',
        state: 'PENDING',
        runtime: '00:00:00',
        nodes: 2,
        cpus: 64,
        submitTime: '2026-06-28T04:30:00Z',
        startTime: null,
        endTime: null,
        nodeList: '',
        qos: 'high',
        command: 'bash run_post.sh',
        workDir: '/home/bob/analysis',
        stdout: '/scratch/bob/4821902.out',
        stderr: '/scratch/bob/4821902.err',
        environment: { SLURM_JOB_NAME: 'postprocess-b' },
        history: buildHistory('PENDING', '2026-06-28T04:30:00Z', null, null),
    },
    {
        jobId: '4821888',
        name: 'checkpoint',
        user: 'charlie',
        account: 'ops',
        partition: 'debug',
        state: 'FAILED',
        runtime: '00:45:12',
        nodes: 1,
        cpus: 16,
        submitTime: '2026-06-27T22:10:00Z',
        startTime: '2026-06-27T22:14:07Z',
        endTime: '2026-06-27T22:59:19Z',
        nodeList: 'node012',
        qos: 'normal',
        command: 'python checkpoint.py',
        workDir: '/home/charlie/checkpoints',
        stdout: '/scratch/charlie/4821888.out',
        stderr: '/scratch/charlie/4821888.err',
        environment: { PYTHONUNBUFFERED: '1' },
        history: buildHistory('FAILED', '2026-06-27T22:10:00Z', '2026-06-27T22:14:07Z', '2026-06-27T22:59:19Z'),
    },
    {
        jobId: '4821833',
        name: 'ensembling',
        user: 'alice',
        account: 'research',
        partition: 'compute',
        state: 'COMPLETED',
        runtime: '04:12:08',
        nodes: 8,
        cpus: 256,
        submitTime: '2026-06-27T09:00:00Z',
        startTime: '2026-06-27T09:05:12Z',
        endTime: '2026-06-27T13:17:20Z',
        nodeList: 'node[021-028]',
        qos: 'normal',
        command: 'python ensemble.py',
        workDir: '/home/alice/ensemble',
        stdout: '/scratch/alice/4821833.out',
        stderr: '/scratch/alice/4821833.err',
        environment: { SLURM_ARRAY_TASK_ID: '16' },
        history: buildHistory('COMPLETED', '2026-06-27T09:00:00Z', '2026-06-27T09:05:12Z', '2026-06-27T13:17:20Z'),
    },
    {
        jobId: '4821908',
        name: 'distillation',
        user: 'dan',
        account: 'ml',
        partition: 'gpu',
        state: 'RUNNING',
        runtime: '00:52:19',
        nodes: 2,
        cpus: 48,
        submitTime: '2026-06-28T05:42:00Z',
        startTime: '2026-06-28T05:45:55Z',
        endTime: null,
        nodeList: 'gpu[003-004]',
        qos: 'high',
        command: 'python distill.py --teacher model-a',
        workDir: '/home/dan/distill',
        stdout: '/scratch/dan/4821908.out',
        stderr: '/scratch/dan/4821908.err',
        environment: { NCCL_DEBUG: 'WARN', TOKENIZERS_PARALLELISM: 'false' },
        history: buildHistory('RUNNING', '2026-06-28T05:42:00Z', '2026-06-28T05:45:55Z', null),
    },
    {
        jobId: '4821801',
        name: 'reservation-scan',
        user: 'eve',
        account: 'ops',
        partition: 'debug',
        state: 'CANCELLED',
        runtime: '00:03:14',
        nodes: 1,
        cpus: 4,
        submitTime: '2026-06-27T08:12:00Z',
        startTime: '2026-06-27T08:13:02Z',
        endTime: '2026-06-27T08:16:16Z',
        nodeList: 'node005',
        qos: 'normal',
        command: 'bash scan.sh',
        workDir: '/home/eve/ops',
        stdout: '/scratch/eve/4821801.out',
        stderr: '/scratch/eve/4821801.err',
        environment: { LC_ALL: 'C' },
        history: buildHistory('CANCELLED', '2026-06-27T08:12:00Z', '2026-06-27T08:13:02Z', '2026-06-27T08:16:16Z'),
    },
];
