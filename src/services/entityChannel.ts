import type { BridgeEnvelope } from '../types/bridge';

import {
    addChannelListener,
    createRequestId,
    openBridgeChannel,
    removeChannelListener,
    sendJsonLine,
} from '../lib/cockpit';
import { parseBridgeMessages } from '../lib/cockpit/parser';

type ExtractPayload<TPayload> = (message: BridgeEnvelope) => TPayload | null;

type EntityFetchOptions<TPayload> = {
    entity: string;
    extractPayload: ExtractPayload<TPayload>;
    closedMessage: string;
};

type EntitySubscribeOptions<TDelta> = {
    entity: string;
    extractDelta: (message: BridgeEnvelope) => TDelta | null;
    callback: (event: BridgeEnvelope, delta: TDelta | null) => void;
    shouldHandleMessage?: (message: BridgeEnvelope) => boolean;
    generation?: number;
};

function waitForChannelReady(channel: ReturnType<typeof openBridgeChannel>, onReady: () => void) {
    const readyListener = () => {
        removeChannelListener(channel, 'ready', readyListener);
        onReady();
    };

    addChannelListener(channel, 'ready', readyListener);

    if (channel.ready) {
        removeChannelListener(channel, 'ready', readyListener);
        onReady();
    }

    return () => removeChannelListener(channel, 'ready', readyListener);
}

function defaultShouldHandleMessage(entity: string, message: BridgeEnvelope) {
    if (!message || typeof message !== 'object') {
        return false;
    }

    const record = message as Record<string, unknown>;
    return record.type === 'event' && (record.entity === entity || record.entity === `${entity}s`);
}

function normalizeCloseMessage(event: unknown, fallbackMessage: string) {
    if (event instanceof Error) {
        return event.message;
    }

    if (event && typeof event === 'object' && 'message' in event && typeof (event as { message?: unknown }).message === 'string') {
        return (event as { message: string }).message;
    }

    return fallbackMessage;
}

export async function fetchEntitySnapshot<TPayload>({ entity, extractPayload, closedMessage }: EntityFetchOptions<TPayload>): Promise<TPayload> {
    const channel = openBridgeChannel();

    return new Promise((resolve, reject) => {
        let settled = false;
        let readyCleanup = () => {};

        const cleanup = () => {
            readyCleanup();
            removeChannelListener(channel, 'message', onMessage);
            removeChannelListener(channel, 'close', onClose);
            channel.close();
        };

        const finish = (payload: TPayload) => {
            if (settled) {
                return;
            }

            settled = true;
            cleanup();
            resolve(payload);
        };

        const fail = (message: string) => {
            if (settled) {
                return;
            }

            settled = true;
            cleanup();
            reject(new Error(message));
        };

        const onMessage = (data: unknown) => {
            const messages = parseBridgeMessages(data);
            for (const message of messages) {
                const payload = extractPayload(message);
                if (payload) {
                    finish(payload);
                    return;
                }
            }
        };

        const onClose = (event: unknown) => {
            if (settled) {
                return;
            }

            fail(normalizeCloseMessage(event, closedMessage));
        };

        addChannelListener(channel, 'message', onMessage);
        addChannelListener(channel, 'close', onClose);

        readyCleanup = waitForChannelReady(channel, () => {
            sendJsonLine(channel, {
                request_id: createRequestId(),
                type: 'list',
                entity,
            });
        });
    });
}

export function subscribeEntityUpdates<TDelta>({ entity, extractDelta, callback, shouldHandleMessage, generation = 0 }: EntitySubscribeOptions<TDelta>) {
    const channel = openBridgeChannel();
    const shouldHandle = shouldHandleMessage ?? ((message: BridgeEnvelope) => defaultShouldHandleMessage(entity, message));

    const onMessage = (data: unknown) => {
        const messages = parseBridgeMessages(data);
        for (const message of messages) {
            if (shouldHandle(message)) {
                callback(message, extractDelta(message));
            }
        }
    };

    const readyCleanup = waitForChannelReady(channel, () => {
        sendJsonLine(channel, {
            request_id: createRequestId(),
            type: 'subscribe',
            entity,
            generation,
        });
    });

    addChannelListener(channel, 'message', onMessage);

    return () => {
        readyCleanup();
        removeChannelListener(channel, 'message', onMessage);
        channel.close();
    };
}
