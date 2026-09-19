import { type IpcChannel, type MessageHandler } from './types.ts';

export function resolveSocketPath(socketPath?: string): string {
  const windowSocket = typeof window !== 'undefined'
    ? (window as Window & { COCKPIT_SLURM_SOCKET_PATH?: string }).COCKPIT_SLURM_SOCKET_PATH
    : undefined;
  const configured = socketPath ?? windowSocket ?? undefined;
  if (configured && configured.trim() !== '') {
    return configured;
  }

  return '/run/cockpit-slurm/cockpit-slurm.sock';
}

export function createChannel(socketPath?: string): IpcChannel {
  const resolved = resolveSocketPath(socketPath);
  const channel = cockpit.channel({
    payload: 'stream',
    unix: resolved,
    binary: true,
  });

  const handlers: { onmessage: MessageHandler | null; onerror: ((error: Error) => void) | null; onclose: (() => void) | null } = {
    onmessage: null,
    onerror: null,
    onclose: null,
  };

  const notifyMessage = (message: unknown) => {
    const data = message instanceof ArrayBuffer ? message : message instanceof Uint8Array ? message : new Uint8Array();
    if (handlers.onmessage) {
      handlers.onmessage(data);
    }
  };

  channel.addEventListener('message', (_event: unknown, message: unknown) => {
    notifyMessage(message);
  });

  channel.addEventListener('close', (_event: unknown, details: unknown) => {
    if (handlers.onerror && details && typeof details === 'object' && 'problem' in details) {
      const problem = (details as { problem?: unknown }).problem;
      if (problem) {
        handlers.onerror(new Error(String(problem)));
      }
    }
    if (handlers.onclose) {
      handlers.onclose();
    }
  });

  return {
    send(data: Uint8Array | ArrayBuffer | number[]) {
      channel.send(data);
    },
    close() {
      channel.close();
    },
    get onmessage() {
      return handlers.onmessage;
    },
    set onmessage(handler: MessageHandler | null) {
      handlers.onmessage = handler;
    },
    get onerror() {
      return handlers.onerror;
    },
    set onerror(handler: ((error: Error) => void) | null) {
      handlers.onerror = handler;
    },
    get onclose() {
      return handlers.onclose;
    },
    set onclose(handler: (() => void) | null) {
      handlers.onclose = handler;
    },
  };
}
