/*
 * SPDX-License-Identifier: LGPL-2.1-or-later
 *
 * Copyright (C) 2017 Red Hat, Inc.
 */

import React, { useEffect, useState } from 'react';
import { Card, CardBody, CardTitle } from "@patternfly/react-core/dist/esm/components/Card/index.js";

import cockpit from 'cockpit';

import { AppShell } from './components/AppShell';
import { getBreadcrumbTrail, getCurrentRole, getNavigationItems, normalizePageId, type AppPageId, type AppRole } from './app/navigation';
import { ChannelProvider } from './lib/cockpit';
import { ClusterOverviewPage } from './features/cluster';
import { Dashboard } from './features/dashboard';
import { AccountsPage, QosPage, ReportsPage, ReservationsPage, SettingsPage, UsersPage } from './features/admin';
import { JobsPage } from './features/jobs';
import { NodesPage } from './features/nodes';
import { PartitionsPage } from './features/partitions';
import { MyFilesPage, MyJobsPage, MyReservationsPage, SubmitJobPage } from './features/self-service';
import type { SinfoPartitionRow } from './types/sinfo';
import { fetchSinfo, subscribeSinfoUpdates } from './services/sinfoChannel';

const _ = cockpit.gettext;

const INITIAL_LOAD_RETRIES = 5;
const INITIAL_LOAD_DELAY_MS = 500;

function isInitializedSinfoPayload(payload: { updated_at?: string | null }) {
    if (!payload.updated_at) {
        return false;
    }

    const timestamp = Date.parse(payload.updated_at);
    return Number.isFinite(timestamp) && timestamp > 0;
}

function renderPlaceholderPage(title: string, description: string) {
    return (
        <Card>
            <CardTitle>{title}</CardTitle>
            <CardBody>
                <p>{description}</p>
            </CardBody>
        </Card>
    );
}

function renderPageContent(
    pageId: AppPageId,
    role: AppRole,
    state: {
        loading: boolean;
        rows: SinfoPartitionRow[];
        updatedAt: string | null;
        waitMessage: string | null;
        error: string | null;
    },
) {
    switch (pageId) {
    case 'dashboard':
        return (
            <Dashboard
                loading={state.loading}
                rows={state.rows}
                updatedAt={state.updatedAt}
                waitMessage={state.waitMessage}
                error={state.error}
            />
        );
    case 'partitions':
        return (
            <PartitionsPage
                loading={state.loading}
                rows={state.rows}
                updatedAt={state.updatedAt}
                waitMessage={state.waitMessage}
                error={state.error}
            />
        );
    case 'nodes':
        return (
            <NodesPage
                loading={state.loading}
                rows={state.rows}
                updatedAt={state.updatedAt}
                waitMessage={state.waitMessage}
                error={state.error}
            />
        );
    case 'cluster-overview':
        return (
            <ClusterOverviewPage
                loading={state.loading}
                rows={state.rows}
                updatedAt={state.updatedAt}
                waitMessage={state.waitMessage}
                error={state.error}
            />
        );
    case 'jobs':
        return <JobsPage role={role} />;
    case 'my-jobs':
        return <MyJobsPage />;
    case 'submit-job':
        return <SubmitJobPage />;
    case 'my-reservations':
        return <MyReservationsPage />;
    case 'my-files':
        return <MyFilesPage />;
    case 'users':
        return <UsersPage />;
    case 'accounts':
        return <AccountsPage />;
    case 'qos':
        return <QosPage />;
    case 'reservations':
        return <ReservationsPage />;
    case 'reports':
        return <ReportsPage />;
    case 'settings':
        return <SettingsPage />;
    default:
        return renderPlaceholderPage(_('Dashboard'), _('Cluster health and summary widgets will be added here next.'));
    }
}

export const Application = () => {
    const [rows, setRows] = useState<SinfoPartitionRow[]>([]);
    const [updatedAt, setUpdatedAt] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [waitMessage, setWaitMessage] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [channelKey, setChannelKey] = useState(0);
    const [activeCluster, setActiveCluster] = useState('production');
    const [pageId, setPageId] = useState<AppPageId>(() => {
        const role = getCurrentRole();
        const currentHash = typeof window !== 'undefined' ? window.location.hash.replace(/^#/, '') : null;
        return normalizePageId(currentHash, role);
    });
    const role = getCurrentRole();
    const navigationItems = getNavigationItems(role);
    const clusterOptions = ['production', 'gpu', 'testing'];

    useEffect(() => {
        let isMounted = true;

        const waitForInitializedCache = async () => {
            for (let attempt = 0; attempt < INITIAL_LOAD_RETRIES; attempt += 1) {
                const payload = await fetchSinfo();

                if (!isMounted) {
                    return null;
                }

                if (isInitializedSinfoPayload(payload)) {
                    return payload;
                }

                if (attempt < INITIAL_LOAD_RETRIES - 1) {
                    await new Promise((resolve) => setTimeout(resolve, INITIAL_LOAD_DELAY_MS));
                }
            }

            return null;
        };

        const loadSinfo = async () => {
            setLoading(true);
            setError(null);
            setWaitMessage(_('Waiting for bridge cache...'));

            try {
                const payload = await waitForInitializedCache();
                if (!isMounted) {
                    return;
                }

                if (!payload) {
                    throw new Error(_('Bridge cache did not initialize in time.'));
                }

                setWaitMessage(null);
                setRows(payload.rows ?? []);
                setUpdatedAt(payload.updated_at ?? null);
            } catch (err: unknown) {
                if (!isMounted) {
                    return;
                }

                setWaitMessage(null);
                setError(err instanceof Error ? err.message : _('Unable to load sinfo data.'));
            } finally {
                if (isMounted) {
                    setLoading(false);
                }
            }
        };

        loadSinfo();

        const unsubscribe = subscribeSinfoUpdates((payload) => {
            if (payload?.type !== 'sinfo.updated') {
                return;
            }

            (async () => {
                const freshPayload = await fetchSinfo();
                if (!isMounted) {
                    return;
                }

                setRows(freshPayload.rows ?? []);
                setUpdatedAt(freshPayload.updated_at ?? null);
            })().catch(() => {
                // Keep the current table contents if the refresh fails.
            });
        });

        return () => {
            isMounted = false;
            unsubscribe();
        };
    }, []);

    useEffect(() => {
        const onHashChange = () => {
            setPageId(normalizePageId(window.location.hash.replace(/^#/, ''), role));
        };

        window.addEventListener('hashchange', onHashChange);
        onHashChange();

        return () => {
            window.removeEventListener('hashchange', onHashChange);
        };
    }, [role]);

    return (
        <ChannelProvider key={channelKey}>
            <AppShell
                role={role}
                pageId={pageId}
                navigationItems={navigationItems}
                breadcrumbs={getBreadcrumbTrail(role, pageId)}
                activeCluster={activeCluster}
                clusterOptions={clusterOptions}
                onClusterChange={setActiveCluster}
                onRefresh={() => setChannelKey((current) => current + 1)}
                onNavigate={(nextPageId) => {
                    const nextHash = `#${nextPageId}`;
                    if (window.location.hash !== nextHash) {
                        window.location.hash = nextHash;
                    }
                    setPageId(nextPageId);
                }}
            >
                {renderPageContent(pageId, role, {
                    loading,
                    rows,
                    updatedAt,
                    waitMessage,
                    error,
                })}
            </AppShell>
        </ChannelProvider>
    );
};
