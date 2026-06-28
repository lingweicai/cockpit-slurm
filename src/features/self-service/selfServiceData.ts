export type ReservationState = 'ACTIVE' | 'UPCOMING' | 'EXPIRED';

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

export interface FileRecord {
    path: string;
    kind: 'script' | 'data' | 'output' | 'log';
    size: string;
    modifiedAt: string;
}

export const RESERVATIONS_FIXTURES: ReservationRecord[] = [
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
];

export const FILES_FIXTURES: FileRecord[] = [
    { path: '/home/alice/project/train.py', kind: 'script', size: '24 KB', modifiedAt: '2026-06-27T14:12:00Z' },
    { path: '/home/alice/project/config.yml', kind: 'data', size: '3 KB', modifiedAt: '2026-06-27T14:08:00Z' },
    { path: '/scratch/alice/4821901.out', kind: 'output', size: '128 MB', modifiedAt: '2026-06-28T03:30:00Z' },
    { path: '/scratch/alice/4821833.err', kind: 'log', size: '12 KB', modifiedAt: '2026-06-27T13:20:00Z' },
];
