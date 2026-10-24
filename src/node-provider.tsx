import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';

import { createChannel, resolveSocketPath } from './ipc/channel';
import { CommandService, IpcCommandError } from './ipc/command-service';
import { createIpcClient, type IpcClient } from './ipc/client';
import { applyNodeStreamMessage, type NodeStreamState } from './ipc/node-stream';
import {
    type IpcChannel,
    type NodeCapabilities,
    type NodeCommandResponsePayload,
    type NodeCommandStatus,
    type NodeStreamNode,
} from './ipc/types';

export type NodeResource = NodeStreamNode;

export interface NodeCommandDisplayState {
    action: 'DRAIN' | 'RESUME';
    status: 'pending' | NodeCommandStatus;
    commandId?: string;
    error?: string;
}

export interface NodeProviderValue {
    nodes: NodeResource[];
    generation: number;
    loading: boolean;
    connected: boolean;
    reconnecting: boolean;
    error: Error | null;
    capabilities: NodeCapabilities | null;
    capabilitiesLoading: boolean;
    capabilitiesError: Error | null;
    commandStates: Record<string, NodeCommandDisplayState>;
    refresh: () => void;
    refreshCapabilities: () => void;
    drainNode: (nodeName: string, reason: string) => Promise<NodeCommandResponsePayload>;
    resumeNode: (nodeName: string) => Promise<NodeCommandResponsePayload>;
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

function isDrained(node: NodeResource): boolean {
    return node.status.state === 'DRAIN' || node.status.stateFlags?.includes('DRAIN') === true;
}

function commandFailureStatus(error: Error): NodeCommandDisplayState['status'] {
    if (!(error instanceof IpcCommandError)) {
        return 'unknown';
    }
    if (error.code === 'authorization-denied') {
        return 'denied';
    }
    if (error.code === 'command-outcome-unknown' ||
        error.code === 'command-timeout' ||
        error.code === 'ledger-persistence-failed') {
        return 'unknown';
    }
    return 'failed';
}

export const NodeProvider = ({ socketPath, createChannel: makeChannel = createChannel, children }: NodeProviderProps) => {
    const [nodes, setNodes] = useState<NodeResource[]>([]);
    const [generation, setGeneration] = useState(0);
    const [loading, setLoading] = useState(true);
    const [connected, setConnected] = useState(false);
    const [reconnecting, setReconnecting] = useState(false);
    const [error, setError] = useState<Error | null>(null);
    const [capabilities, setCapabilities] = useState<NodeCapabilities | null>(null);
    const [capabilitiesLoading, setCapabilitiesLoading] = useState(true);
    const [capabilitiesError, setCapabilitiesError] = useState<Error | null>(null);
    const [commandStates, setCommandStates] = useState<Record<string, NodeCommandDisplayState>>({});
    const forceReconnect = useRef<(() => void) | null>(null);
    const commandService = useRef<CommandService | null>(null);
    const commandsInFlight = useRef(new Set<string>());
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
        let client: IpcClient | null = null;
        let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
        let snapshotTimer: ReturnType<typeof setTimeout> | null = null;
        let reconnectAttempt = 0;
        let hasSnapshot = false;
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
            const current = client;
            client = null;
            commandService.current = null;
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
            hasSnapshot = false;
            streamState.current = { ...streamState.current, hasSnapshot: false };
            setCapabilities(null);
            setCapabilitiesLoading(true);
            setCapabilitiesError(null);
            setConnected(false);
            setReconnecting(reconnectAttempt > 0);
            try {
                const current = createIpcClient(makeChannel(resolvedSocketPath));
                client = current;
                const service = new CommandService(current);
                commandService.current = service;
                current.onmessage = message => {
                    if (!active || client !== current) {
                        return;
                    }
                    if (message.type === 'command-status' && message.payload && typeof message.payload === 'object') {
                        const payload = message.payload as { commandId?: unknown; status?: unknown };
                        if (typeof payload.commandId === 'string' &&
                            (payload.status === 'running' || payload.status === 'succeeded' ||
                                payload.status === 'failed' || payload.status === 'denied' || payload.status === 'unknown')) {
                            setCommandStates(currentStates => {
                                const entry = Object.entries(currentStates).find(([, state]) => state.commandId === payload.commandId);
                                if (!entry) {
                                    return currentStates;
                                }
                                const [nodeName, state] = entry;
                                return { ...currentStates, [nodeName]: { ...state, status: payload.status as NodeCommandStatus } };
                            });
                        }
                        return;
                    }
                    if (message.type !== 'event') {
                        return;
                    }
                    try {
                        const isSnapshot = message.payload &&
                            typeof message.payload === 'object' &&
                            'event' in message.payload && message.payload.event === 'snapshot';
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
                        if (isSnapshot) {
                            service.requestCapabilities()
                                    .then(value => {
                                        if (active && client === current) {
                                            setCapabilities(value);
                                            setCapabilitiesError(null);
                                        }
                                    })
                                    .catch(cause => {
                                        if (active && client === current) {
                                            setCapabilities(null);
                                            setCapabilitiesError(cause instanceof Error ? cause : new Error(String(cause)));
                                        }
                                    })
                                    .finally(() => {
                                        if (active && client === current) {
                                            setCapabilitiesLoading(false);
                                        }
                                    });
                        }
                    } catch (cause) {
                        closeCurrentChannel(false);
                        setError(cause instanceof Error ? cause : new Error(String(cause)));
                    }
                };
                current.onerror = cause => {
                    if (active && client === current) {
                        setError(cause);
                    }
                };
                current.onclose = () => {
                    if (active && client === current) {
                        client = null;
                        commandService.current = null;
                        scheduleReconnect();
                    }
                };
                snapshotTimer = setTimeout(() => {
                    if (active && client === current && !hasSnapshot) {
                        closeCurrentChannel(false);
                        setError(new Error('Node stream snapshot timed out'));
                    }
                }, SNAPSHOT_TIMEOUT_MS);
            } catch (cause) {
                client = null;
                commandService.current = null;
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
            if (client) {
                client.onmessage = null;
                client.onerror = null;
                client.onclose = null;
                client.close();
                client = null;
            }
        };
    }, [makeChannel, socketPath]);

    const refreshCapabilities = useCallback(() => {
        const currentService = commandService.current;
        if (!currentService) {
            setCapabilities(null);
            setCapabilitiesLoading(false);
            setCapabilitiesError(new Error('IPC connection is not ready'));
            return;
        }
        setCapabilitiesLoading(true);
        setCapabilitiesError(null);
        currentService.requestCapabilities()
                .then(value => {
                    if (commandService.current === currentService) {
                        setCapabilities(value);
                    }
                })
                .catch(cause => {
                    if (commandService.current === currentService) {
                        setCapabilities(null);
                        setCapabilitiesError(cause instanceof Error ? cause : new Error(String(cause)));
                    }
                })
                .finally(() => {
                    if (commandService.current === currentService) {
                        setCapabilitiesLoading(false);
                    }
                });
    }, []);

    const submitCommand = useCallback(async (
        nodeName: string,
        action: 'DRAIN' | 'RESUME',
        reason?: string,
    ): Promise<NodeCommandResponsePayload> => {
        const currentService = commandService.current;
        if (!connected || !currentService || !capabilities || capabilitiesLoading) {
            throw new Error('Node commands are unavailable until connection capabilities are loaded');
        }
        const capability = action === 'DRAIN' ? capabilities['nodes.drain'] : capabilities['nodes.resume'];
        if (!capability) {
            throw new IpcCommandError({
                code: 'authorization-denied',
                category: 'authorization-error',
                message: `The current user cannot ${action.toLowerCase()} nodes`,
                retryable: false,
            });
        }
        const node = nodes.find(item => item.spec.nodeName === nodeName);
        if (!node) {
            throw new Error(`Node ${nodeName} is not in the current authoritative snapshot`);
        }
        const state = node.status.state;
        const applicable = action === 'DRAIN'
            ? state === 'IDLE' && !isDrained(node)
            : isDrained(node);
        if (!applicable) {
            throw new Error(`${action} is not applicable to the latest observed state of ${nodeName}`);
        }
        const existing = commandStates[nodeName];
        if (commandsInFlight.current.has(nodeName) || existing?.status === 'pending' ||
            existing?.status === 'running' || existing?.status === 'unknown') {
            throw new Error(`A command for ${nodeName} is already pending or has an unknown outcome`);
        }

        commandsInFlight.current.add(nodeName);
        setCommandStates(current => ({ ...current, [nodeName]: { action, status: 'pending' } }));
        try {
            const parameters = action === 'DRAIN'
                ? reason === undefined ? { state: action } : { state: action, reason }
                : { state: action };
            const result = await currentService.updateNode(
                nodeName,
                parameters,
            );
            setCommandStates(current => ({
                ...current,
                [nodeName]: { action, status: result.status, commandId: result.commandId },
            }));
            return result;
        } catch (cause) {
            const failure = cause instanceof Error ? cause : new Error(String(cause));
            const failedState: NodeCommandDisplayState = {
                action,
                status: commandFailureStatus(failure),
                error: failure.message,
            };
            if (cause instanceof IpcCommandError && cause.commandId !== undefined) {
                failedState.commandId = cause.commandId;
            }
            setCommandStates(current => ({
                ...current,
                [nodeName]: failedState,
            }));
            throw failure;
        } finally {
            commandsInFlight.current.delete(nodeName);
        }
    }, [capabilities, capabilitiesLoading, commandStates, connected, nodes]);

    const drainNode = useCallback((nodeName: string, reason: string) => {
        return submitCommand(nodeName, 'DRAIN', reason);
    }, [submitCommand]);

    const resumeNode = useCallback((nodeName: string) => {
        return submitCommand(nodeName, 'RESUME');
    }, [submitCommand]);

    const value = useMemo(() => ({
        nodes,
        generation,
        loading,
        connected,
        reconnecting,
        error,
        capabilities,
        capabilitiesLoading,
        capabilitiesError,
        commandStates,
        refresh,
        refreshCapabilities,
        drainNode,
        resumeNode,
    }), [nodes, generation, loading, connected, reconnecting, error, capabilities,
        capabilitiesLoading, capabilitiesError, commandStates, refresh, refreshCapabilities,
        drainNode, resumeNode]);
    return <NodeContext.Provider value={value}>{children}</NodeContext.Provider>;
};

export function useNodes(): NodeProviderValue {
    const value = useContext(NodeContext);
    if (!value) {
        throw new Error('useNodes must be used inside NodeProvider');
    }
    return value;
}
