/*
 * SPDX-License-Identifier: LGPL-2.1-or-later
 *
 * Copyright (C) 2017 Red Hat, Inc.
 */

import React, { useEffect, useState } from 'react';

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

function parseHashRoute(hash: string) {
    const cleanHash = hash.replace(/^#/, '');
    const [pagePart, queryPart] = cleanHash.split('?', 2);
    const query = new URLSearchParams(queryPart ?? '');

    return {
        pageId: pagePart,
        selectedNode: query.get('node'),
        selectedJob: query.get('job'),
        selectedPartition: query.get('partition'),
        selectedUser: query.get('user'),
        selectedAccount: query.get('account'),
        selectedReservation: query.get('reservation'),
        selectedQos: query.get('qos'),
        selectedReport: query.get('report'),
    };
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
        selectedNode: string | null;
        selectedJob: string | null;
        selectedPartition: string | null;
        selectedUser: string | null;
        selectedAccount: string | null;
        selectedReservation: string | null;
        selectedQos: string | null;
        selectedReport: string | null;
    },
    onSelectNode: (nodeName: string) => void,
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
                initialSelectedPartition={state.selectedPartition}
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
                initialSelectedNode={state.selectedNode}
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
                onSelectNode={onSelectNode}
            />
        );
    case 'jobs':
        return <JobsPage role={role} initialSelectedJob={state.selectedJob} />;
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
        return <QosPage initialSelectedQos={state.selectedQos} />;
    case 'reservations':
        return <ReservationsPage initialSelectedReservation={state.selectedReservation} />;
    case 'reports':
        return <ReportsPage initialSelectedReport={state.selectedReport} />;
    case 'settings':
        return <SettingsPage />;
    default:
        return (
            <Dashboard
                loading={state.loading}
                rows={state.rows}
                updatedAt={state.updatedAt}
                waitMessage={state.waitMessage}
                error={state.error}
            />
        );
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
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return normalizePageId(parseHashRoute(currentHash).pageId, role);
    });
    const [selectedNode, setSelectedNode] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedNode;
    });
    const [selectedJob, setSelectedJob] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedJob;
    });
    const [selectedPartition, setSelectedPartition] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedPartition;
    });
    const [selectedUser, setSelectedUser] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedUser;
    });
    const [selectedAccount, setSelectedAccount] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedAccount;
    });
    const [selectedReservation, setSelectedReservation] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedReservation;
    });
    const [selectedQos, setSelectedQos] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedQos;
    });
    const [selectedReport, setSelectedReport] = useState<string | null>(() => {
        const currentHash = typeof window !== 'undefined' ? window.location.hash : '';
        return parseHashRoute(currentHash).selectedReport;
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
            if (!payload || typeof payload !== 'object') {
                return;
            }

            const record = payload as Record<string, unknown>;
            const isSinfoEvent = record.type === 'event' && record.entity === 'sinfo';
            if (!isSinfoEvent) {
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
            const nextRoute = parseHashRoute(window.location.hash);
            setPageId(normalizePageId(nextRoute.pageId, role));
            setSelectedNode(nextRoute.selectedNode);
            setSelectedJob(nextRoute.selectedJob);
            setSelectedPartition(nextRoute.selectedPartition);
            setSelectedUser(nextRoute.selectedUser);
            setSelectedAccount(nextRoute.selectedAccount);
            setSelectedReservation(nextRoute.selectedReservation);
            setSelectedQos(nextRoute.selectedQos);
            setSelectedReport(nextRoute.selectedReport);
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
                    setSelectedNode(null);
                    setSelectedJob(null);
                    setSelectedPartition(null);
                    setSelectedUser(null);
                    setSelectedAccount(null);
                    setSelectedReservation(null);
                    setSelectedQos(null);
                    setSelectedReport(null);
                }}
            >
                {renderPageContent(pageId, role, {
                    loading,
                    rows,
                    updatedAt,
                    waitMessage,
                    error,
                    selectedNode,
                    selectedJob,
                    selectedPartition,
                    selectedUser,
                    selectedAccount,
                    selectedReservation,
                    selectedQos,
                    selectedReport,
                }, (nodeName) => {
                    const nextHash = `#nodes?node=${encodeURIComponent(nodeName)}`;
                    if (window.location.hash !== nextHash) {
                        window.location.hash = nextHash;
                    }
                    setPageId('nodes');
                    setSelectedNode(nodeName);
                    setSelectedJob(null);
                    setSelectedPartition(null);
                    setSelectedUser(null);
                    setSelectedAccount(null);
                    setSelectedReservation(null);
                    setSelectedQos(null);
                    setSelectedReport(null);
                })}
            </AppShell>
        </ChannelProvider>
    );
};
