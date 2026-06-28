import React, { useMemo } from 'react';
import { Alert, Card, CardBody, CardTitle, Gallery, GalleryItem } from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import { getCurrentUserName } from '../../lib/cockpit/session';
import { JOB_FIXTURES } from '../jobs/jobsData';

const _ = cockpit.gettext;

function formatCount(value: number) {
    return value.toLocaleString();
}

export const MyJobsPage = () => {
    const currentUser = getCurrentUserName();
    const jobs = useMemo(() => JOB_FIXTURES.filter((job) => job.user === currentUser), [currentUser]);
    const running = jobs.filter((job) => job.state === 'RUNNING').length;
    const pending = jobs.filter((job) => job.state === 'PENDING').length;
    const completed = jobs.filter((job) => job.state === 'COMPLETED').length;

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <Gallery hasGutter>
                <GalleryItem><Card><CardTitle>{_('My jobs')}</CardTitle><CardBody><strong>{formatCount(jobs.length)}</strong></CardBody></Card></GalleryItem>
                <GalleryItem><Card><CardTitle>{_('Running')}</CardTitle><CardBody><strong>{formatCount(running)}</strong></CardBody></Card></GalleryItem>
                <GalleryItem><Card><CardTitle>{_('Pending')}</CardTitle><CardBody><strong>{formatCount(pending)}</strong></CardBody></Card></GalleryItem>
                <GalleryItem><Card><CardTitle>{_('Completed')}</CardTitle><CardBody><strong>{formatCount(completed)}</strong></CardBody></Card></GalleryItem>
            </Gallery>

            <Card>
                <CardTitle>{cockpit.format(_('Jobs for $0'), currentUser)}</CardTitle>
                <CardBody>
                    {jobs.length === 0 && (
                        <Alert variant="info" title={_('No jobs found for the current user.')} />
                    )}
                    {jobs.length > 0 && (
                        <Table aria-label={_('My jobs table')} variant="compact">
                            <Thead>
                                <Tr>
                                    <Th>{_('JobID')}</Th>
                                    <Th>{_('Name')}</Th>
                                    <Th>{_('Partition')}</Th>
                                    <Th>{_('State')}</Th>
                                    <Th>{_('Runtime')}</Th>
                                </Tr>
                            </Thead>
                            <Tbody>
                                {jobs.map((job) => (
                                    <Tr key={job.jobId}>
                                        <Td dataLabel={_('JobID')}>{job.jobId}</Td>
                                        <Td dataLabel={_('Name')}>{job.name}</Td>
                                        <Td dataLabel={_('Partition')}>{job.partition}</Td>
                                        <Td dataLabel={_('State')}>{job.state}</Td>
                                        <Td dataLabel={_('Runtime')}>{job.runtime}</Td>
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
