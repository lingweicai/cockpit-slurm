import { IpcClient } from './client.ts';
import { decodeFrameChunk, encodeFrame } from './framing.ts';
import { type ApplicationEnvelope, type IpcChannel, type MessageHandler } from './types.ts';

function assertEqual<T>(actual: T, expected: T, message: string): void {
  if (actual !== expected) {
    throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
  }
}

async function assertRejects(promise: Promise<unknown>, expectedMessage: string): Promise<void> {
  try {
    await promise;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (message.includes(expectedMessage)) {
      return;
    }
    throw new Error(`expected rejection containing ${expectedMessage}, got ${message}`);
  }
  throw new Error(`expected rejection containing ${expectedMessage}`);
}

class FakeChannel implements IpcChannel {
  readonly sent: Uint8Array[] = [];
  closed = false;
  onmessage: MessageHandler | null = null;
  onerror: ((error: Error) => void) | null = null;
  onclose: (() => void) | null = null;

  send(data: Uint8Array | ArrayBuffer | number[]): void {
    this.sent.push(new Uint8Array(data));
  }

  close(): void {
    this.closed = true;
  }

  respond(message: ApplicationEnvelope): void {
    this.onmessage?.(encodeFrame(message));
  }

  fail(error: Error): void {
    this.onerror?.(error);
  }

  disconnect(): void {
    this.closed = true;
    this.onclose?.();
  }
}

function response(messageId: string): ApplicationEnvelope {
  return {
    protocol: 'cockpit-slurm',
    version: '1.0',
    messageId,
    type: 'pong',
    payload: { status: 'ok' },
  };
}

async function run(): Promise<void> {
  const firstChannel = new FakeChannel();
  const firstClient = new IpcClient(firstChannel);
  const firstRequest = firstClient.send({ type: 'ping', messageId: 'REQUEST-1' });
  const secondRequest = firstClient.send({ type: 'ping', messageId: 'REQUEST-2' });
  assertEqual(firstChannel.sent.length, 2, 'client should send each request');
  assertEqual(decodeFrameChunk(firstChannel.sent[0])[0].messageId, 'REQUEST-1', 'client should preserve request message IDs');

  firstChannel.respond(response('REQUEST-2'));
  assertEqual((await secondRequest).messageId, 'REQUEST-2', 'client should correlate an out-of-order response');
  firstChannel.respond(response('REQUEST-1'));
  assertEqual((await firstRequest).messageId, 'REQUEST-1', 'client should resolve the matching pending response');

  const errorChannel = new FakeChannel();
  const errorClient = new IpcClient(errorChannel);
  const transportFailure = errorClient.send({ type: 'ping' });
  errorChannel.fail(new Error('transport unavailable'));
  await assertRejects(transportFailure, 'transport unavailable');

  const closedChannel = new FakeChannel();
  const closedClient = new IpcClient(closedChannel);
  const pendingRequest = closedClient.send({ type: 'ping' });
  closedChannel.disconnect();
  await assertRejects(pendingRequest, 'connection closed');
  await assertRejects(closedClient.send({ type: 'ping' }), 'client is closed');

  const secondChannel = new FakeChannel();
  const secondClient = new IpcClient(secondChannel);
  const independentRequest = secondClient.send({ type: 'ping', messageId: 'REQUEST-3' });
  secondChannel.respond(response('REQUEST-3'));
  assertEqual((await independentRequest).messageId, 'REQUEST-3', 'separate clients should keep independent request state');

  console.log('ipc client checks passed');
}

run().catch(error => {
  throw error;
});