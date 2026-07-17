import React, { useMemo } from 'react';
import { Alert, Badge, Card, CardBody, CardTitle } from '@patternfly/react-core';

import cockpit from 'cockpit';

import { EntityTable, type EntityTableColumn } from '../../components/EntityTable';
import { SummaryMetricsGallery } from '../../components/SummaryMetricsGallery';
import { getCurrentUserName } from '../../lib/cockpit/session';
import { RESERVATIONS_FIXTURES, type ReservationRecord } from './selfServiceData';

const _ = cockpit.gettext;

function formatCount(value: number) {
    return value.toLocaleString();
}

function badgeVariant(state: string) {
    switch (state) {
    case 'ACTIVE':
        return 'success';
    case 'UPCOMING':
        return 'warning';
    default:
        return 'neutral';
    }
}

export const MyReservationsPage = () => {
    const currentUser = getCurrentUserName();
    const reservations = useMemo(() => RESERVATIONS_FIXTURES.filter((reservation) => reservation.users.includes(currentUser)), [currentUser]);
    const active = reservations.filter((reservation) => reservation.state === 'ACTIVE').length;
    const upcoming = reservations.filter((reservation) => reservation.state === 'UPCOMING').length;
    const summaryMetrics = [
        { title: _('Reservations'), value: formatCount(reservations.length) },
        { title: _('Active'), value: formatCount(active) },
        { title: _('Upcoming'), value: formatCount(upcoming) },
    ];

    const columns: EntityTableColumn<ReservationRecord>[] = [
        {
            header: _('Name'),
            dataLabel: _('Name'),
            cell: (reservation) => (
                <>
                    {reservation.name}{' '}
                    <Badge isRead variant={badgeVariant(reservation.state)}>{reservation.state}</Badge>
                </>
            ),
        },
        {
            header: _('State'),
            dataLabel: _('State'),
            cell: (reservation) => reservation.state,
        },
        {
            header: _('Time'),
            dataLabel: _('Time'),
            cell: (reservation) => `${reservation.startTime} → ${reservation.endTime}`,
        },
        {
            header: _('Nodes'),
            dataLabel: _('Nodes'),
            cell: (reservation) => reservation.nodes,
        },
        {
            header: _('Purpose'),
            dataLabel: _('Purpose'),
            cell: (reservation) => reservation.purpose,
        },
    ];

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryMetricsGallery metrics={summaryMetrics} />

            <Card>
                <CardTitle>{cockpit.format(_('My reservations for $0'), currentUser)}</CardTitle>
                <CardBody>
                    <EntityTable
                        ariaLabel={_('My reservations table')}
                        columns={columns}
                        rows={reservations}
                        rowKey={(reservation) => reservation.name}
                        emptyState={<Alert variant="info" title={_('No reservations are associated with the current user.')} />}
                    />
                </CardBody>
            </Card>
        </div>
    );
};
