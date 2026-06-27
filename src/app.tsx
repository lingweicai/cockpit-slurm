/*
 * SPDX-License-Identifier: LGPL-2.1-or-later
 *
 * Copyright (C) 2017 Red Hat, Inc.
 */

import React, { useEffect, useState } from 'react';
import { Alert } from "@patternfly/react-core/dist/esm/components/Alert/index.js";
import { Card, CardBody, CardTitle } from "@patternfly/react-core/dist/esm/components/Card/index.js";
import { ExpandableRowContent, Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import { ConnectionBanner } from './components/ConnectionBanner';
import { ChannelProvider } from './lib/cockpit';
import type { SinfoPartitionRow } from './types/sinfo';
import { fetchSinfo, subscribeSinfoUpdates } from './services/sinfoChannel';

const _ = cockpit.gettext;

type ExpandedState = Record<number, boolean>;

function formatUpdatedAt(value: string | null) {
    if (!value) {
        return _('Unknown');
    }

    const timestamp = new Date(value);
    if (Number.isNaN(timestamp.getTime())) {
        return value;
    }

    return timestamp.toLocaleString();
}

function formatSummary(value: string | undefined | null, fallback = _('Unknown')) {
    return value?.trim() || fallback;
}

const INITIAL_LOAD_RETRIES = 5;
const INITIAL_LOAD_DELAY_MS = 500;

function isInitializedSinfoPayload(payload: { updated_at?: string | null }) {
    if (!payload.updated_at) {
        return false;
    }

    const timestamp = Date.parse(payload.updated_at);
    return Number.isFinite(timestamp) && timestamp > 0;
}

function renderDetails(row: SinfoPartitionRow) {
    return (
        <div style={{ display: 'grid', gap: '0.5rem' }}>
            <div><strong>{_('Node list')}:</strong> {formatSummary(row.nodeList, _('N/A'))}</div>
            <div><strong>{_('Node state')}:</strong> {formatSummary(row.nodeState)}</div>
            <div><strong>{_('Partition state')}:</strong> {row.partitionState?.join(', ') || _('Unknown')}</div>
            <div><strong>{_('Availability')}:</strong> {formatSummary(row.availability)}</div>
            <div><strong>{_('Features')}:</strong> {formatSummary(row.featuresActive, _('N/A'))}</div>
            <div><strong>{_('GRES used')}:</strong> {formatSummary(row.gresUsed, _('N/A'))}</div>
            <div><strong>{_('Reservation')}:</strong> {formatSummary(row.reservation, _('N/A'))}</div>
            <div><strong>{_('Comment')}:</strong> {formatSummary(row.comment, _('None'))}</div>
            <div><strong>{_('Reason')}:</strong> {formatSummary(row.reasonDescription, _('N/A'))}</div>
            <div><strong>{_('Reason user')}:</strong> {formatSummary(row.reasonUser, _('N/A'))}</div>
            <div><strong>{_('Reason time')}:</strong> {row.reasonTime || _('N/A')}</div>
            <div><strong>{_('Time limit')}:</strong> {formatSummary(row.timeLimit, _('N/A'))}</div>
            <div><strong>{_('Partition TRES')}:</strong> {formatSummary(row.partitionTRES, _('N/A'))}</div>
            <div><strong>{_('Memory free min/max')}:</strong> {row.memoryFreeMin} / {row.memoryFreeMax}</div>
            <div><strong>{_('Memory allocated')}:</strong> {row.memoryAllocated}</div>
            <div><strong>{_('CPU totals')}:</strong> {row.cpusTotal} ({_('allocated')} {row.cpusAllocated}, {_('idle')} {row.cpusIdle}, {_('other')} {row.cpusOther})</div>
            <div><strong>{_('Node totals')}:</strong> {row.nodesTotal} ({_('allocated')} {row.nodesAllocated}, {_('idle')} {row.nodesIdle}, {_('other')} {row.nodesOther})</div>
        </div>
    );
}

export const Application = () => {
    const [rows, setRows] = useState<SinfoPartitionRow[]>([]);
    const [updatedAt, setUpdatedAt] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [waitMessage, setWaitMessage] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [expandedRows, setExpandedRows] = useState<ExpandedState>({});

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

            fetchSinfo()
                .then((freshPayload) => {
                    if (!isMounted) {
                        return;
                    }

                    setRows(freshPayload.rows ?? []);
                    setUpdatedAt(freshPayload.updated_at ?? null);
                })
                .catch(() => {
                    // Keep the current table contents if the refresh fails.
                });
        });

        return () => {
            isMounted = false;
            unsubscribe();
        };
    }, []);

    const handleToggle = (rowIndex: number) => {
        setExpandedRows((current) => ({
            ...current,
            [rowIndex]: !current[rowIndex],
        }));
    };

    return (
        <ChannelProvider>
            <ConnectionBanner />
            <Card>
                <CardTitle>{_('Sinfo partitions')}</CardTitle>
                <CardBody>
                    {loading && !rows.length && (
                        <>
                            <Alert variant="info" title={_('Loading sinfo data from the bridge cache...')} />
                            {waitMessage && <p>{waitMessage}</p>}
                        </>
                    )}
                    {error && (
                        <Alert variant="danger" title={_('Unable to load sinfo data')}>
                            {error}
                        </Alert>
                    )}

                    {!loading && !error && (
                        <>
                            <p>{cockpit.format(_('Last update: $0'), formatUpdatedAt(updatedAt))}</p>

                            {rows.length === 0 ? (
                                <Alert variant="info" title={_('No sinfo rows are currently available.')} />
                            ) : (
                                <Table variant="compact">
                                    <Thead>
                                        <Tr>
                                            <Th screenReaderText={_('Expand row')} />
                                            <Th>{_('Partition')}</Th>
                                            <Th>{_('State')}</Th>
                                            <Th>{_('Nodes')}</Th>
                                            <Th>{_('CPUs')}</Th>
                                            <Th>{_('Memory')}</Th>
                                            <Th>{_('Availability')}</Th>
                                        </Tr>
                                    </Thead>
                                    {rows.map((row, rowIndex) => {
                                        const isExpanded = Boolean(expandedRows[rowIndex]);

                                        return (
                                            <Tbody key={row.partitionName} isExpanded={isExpanded}>
                                                <Tr>
                                                    <Td
                                                        expand={{
                                                            isExpanded,
                                                            rowIndex,
                                                            onToggle: () => handleToggle(rowIndex),
                                                        }}
                                                    />
                                                    <Td dataLabel={_('Partition')}>{row.partitionName}</Td>
                                                    <Td dataLabel={_('State')}>{row.partitionState?.join(', ') || _('Unknown')}</Td>
                                                    <Td dataLabel={_('Nodes')}>{row.nodesAllocated}/{row.nodesTotal}</Td>
                                                    <Td dataLabel={_('CPUs')}>{row.cpusAllocated}/{row.cpusTotal}</Td>
                                                    <Td dataLabel={_('Memory')}>{row.memoryAllocated}</Td>
                                                    <Td dataLabel={_('Availability')}>{row.availability}</Td>
                                                </Tr>
                                                <Tr isExpanded={isExpanded}>
                                                    <Td colSpan={7}>
                                                        <ExpandableRowContent>
                                                            {renderDetails(row)}
                                                        </ExpandableRowContent>
                                                    </Td>
                                                </Tr>
                                            </Tbody>
                                        );
                                    })}
                                </Table>
                            )}
                        </>
                    )}
                </CardBody>
            </Card>
        </ChannelProvider>
    );
};
