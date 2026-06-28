import React, { useMemo } from 'react';
import { Alert, Badge, Card, CardBody, CardTitle, Gallery, GalleryItem } from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import { getCurrentUserName } from '../../lib/cockpit/session';
import { RESERVATIONS_FIXTURES } from './selfServiceData';

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
                    {reservations.length === 0 && (
                        <Alert variant="info" title={_('No reservations are associated with the current user.')} />
                    )}
                    {reservations.length > 0 && (
                        <Table aria-label={_('My reservations table')} variant="compact">
                            <Thead>
                                <Tr>
                                    <Th>{_('Name')}</Th>
                                    <Th>{_('State')}</Th>
                                    <Th>{_('Time')}</Th>
                                    <Th>{_('Nodes')}</Th>
                                    <Th>{_('Purpose')}</Th>
                                </Tr>
                            </Thead>
                            <Tbody>
                                {reservations.map((reservation) => (
                                    <Tr key={reservation.name}>
                                        <Td dataLabel={_('Name')}>
                                            {reservation.name}{' '}
                                            <Badge isRead variant={badgeVariant(reservation.state)}>{reservation.state}</Badge>
                                        </Td>
                                        <Td dataLabel={_('State')}>{reservation.state}</Td>
                                        <Td dataLabel={_('Time')}>{reservation.startTime} → {reservation.endTime}</Td>
                                        <Td dataLabel={_('Nodes')}>{reservation.nodes}</Td>
                                        <Td dataLabel={_('Purpose')}>{reservation.purpose}</Td>
                                    </Tr>
                                ))}
                            </Tbody>
                        </Table>
                    )}
                </CardBody>
            </Card>
        </div>
    );
};
