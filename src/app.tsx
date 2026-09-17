/*
 * SPDX-License-Identifier: LGPL-2.1-or-later
 *
 * Copyright (C) 2017 Red Hat, Inc.
 */

import React, { useCallback, useEffect, useState } from 'react';
import { Alert } from "@patternfly/react-core/dist/esm/components/Alert/index.js";
import { Button } from '@patternfly/react-core/dist/esm/components/Button/index.js';
import { Card, CardBody, CardTitle } from "@patternfly/react-core/dist/esm/components/Card/index.js";
import { Page } from '@patternfly/react-core/dist/esm/components/Page/index.js';

import cockpit from 'cockpit';
import { createIpcClient } from './ipc/client';
import { createChannel, resolveSocketPath } from './ipc/channel';

const _ = cockpit.gettext;

export const Application = () => {
    const [hostname, setHostname] = useState(_("Unknown"));
    const [socketPath, setSocketPath] = useState(resolveSocketPath());
    const [socketStatus, setSocketStatus] = useState('Checking socket…');
    const [socketInfo, setSocketInfo] = useState('');
    const [ipcStatus, setIpcStatus] = useState('Ready');
    const [ipcResponse, setIpcResponse] = useState('Waiting for ping');
    const [ipcError, setIpcError] = useState('');
    const [debugLog, setDebugLog] = useState<string[]>([]);

    const appendDebugLog = useCallback((entry: string) => {
        setDebugLog(prev => [...prev.slice(-9), entry]);
    }, []);

    const validateSocket = useCallback(async (path = socketPath) => {
        setSocketPath(path);

        try {
            await cockpit.spawn(['/usr/bin/test', '-S', path]);
            const info = await cockpit.spawn(['/usr/bin/stat', '-c', '%A %U %G %n', path]);
            setSocketStatus('Socket ready');
            setSocketInfo(info.trim());
            appendDebugLog(`socket-ok ${path}`);
            return info;
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            setSocketStatus('Socket missing or inaccessible');
            setSocketInfo(message);
            appendDebugLog(`socket-error ${path}: ${message}`);
            throw error;
        }
    }, [appendDebugLog, socketPath]);

    const runPingPong = useCallback(async () => {
        const path = resolveSocketPath();
        setSocketPath(path);
        setIpcStatus('Sending ping…');
        setIpcError('');
        setIpcResponse('');

        try {
            await validateSocket(path);
            const client = createIpcClient(createChannel(path));
            const requestId = `DEBUG-${Date.now()}`;
            appendDebugLog(`request ${requestId}`);
            const response = await Promise.race([
                client.send({ type: 'ping', messageId: requestId }),
                new Promise<never>((_, reject) => window.setTimeout(() => reject(new Error('ping timeout')), 5000)),
            ]);
            client.close();
            appendDebugLog(`response ${response.messageId} type=${response.type}`);
            setIpcStatus('Pong received');
            setIpcResponse(JSON.stringify(response));
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            appendDebugLog(`ping-failed ${message}`);
            setIpcStatus('Ping failed');
            setIpcError(message);
        }
    }, [appendDebugLog, validateSocket]);

    useEffect(() => {
        const hostname = cockpit.file('/etc/hostname');
        hostname.watch(content => setHostname(content?.trim() ?? ""));

        void validateSocket().catch(() => undefined);

        return () => {
            hostname.close();
        };
    }, [validateSocket]);

    return (
        <Page className='pf-m-no-sidebar'>
            <Card>
                <CardTitle>Socket integration</CardTitle>
                <CardBody>
                    <Alert
                    variant="info"
                    title={ cockpit.format(_("Running on $0"), hostname) }
                    />
                    <Alert
                    variant={ socketStatus === 'Socket ready' ? 'success' : 'warning' }
                    title={ socketStatus }
                    style={{ marginTop: '1rem' }}
                    >
                        { socketPath }
                        { socketInfo ? <div style={{ marginTop: '0.5rem' }}>{ socketInfo }</div> : null }
                    </Alert>
                    <div style={{ marginTop: '1rem' }}>
                        <Button variant="primary" onClick={ runPingPong }>Run ping/pong round trip</Button>
                    </div>
                    <Alert
                    variant={ ipcError ? 'danger' : 'success' }
                    title={ ipcStatus }
                    style={{ marginTop: '1rem' }}
                    >
                        { ipcError || ipcResponse }
                    </Alert>
                    <Card style={{ marginTop: '1rem' }}>
                        <CardTitle>Debug log</CardTitle>
                        <CardBody>
                            <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
                                { debugLog.length > 0 ? debugLog.join('\n') : 'No socket activity yet' }
                            </pre>
                        </CardBody>
                    </Card>
                </CardBody>
            </Card>
        </Page>
    );
};