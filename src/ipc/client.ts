import { createChannel } from './channel.ts';
import { encodeFrame, FrameDecoder, validateEnvelope } from './framing.ts';
import { PROTOCOL_NAME, PROTOCOL_VERSION, type ApplicationEnvelope, type IpcChannel } from './types.ts';

function generateMessageId(): string {
  return `MSG-${Date.now()}-${Math.random().toString(16).slice(2, 10)}`;
}

export class IpcClient {
  private readonly channel: IpcChannel;
  private readonly decoder = new FrameDecoder();
  private readonly pending = new Map<string, { resolve: (value: ApplicationEnvelope) => void; reject: (reason: Error) => void }>();
  private closed = false;

  constructor(channel: IpcChannel = createChannel()) {
    this.channel = channel;
    this.channel.onmessage = (chunk) => {
      try {
        for (const frame of this.decoder.push(chunk)) {
          const pending = this.pending.get(frame.messageId);
          if (!pending) {
            continue;
          }
          this.pending.delete(frame.messageId);
          pending.resolve(frame);
        }
      } catch (error) {
        this.rejectAll(error instanceof Error ? error : new Error(String(error)));
      }
    };

    this.channel.onerror = (error) => {
      this.rejectAll(error);
    };

    this.channel.onclose = () => {
      this.closed = true;
      this.rejectAll(new Error('connection closed'));
    };
  }

  send(request: Partial<ApplicationEnvelope> & { type: string }): Promise<ApplicationEnvelope> {
    if (this.closed) {
      return Promise.reject(new Error('client is closed'));
    }

    const message: ApplicationEnvelope = {
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

    return new Promise<ApplicationEnvelope>((resolve, reject) => {
      this.pending.set(message.messageId, { resolve, reject });
      try {
        this.channel.send(encodeFrame(message));
      } catch (error) {
        this.pending.delete(message.messageId);
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
      request.reject(error);
    }
  }
}

export function createIpcClient(channel?: IpcChannel): IpcClient {
  return new IpcClient(channel ?? createChannel());
}
