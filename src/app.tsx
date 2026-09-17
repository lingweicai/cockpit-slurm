/*
 * SPDX-License-Identifier: LGPL-2.1-or-later
 *
 * Copyright (C) 2017 Red Hat, Inc.
 */

import React, { useEffect, useState } from 'react';
import { Alert } from "@patternfly/react-core/dist/esm/components/Alert/index.js";
import { Card, CardBody, CardTitle } from "@patternfly/react-core/dist/esm/components/Card/index.js";
import { Page } from '@patternfly/react-core/dist/esm/components/Page/index.js';

import cockpit from 'cockpit';
import { createIpcClient } from './ipc/client';

const _ = cockpit.gettext;

export const Application = () => {
    const [hostname, setHostname] = useState(_("Unknown"));
    const [ipcStatus, setIpcStatus] = useState('Connecting');
    const [ipcResponse, setIpcResponse] = useState('Waiting for pong');
    const [ipcError, setIpcError] = useState('');

    useEffect(() => {
        const hostname = cockpit.file('/etc/hostname');
        hostname.watch(content => setHostname(content?.trim() ?? ""));

        const client = createIpcClient();
        const timer = window.setTimeout(() => {
            void client.send({ type: 'ping' }).then(response => {
                setIpcStatus('Connected');
                setIpcResponse(JSON.stringify(response));
            }).catch(error => {
                setIpcStatus('Connection error');
                setIpcError(error instanceof Error ? error.message : String(error));
            });
        }, 100);

        return () => {
            window.clearTimeout(timer);
            client.close();
            hostname.close();
        };
    }, []);

    return (
        <Page className='pf-m-no-sidebar'>
            <Card>
                <CardTitle>Starter Kit</CardTitle>
                <CardBody>
                    <Alert
                    variant="info"
                    title={ cockpit.format(_("Running on $0"), hostname) }
                    />
                    <Alert
                    variant={ ipcError ? 'danger' : 'success' }
                    title={ ipcStatus }
                    style={{ marginTop: '1rem' }}
                    >
                        { ipcError || ipcResponse }
                    </Alert>
                </CardBody>
            </Card>
        </Page>
    );
};