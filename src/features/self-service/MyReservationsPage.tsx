import React, { useMemo } from 'react';
import { Alert, Badge, Card, CardBody, CardTitle, Gallery, GalleryItem } from '@patternfly/react-core';

import cockpit from 'cockpit';

import { EntityTable, type EntityTableColumn } from '../../components/EntityTable';
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
            <Gallery hasGutter>
                <GalleryItem><Card><CardTitle>{_('Reservations')}</CardTitle><CardBody><strong>{formatCount(reservations.length)}</strong></CardBody></Card></GalleryItem>
                <GalleryItem><Card><CardTitle>{_('Active')}</CardTitle><CardBody><strong>{formatCount(active)}</strong></CardBody></Card></GalleryItem>
                <GalleryItem><Card><CardTitle>{_('Upcoming')}</CardTitle><CardBody><strong>{formatCount(upcoming)}</strong></CardBody></Card></GalleryItem>
            </Gallery>

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
