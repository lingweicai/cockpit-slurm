import { createChannel } from './channel.ts';
import { encodeFrame, FrameDecoder, validateEnvelope } from './framing.ts';
import { PROTOCOL_NAME, PROTOCOL_VERSION, type Envelope, type IpcChannel, type IpcSendOptions } from './types.ts';

interface PendingRequest {
  resolve: (value: Envelope) => void;
  reject: (reason: Error) => void;
  timer?: ReturnType<typeof setTimeout>;
}

function generateMessageId(): string {
    return `MSG-${Date.now()}-${Math.random().toString(16)
            .slice(2, 10)}`;
}

export class IpcClient {
    private readonly channel: IpcChannel;
    private readonly decoder = new FrameDecoder();
    private readonly pending = new Map<string, PendingRequest>();
    private closed = false;
    onmessage: ((message: Envelope) => void) | null = null;
    onerror: ((error: Error) => void) | null = null;
    onclose: (() => void) | null = null;

    constructor(channel: IpcChannel = createChannel()) {
        this.channel = channel;
        this.channel.onmessage = (chunk) => {
            try {
                for (const frame of this.decoder.push(chunk)) {
                    const pending = this.pending.get(frame.messageId);
                    if (!pending) {
                        this.onmessage?.(frame);
                        continue;
                    }
                    this.pending.delete(frame.messageId);
                    if (pending.timer !== undefined) {
                        clearTimeout(pending.timer);
                    }
                    pending.resolve(frame);
                }
            } catch (error) {
                this.rejectAll(error instanceof Error ? error : new Error(String(error)));
            }
        };

        this.channel.onerror = (error) => {
            this.rejectAll(error);
            this.onerror?.(error);
        };

        this.channel.onclose = () => {
            this.closed = true;
            this.rejectAll(new Error('connection closed'));
            this.onclose?.();
        };
    }

    send(request: Partial<Envelope> & { type: string }, options: IpcSendOptions = {}): Promise<Envelope> {
        if (this.closed) {
            return Promise.reject(new Error('client is closed'));
        }

        const message: Envelope = {
            protocol: request.protocol ?? PROTOCOL_NAME,
            version: request.version ?? PROTOCOL_VERSION,
            messageId: request.messageId ?? generateMessageId(),
            type: request.type,
            timestamp: request.timestamp ?? new Date().toISOString(),
            payload: request.payload ?? null,
        };

        if (!validateEnvelope(message)) {
            return Promise.reject(new Error('invalid request envelope'));
        }

        return new Promise<Envelope>((resolve, reject) => {
            const pending: PendingRequest = { resolve, reject };
            if (options.timeoutMs !== undefined && options.timeoutMs > 0) {
                pending.timer = setTimeout(() => {
                    if (!this.pending.delete(message.messageId)) {
                        return;
                    }
                    reject(new Error(`request ${message.messageId} timed out`));
                }, options.timeoutMs);
            }
            this.pending.set(message.messageId, pending);
            try {
                this.channel.send(encodeFrame(message));
            } catch (error) {
                this.pending.delete(message.messageId);
                if (pending.timer !== undefined) {
                    clearTimeout(pending.timer);
                }
                reject(error instanceof Error ? error : new Error(String(error)));
            }
        });
    }

    close(): void {
        if (this.closed) {
            return;
        }
        this.closed = true;
        this.channel.close();
        this.rejectAll(new Error('connection closed'));
    }

    private rejectAll(error: Error): void {
        if (this.pending.size === 0) {
            return;
        }

        const pending = [...this.pending.values()];
        this.pending.clear();
        for (const request of pending) {
            if (request.timer !== undefined) {
                clearTimeout(request.timer);
            }
            request.reject(error);
        }
    }
}

export function createIpcClient(channel?: IpcChannel): IpcClient {
    return new IpcClient(channel ?? createChannel());
}
