import { createChannel, resolveSocketPath } from './channel.ts';
import { type IpcChannel } from './types.ts';

function assertEqual<T>(actual: T, expected: T, message: string): void {
    if (actual !== expected) {
        throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
    }
}

interface CockpitChannelOptions {
  payload: string;
  unix: string;
  binary: boolean;
}

interface CockpitTestChannel {
  addEventListener(event: string, handler: (event: unknown, data: unknown) => void): void;
  close(): void;
  send(data: Uint8Array | ArrayBuffer | number[]): void;
}

type ChannelListener = (event: unknown, data: unknown) => void;

const listeners = new Map<string, ChannelListener>();
const sent: Array<Uint8Array | ArrayBuffer | number[]> = [];
let closed = false;
let options: CockpitChannelOptions | undefined;

const cockpitChannel: CockpitTestChannel = {
    addEventListener(event, handler) {
        listeners.set(event, handler);
    },
    close() {
        closed = true;
    },
    send(data) {
        sent.push(data);
    },
};

Object.defineProperty(globalThis, 'cockpit', {
    configurable: true,
    value: {
        channel(channelOptions: CockpitChannelOptions) {
            options = channelOptions;
            return cockpitChannel;
        },
    },
});

function run(): void {
    assertEqual(resolveSocketPath('/custom/socket'), '/custom/socket', 'explicit socket path should win');
    assertEqual(resolveSocketPath(), '/run/cockpit-slurm/cockpit-slurm.sock', 'default socket path should be centralized');

    const channel: IpcChannel = createChannel('/run/cockpit-slurm/test.sock');
    if (!options) {
        throw new Error('cockpit.channel was not called');
    }
    assertEqual(options.payload, 'stream', 'native channel payload');
    assertEqual(options.unix, '/run/cockpit-slurm/test.sock', 'native channel Unix socket');
    assertEqual(options.binary, true, 'native channel binary mode');

    const bytes = new Uint8Array([1, 2, 3]);
    let received: Uint8Array | ArrayBuffer | null = null;
    let closeError: Error | null = null;
    let closeCount = 0;
    channel.onmessage = data => {
        received = data;
    };
    channel.onerror = error => {
        closeError = error;
    };
    channel.onclose = () => {
        closeCount++;
    };

    listeners.get('message')?.({}, bytes);
    assertEqual(received, bytes, 'native channel binary message should be forwarded');

    listeners.get('close')?.({}, { problem: 'not-authorized' });
    assertEqual(closeError?.message, 'not-authorized', 'native channel close problem should be surfaced');
    assertEqual(closeCount, 1, 'native channel close should be forwarded');

    channel.send(bytes);
    assertEqual(sent.length, 1, 'outbound data should be forwarded');
    channel.close();
    assertEqual(closed, true, 'closing adapter should close native channel');

    console.log('native cockpit channel checks passed');
}

run();
