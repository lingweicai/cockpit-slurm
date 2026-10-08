import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';

import { createChannel, resolveSocketPath } from './ipc/channel';
import { FrameDecoder } from './ipc/framing';
import { applyNodeStreamMessage, type NodeStreamState } from './ipc/node-stream';
import { type IpcChannel, type NodeStreamNode } from './ipc/types';

export type NodeResource = NodeStreamNode;

export interface NodeProviderValue {
    nodes: NodeResource[];
    generation: number;
    loading: boolean;
    connected: boolean;
    reconnecting: boolean;
    error: Error | null;
    refresh: () => void;
}

interface NodeProviderProps {
    socketPath?: string;
    createChannel?: (socketPath: string) => IpcChannel;
    children: React.ReactNode;
}

const NodeContext = createContext<NodeProviderValue | null>(null);

const INITIAL_RECONNECT_DELAY_MS = 500;
const MAX_RECONNECT_DELAY_MS = 10_000;
const SNAPSHOT_TIMEOUT_MS = 10_000;

export const NodeProvider = ({ socketPath, createChannel: makeChannel = createChannel, children }: NodeProviderProps) => {
    const [nodes, setNodes] = useState<NodeResource[]>([]);
    const [generation, setGeneration] = useState(0);
    const [loading, setLoading] = useState(true);
    const [connected, setConnected] = useState(false);
    const [reconnecting, setReconnecting] = useState(false);
    const [error, setError] = useState<Error | null>(null);
    const forceReconnect = useRef<(() => void) | null>(null);
    const streamState = useRef<NodeStreamState>({
        resource: 'nodes',
        generation: 0,
        nodes: [],
        hasSnapshot: false,
    });

    const refresh = useCallback(() => {
        setError(null);
        forceReconnect.current?.();
    }, []);

    useEffect(() => {
        let active = true;
        let channel: IpcChannel | null = null;
        let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
        let snapshotTimer: ReturnType<typeof setTimeout> | null = null;
        let reconnectAttempt = 0;
        let hasSnapshot = false;
        let decoder: FrameDecoder | null = null;
        const resolvedSocketPath = resolveSocketPath(socketPath);

        const clearSnapshotTimer = () => {
            if (snapshotTimer !== null) {
                clearTimeout(snapshotTimer);
                snapshotTimer = null;
            }
        };

        const clearReconnectTimer = () => {
            if (reconnectTimer !== null) {
                clearTimeout(reconnectTimer);
                reconnectTimer = null;
            }
        };

        const scheduleReconnect = (cause?: Error, immediate = false) => {
            if (!active || reconnectTimer !== null) {
                return;
            }
            if (cause) {
                setError(cause);
            }
            setConnected(false);
            setReconnecting(true);
            clearSnapshotTimer();
            const delay = immediate
                ? 0
                : Math.min(INITIAL_RECONNECT_DELAY_MS * (2 ** reconnectAttempt), MAX_RECONNECT_DELAY_MS);
            reconnectAttempt++;
            reconnectTimer = setTimeout(() => {
                reconnectTimer = null;
                openChannel();
            }, delay);
        };

        const closeCurrentChannel = (immediate = true) => {
            clearSnapshotTimer();
            const current = channel;
            channel = null;
            if (current) {
                current.onmessage = null;
                current.onerror = null;
                current.onclose = null;
                current.close();
            }
            scheduleReconnect(undefined, immediate);
        };

        const openChannel = () => {
            if (!active) {
                return;
            }
            decoder = new FrameDecoder();
            hasSnapshot = false;
            streamState.current = { ...streamState.current, hasSnapshot: false };
            setConnected(false);
            setReconnecting(reconnectAttempt > 0);
            try {
                const current = makeChannel(resolvedSocketPath);
                channel = current;
                current.onmessage = chunk => {
                    if (!active || channel !== current || !decoder) {
                        return;
                    }
                    try {
                        const messages = decoder.push(chunk);
                        for (const message of messages) {
                            const next = applyNodeStreamMessage(streamState.current, message);
                            streamState.current = next;
                            hasSnapshot = next.hasSnapshot;
                            setNodes(next.nodes);
                            setGeneration(next.generation);
                            if (next.hasSnapshot) {
                                clearSnapshotTimer();
                                reconnectAttempt = 0;
                                setLoading(false);
                                setConnected(true);
                                setReconnecting(false);
                                setError(null);
                            }
                        }
                    } catch (cause) {
                        closeCurrentChannel(false);
                        setError(cause instanceof Error ? cause : new Error(String(cause)));
                    }
                };
                current.onerror = cause => {
                    if (active && channel === current) {
                        setError(cause);
                    }
                };
                current.onclose = () => {
                    if (active && channel === current) {
                        channel = null;
                        scheduleReconnect();
                    }
                };
                snapshotTimer = setTimeout(() => {
                    if (active && channel === current && !hasSnapshot) {
                        closeCurrentChannel(false);
                        setError(new Error('Node stream snapshot timed out'));
                    }
                }, SNAPSHOT_TIMEOUT_MS);
            } catch (cause) {
                channel = null;
                scheduleReconnect(cause instanceof Error ? cause : new Error(String(cause)));
            }
        };

        forceReconnect.current = () => {
            reconnectAttempt = 0;
            clearReconnectTimer();
            closeCurrentChannel(true);
        };
        openChannel();

        return () => {
            active = false;
            forceReconnect.current = null;
            clearReconnectTimer();
            clearSnapshotTimer();
            if (channel) {
                channel.onmessage = null;
                channel.onerror = null;
                channel.onclose = null;
                channel.close();
                channel = null;
            }
        };
    }, [makeChannel, socketPath]);

    const value = useMemo(() => ({
        nodes,
        generation,
        loading,
        connected,
        reconnecting,
        error,
        refresh,
    }), [nodes, generation, loading, connected, reconnecting, error, refresh]);
    return <NodeContext.Provider value={value}>{children}</NodeContext.Provider>;
};

export function useNodes(): NodeProviderValue {
    const value = useContext(NodeContext);
    if (!value) {
        throw new Error('useNodes must be used inside NodeProvider');
    }
    return value;
}
