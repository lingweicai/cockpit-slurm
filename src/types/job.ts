export type JobState = 'RUNNING' | 'PENDING' | 'FAILED' | 'COMPLETED' | 'CANCELLED';

export interface JobHistoryEntry {
    timestamp: string;
    event: string;
    detail: string;
}

export interface JobRecord {
    jobId: string;
    name: string;
    user: string;
    account: string;
    partition: string;
    state: JobState;
    runtime: string;
    nodes: number;
    cpus: number;
    submitTime: string;
    startTime: string | null;
    endTime: string | null;
    nodeList: string;
    qos: string;
    command: string;
    workDir: string;
    stdout: string;
    stderr: string;
    environment: Record<string, string>;
    history: JobHistoryEntry[];
}
