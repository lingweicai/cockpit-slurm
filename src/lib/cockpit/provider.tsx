import React, { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';

import type { BridgeConnectionStatus } from '../../types/bridge';
import type { CockpitChannelLike } from '../../types/cockpit';

import { addChannelListener, openBridgeChannel, removeChannelListener } from './channel';

type ChannelProviderProps = {
  children: ReactNode;
  createChannel?: () => CockpitChannelLike;
};

type ChannelContextValue = {
  channel: CockpitChannelLike | null;
  status: BridgeConnectionStatus;
  error: string | null;
  lastConnectedAt: string | null;
  lastMessageAt: string | null;
  reconnect: () => void;
};

const ChannelContext = createContext<ChannelContextValue | null>(null);

function formatChannelError(error: unknown): string {
    if (error instanceof Error) {
        return error.message;
    }

    if (typeof error === 'string') {
        return error;
    }

    return 'Cockpit channel error';
}

export function ChannelProvider({ children, createChannel = openBridgeChannel }: ChannelProviderProps) {
    const [status, setStatus] = useState<BridgeConnectionStatus>('connecting');
    const [error, setError] = useState<string | null>(null);
    const [lastConnectedAt, setLastConnectedAt] = useState<string | null>(null);
    const [lastMessageAt, setLastMessageAt] = useState<string | null>(null);
    const [reconnectCount, setReconnectCount] = useState(0);
    const channelRef = useRef<CockpitChannelLike | null>(null);

    useEffect(() => {
        let active = true;

        setStatus('connecting');
        setError(null);

        let channel: CockpitChannelLike;

        try {
            channel = createChannel();
        } catch (err) {
            setStatus('error');
            setError(formatChannelError(err));
            return () => {
                active = false;
            };
        }

        channelRef.current = channel;

        const markConnected = () => {
            if (!active) {
                return;
            }

            setStatus('ready');
            setError(null);
            setLastConnectedAt((current) => current ?? new Date().toISOString());
            setLastMessageAt(new Date().toISOString());
        };

        const markMessage = () => {
            if (!active) {
                return;
            }

            setLastMessageAt(new Date().toISOString());
            setStatus((current) => (current === 'error' ? 'connecting' : 'ready'));
        };

        const markClose = (event: unknown) => {
            if (!active) {
                return;
            }

            setStatus('closed');
            if (event instanceof Error) {
                setError(event.message);
            } else if (event && typeof event === 'object' && 'message' in event && typeof (event as { message?: unknown }).message === 'string') {
                setError((event as { message: string }).message);
            }
        };

        const markError = (event: unknown) => {
            if (!active) {
                return;
            }

            setStatus('error');
            setError(formatChannelError(event));
        };

        addChannelListener(channel, 'message', markMessage);
        addChannelListener(channel, 'ready', markConnected);
        addChannelListener(channel, 'close', markClose);
        addChannelListener(channel, 'error', markError);

        if (channel.ready) {
            markConnected();
        }

        return () => {
            active = false;
            removeChannelListener(channel, 'message', markMessage);
            removeChannelListener(channel, 'ready', markConnected);
            removeChannelListener(channel, 'close', markClose);
            removeChannelListener(channel, 'error', markError);
            channel.close();
            if (channelRef.current === channel) {
                channelRef.current = null;
            }
        };
    }, [createChannel, reconnectCount]);

    const value = useMemo<ChannelContextValue>(() => ({
        channel: channelRef.current,
        status,
        error,
        lastConnectedAt,
        lastMessageAt,
        reconnect: () => setReconnectCount((current) => current + 1),
    }), [error, lastConnectedAt, lastMessageAt, status]);

    return <ChannelContext.Provider value={value}>{children}</ChannelContext.Provider>;
}

export function useChannelContext(): ChannelContextValue {
    const context = useContext(ChannelContext);
    if (!context) {
        throw new Error('useChannelContext must be used within a ChannelProvider');
    }

    return context;
}
