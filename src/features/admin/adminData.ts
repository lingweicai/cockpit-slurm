export type UserState = 'ACTIVE' | 'PENDING' | 'LOCKED';

export type AccountState = 'ACTIVE' | 'LIMITED';

export type QosState = 'ENABLED' | 'PAUSED';

export type ReservationState = 'ACTIVE' | 'UPCOMING' | 'EXPIRED';

export interface UserRecord {
    name: string;
    role: 'admin' | 'operator' | 'user';
    accounts: string[];
    state: UserState;
    lastLogin: string;
    mfaEnabled: boolean;
    defaultQos: string;
}

export interface AccountRecord {
    name: string;
    parent: string;
    users: number;
    cpuHoursUsed: number;
    cpuHoursLimit: number;
    state: AccountState;
}

export interface QosRecord {
    name: string;
    priority: number;
    maxJobs: number;
    maxNodes: number;
    maxWallTime: string;
    preemptMode: string;
    state: QosState;
}

export interface ReservationRecord {
    name: string;
    state: ReservationState;
    startTime: string;
    endTime: string;
    nodes: string;
    accounts: string[];
    users: string[];
    purpose: string;
}

export interface ReportRecord {
    title: string;
    value: number;
    delta: string;
    description: string;
}

export const USER_FIXTURES: UserRecord[] = [
    {
        name: 'alice',
        role: 'admin',
        accounts: ['ops', 'research'],
        state: 'ACTIVE',
        lastLogin: '2026-06-28T16:25:00Z',
        mfaEnabled: true,
        defaultQos: 'high',
    },
    {
        name: 'bob',
        role: 'operator',
        accounts: ['ops'],
        state: 'ACTIVE',
        lastLogin: '2026-06-28T15:40:00Z',
        mfaEnabled: true,
        defaultQos: 'normal',
    },
    {
        name: 'carol',
        role: 'user',
        accounts: ['research'],
        state: 'PENDING',
        lastLogin: '2026-06-25T09:10:00Z',
        mfaEnabled: false,
        defaultQos: 'debug',
    },
    {
        name: 'dan',
        role: 'user',
        accounts: ['ml'],
        state: 'LOCKED',
        lastLogin: '2026-06-20T08:00:00Z',
        mfaEnabled: false,
        defaultQos: 'gpu',
    },
];

export const ACCOUNT_FIXTURES: AccountRecord[] = [
    {
        name: 'ops',
        parent: 'root',
        users: 2,
        cpuHoursUsed: 12800,
        cpuHoursLimit: 20000,
        state: 'ACTIVE',
    },
    {
        name: 'research',
        parent: 'root',
        users: 7,
        cpuHoursUsed: 64200,
        cpuHoursLimit: 80000,
        state: 'ACTIVE',
    },
    {
        name: 'ml',
        parent: 'research',
        users: 4,
        cpuHoursUsed: 91000,
        cpuHoursLimit: 95000,
        state: 'LIMITED',
    },
];

export const QOS_FIXTURES: QosRecord[] = [
    {
        name: 'normal',
        priority: 100,
        maxJobs: 64,
        maxNodes: 128,
        maxWallTime: '24:00:00',
        preemptMode: 'OFF',
        state: 'ENABLED',
    },
    {
        name: 'high',
        priority: 300,
        maxJobs: 16,
        maxNodes: 64,
        maxWallTime: '08:00:00',
        preemptMode: 'SUSPEND',
        state: 'ENABLED',
    },
    {
        name: 'gpu',
        priority: 250,
        maxJobs: 12,
        maxNodes: 32,
        maxWallTime: '12:00:00',
        preemptMode: 'REQUEUE',
        state: 'ENABLED',
    },
    {
        name: 'debug',
        priority: 50,
        maxJobs: 2,
        maxNodes: 8,
        maxWallTime: '00:30:00',
        preemptMode: 'OFF',
        state: 'PAUSED',
    },
];

export const RESERVATION_FIXTURES: ReservationRecord[] = [
    {
        name: 'maint-window-01',
        state: 'ACTIVE',
        startTime: '2026-06-28T00:00:00Z',
        endTime: '2026-06-28T12:00:00Z',
        nodes: 'node[001-004]',
        accounts: ['ops'],
        users: ['alice', 'bob'],
        purpose: 'Infrastructure maintenance checks',
    },
    {
        name: 'gpu-sprint',
        state: 'UPCOMING',
        startTime: '2026-06-29T02:00:00Z',
        endTime: '2026-06-29T08:00:00Z',
        nodes: 'gpu[001-008]',
        accounts: ['ml'],
        users: ['dan'],
        purpose: 'Model benchmarking sprint',
    },
    {
        name: 'debug-hold',
        state: 'EXPIRED',
        startTime: '2026-06-26T04:00:00Z',
        endTime: '2026-06-26T06:00:00Z',
        nodes: 'node010',
        accounts: ['research'],
        users: ['alice'],
        purpose: 'Debug window for short runs',
    },
    {
        name: 'review-room',
        state: 'UPCOMING',
        startTime: '2026-06-30T01:00:00Z',
        endTime: '2026-06-30T03:00:00Z',
        nodes: 'node[020-024]',
        accounts: ['ops', 'research'],
        users: ['alice', 'carol'],
        purpose: 'Shared review and validation window',
    },
];

export const REPORT_FIXTURES: ReportRecord[] = [
    {
        title: 'Cluster utilization',
        value: 78,
        delta: '+4%',
        description: 'Average CPU utilization across the last 24 hours.',
    },
    {
        title: 'Queue depth',
        value: 42,
        delta: '-6%',
        description: 'Jobs waiting for resources right now.',
    },
    {
        title: 'Failed jobs',
        value: 3,
        delta: '-1',
        description: 'Failures recorded since the last report refresh.',
    },
    {
        title: 'Active users',
        value: 11,
        delta: '+2',
        description: 'Distinct users running jobs or reservations.',
    },
];

export const ADMIN_SETTINGS = {
    maintenanceMode: false,
    defaultAccount: 'research',
    notificationEmail: 'slurm-admins@example.org',
    retentionDays: '30',
    enableUsageReports: true,
};
