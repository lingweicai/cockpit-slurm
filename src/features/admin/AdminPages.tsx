import React, { useMemo, useState } from 'react';
import {
    Alert,
    Badge,
    Button,
    Card,
    CardBody,
    CardTitle,
    Checkbox,
    Form,
    FormGroup,
    Gallery,
    GalleryItem,
    Progress,
    Switch,
    TextInput,
} from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import cockpit from 'cockpit';

import {
    ACCOUNT_FIXTURES,
    ADMIN_SETTINGS,
    QOS_FIXTURES,
    REPORT_FIXTURES,
    RESERVATION_FIXTURES,
    USER_FIXTURES,
    type AccountRecord,
    type QosRecord,
    type ReservationRecord,
    type UserRecord,
} from './adminData';

const _ = cockpit.gettext;

function formatCount(value: number) {
    return value.toLocaleString();
}

function formatPercent(numerator: number, denominator: number) {
    if (denominator === 0) {
        return '0%';
    }

    return `${Math.round((numerator / denominator) * 100)}%`;
}

function formatTimestamp(value: string) {
    return new Date(value).toLocaleString();
}

function badgeVariantForUser(state: UserRecord['state']) {
    switch (state) {
    case 'ACTIVE':
        return 'success';
    case 'PENDING':
        return 'warning';
    default:
        return 'danger';
    }
}

function badgeVariantForAccount(state: AccountRecord['state']) {
    return state === 'ACTIVE' ? 'success' : 'warning';
}

function badgeVariantForQos(state: QosRecord['state']) {
    return state === 'ENABLED' ? 'success' : 'warning';
}

function badgeVariantForReservation(state: ReservationRecord['state']) {
    switch (state) {
    case 'ACTIVE':
        return 'success';
    case 'UPCOMING':
        return 'warning';
    default:
        return 'neutral';
    }
}

function SummaryCards({ items }: { items: Array<{ title: string; value: string; description: string; }> }) {
    return (
        <Gallery hasGutter>
            {items.map((item) => (
                <GalleryItem key={item.title}>
                    <Card>
                        <CardTitle>{item.title}</CardTitle>
                        <CardBody>
                            <strong>{item.value}</strong>
                            <div>{item.description}</div>
                        </CardBody>
                    </Card>
                </GalleryItem>
            ))}
        </Gallery>
    );
}

export const UsersPage = () => {
    const totalUsers = USER_FIXTURES.length;
    const activeUsers = USER_FIXTURES.filter((user) => user.state === 'ACTIVE').length;
    const pendingUsers = USER_FIXTURES.filter((user) => user.state === 'PENDING').length;
    const mfaUsers = USER_FIXTURES.filter((user) => user.mfaEnabled).length;

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryCards
                items={[
                    { title: _('Users'), value: formatCount(totalUsers), description: _('All managed identities in the cluster.') },
                    { title: _('Active'), value: formatCount(activeUsers), description: _('Users currently allowed to log in.') },
                    { title: _('Pending'), value: formatCount(pendingUsers), description: _('Accounts waiting for approval.') },
                    { title: _('MFA enabled'), value: formatCount(mfaUsers), description: _('Users protected with multi-factor auth.') },
                ]}
            />

            <Card>
                <CardTitle>{_('Managed users')}</CardTitle>
                <CardBody>
                    <Table aria-label={_('Managed users table')} variant="compact">
                        <Thead>
                            <Tr>
                                <Th>{_('Name')}</Th>
                                <Th>{_('Role')}</Th>
                                <Th>{_('Accounts')}</Th>
                                <Th>{_('State')}</Th>
                                <Th>{_('Last login')}</Th>
                                <Th>{_('MFA')}</Th>
                                <Th>{_('Default QOS')}</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {USER_FIXTURES.map((user) => (
                                <Tr key={user.name}>
                                    <Td dataLabel={_('Name')}>{user.name}</Td>
                                    <Td dataLabel={_('Role')}>{user.role}</Td>
                                    <Td dataLabel={_('Accounts')}>{user.accounts.join(', ')}</Td>
                                    <Td dataLabel={_('State')}>
                                        <Badge isRead variant={badgeVariantForUser(user.state)}>{user.state}</Badge>
                                    </Td>
                                    <Td dataLabel={_('Last login')}>{formatTimestamp(user.lastLogin)}</Td>
                                    <Td dataLabel={_('MFA')}>{user.mfaEnabled ? _('Enabled') : _('Disabled')}</Td>
                                    <Td dataLabel={_('Default QOS')}>{user.defaultQos}</Td>
                                </Tr>
                            ))}
                        </Tbody>
                    </Table>
                </CardBody>
            </Card>
        </div>
    );
};

export const AccountsPage = () => {
    const totalUsed = ACCOUNT_FIXTURES.reduce((sum, account) => sum + account.cpuHoursUsed, 0);
    const totalLimit = ACCOUNT_FIXTURES.reduce((sum, account) => sum + account.cpuHoursLimit, 0);
    const limitedAccounts = ACCOUNT_FIXTURES.filter((account) => account.state === 'LIMITED').length;

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryCards
                items={[
                    { title: _('Accounts'), value: formatCount(ACCOUNT_FIXTURES.length), description: _('Allocated account trees and billing scopes.') },
                    { title: _('CPU hours used'), value: formatCount(totalUsed), description: _('Current accounting usage across all accounts.') },
                    { title: _('Capacity used'), value: formatPercent(totalUsed, totalLimit), description: _('Overall usage against configured limits.') },
                    { title: _('Limited accounts'), value: formatCount(limitedAccounts), description: _('Accounts approaching policy thresholds.') },
                ]}
            />

            <Card>
                <CardTitle>{_('Account usage')}</CardTitle>
                <CardBody>
                    <Table aria-label={_('Account usage table')} variant="compact">
                        <Thead>
                            <Tr>
                                <Th>{_('Account')}</Th>
                                <Th>{_('Parent')}</Th>
                                <Th>{_('Users')}</Th>
                                <Th>{_('Usage')}</Th>
                                <Th>{_('State')}</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {ACCOUNT_FIXTURES.map((account) => (
                                <Tr key={account.name}>
                                    <Td dataLabel={_('Account')}>{account.name}</Td>
                                    <Td dataLabel={_('Parent')}>{account.parent}</Td>
                                    <Td dataLabel={_('Users')}>{formatCount(account.users)}</Td>
                                    <Td dataLabel={_('Usage')}>
                                        <div style={{ display: 'grid', gap: '0.25rem' }}>
                                            <div>{formatPercent(account.cpuHoursUsed, account.cpuHoursLimit)}</div>
                                            <Progress
                                                value={Math.min((account.cpuHoursUsed / account.cpuHoursLimit) * 100, 100)}
                                                aria-label={cockpit.format(_('Account $0 usage'), account.name)}
                                            />
                                        </div>
                                    </Td>
                                    <Td dataLabel={_('State')}>
                                        <Badge isRead variant={badgeVariantForAccount(account.state)}>{account.state}</Badge>
                                    </Td>
                                </Tr>
                            ))}
                        </Tbody>
                    </Table>
                </CardBody>
            </Card>
        </div>
    );
};

export const QosPage = () => {
    const enabledQos = QOS_FIXTURES.filter((qos) => qos.state === 'ENABLED').length;

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryCards
                items={[
                    { title: _('QOS profiles'), value: formatCount(QOS_FIXTURES.length), description: _('Quality-of-service policies available to users.') },
                    { title: _('Enabled'), value: formatCount(enabledQos), description: _('Profiles currently accepting new allocations.') },
                    { title: _('Paused'), value: formatCount(QOS_FIXTURES.length - enabledQos), description: _('Profiles temporarily disabled by administrators.') },
                    { title: _('Highest priority'), value: `${Math.max(...QOS_FIXTURES.map((qos) => qos.priority))}`, description: _('Largest scheduling priority currently configured.') },
                ]}
            />

            <Card>
                <CardTitle>{_('QOS policies')}</CardTitle>
                <CardBody>
                    <Table aria-label={_('QOS policies table')} variant="compact">
                        <Thead>
                            <Tr>
                                <Th>{_('Name')}</Th>
                                <Th>{_('Priority')}</Th>
                                <Th>{_('Max jobs')}</Th>
                                <Th>{_('Max nodes')}</Th>
                                <Th>{_('Max wall time')}</Th>
                                <Th>{_('Preempt')}</Th>
                                <Th>{_('State')}</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {QOS_FIXTURES.map((qos) => (
                                <Tr key={qos.name}>
                                    <Td dataLabel={_('Name')}>{qos.name}</Td>
                                    <Td dataLabel={_('Priority')}>{qos.priority}</Td>
                                    <Td dataLabel={_('Max jobs')}>{qos.maxJobs}</Td>
                                    <Td dataLabel={_('Max nodes')}>{qos.maxNodes}</Td>
                                    <Td dataLabel={_('Max wall time')}>{qos.maxWallTime}</Td>
                                    <Td dataLabel={_('Preempt')}>{qos.preemptMode}</Td>
                                    <Td dataLabel={_('State')}>
                                        <Badge isRead variant={badgeVariantForQos(qos.state)}>{qos.state}</Badge>
                                    </Td>
                                </Tr>
                            ))}
                        </Tbody>
                    </Table>
                </CardBody>
            </Card>
        </div>
    );
};

export const ReservationsPage = () => {
    const activeReservations = RESERVATION_FIXTURES.filter((reservation) => reservation.state === 'ACTIVE').length;
    const upcomingReservations = RESERVATION_FIXTURES.filter((reservation) => reservation.state === 'UPCOMING').length;

    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryCards
                items={[
                    { title: _('Reservations'), value: formatCount(RESERVATION_FIXTURES.length), description: _('Reservation objects visible to administrators.') },
                    { title: _('Active'), value: formatCount(activeReservations), description: _('Reservations currently consuming nodes.') },
                    { title: _('Upcoming'), value: formatCount(upcomingReservations), description: _('Reservations scheduled to start soon.') },
                    { title: _('Accounts covered'), value: formatCount(new Set(RESERVATION_FIXTURES.flatMap((reservation) => reservation.accounts)).size), description: _('Distinct accounts attached to reservations.') },
                ]}
            />

            <Card>
                <CardTitle>{_('Reservation schedule')}</CardTitle>
                <CardBody>
                    <Table aria-label={_('Reservation schedule table')} variant="compact">
                        <Thead>
                            <Tr>
                                <Th>{_('Name')}</Th>
                                <Th>{_('State')}</Th>
                                <Th>{_('Window')}</Th>
                                <Th>{_('Nodes')}</Th>
                                <Th>{_('Accounts')}</Th>
                                <Th>{_('Users')}</Th>
                                <Th>{_('Purpose')}</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {RESERVATION_FIXTURES.map((reservation) => (
                                <Tr key={reservation.name}>
                                    <Td dataLabel={_('Name')}>{reservation.name}</Td>
                                    <Td dataLabel={_('State')}>
                                        <Badge isRead variant={badgeVariantForReservation(reservation.state)}>{reservation.state}</Badge>
                                    </Td>
                                    <Td dataLabel={_('Window')}>
                                        {formatTimestamp(reservation.startTime)} → {formatTimestamp(reservation.endTime)}
                                    </Td>
                                    <Td dataLabel={_('Nodes')}>{reservation.nodes}</Td>
                                    <Td dataLabel={_('Accounts')}>{reservation.accounts.join(', ')}</Td>
                                    <Td dataLabel={_('Users')}>{reservation.users.join(', ')}</Td>
                                    <Td dataLabel={_('Purpose')}>{reservation.purpose}</Td>
                                </Tr>
                            ))}
                        </Tbody>
                    </Table>
                </CardBody>
            </Card>
        </div>
    );
};

export const ReportsPage = () => {
    return (
        <div style={{ display: 'grid', gap: '1rem' }}>
            <SummaryCards
                items={REPORT_FIXTURES.map((report) => ({
                    title: report.title,
                    value: `${report.value}${report.title === 'Cluster utilization' ? '%' : ''}`,
                    description: `${report.delta} · ${report.description}`,
                }))}
            />

            <Card>
                <CardTitle>{_('Utilization trend')}</CardTitle>
                <CardBody>
                    <div style={{ display: 'grid', gap: '1rem' }}>
                        <Progress value={78} title={_('CPU utilization')} label={_('78%')} aria-label={_('CPU utilization')} />
                        <Progress value={63} title={_('Memory utilization')} label={_('63%')} aria-label={_('Memory utilization')} />
                        <Progress value={41} title={_('GPU utilization')} label={_('41%')} aria-label={_('GPU utilization')} />
                    </div>
                </CardBody>
            </Card>
        </div>
    );
};

export const SettingsPage = () => {
    const [maintenanceMode, setMaintenanceMode] = useState(ADMIN_SETTINGS.maintenanceMode);
    const [defaultAccount, setDefaultAccount] = useState(ADMIN_SETTINGS.defaultAccount);
    const [notificationEmail, setNotificationEmail] = useState(ADMIN_SETTINGS.notificationEmail);
    const [retentionDays, setRetentionDays] = useState(ADMIN_SETTINGS.retentionDays);
    const [usageReportsEnabled, setUsageReportsEnabled] = useState(ADMIN_SETTINGS.enableUsageReports);
    const [saved, setSaved] = useState(false);

    const hasChanges = useMemo(() => {
        return maintenanceMode !== ADMIN_SETTINGS.maintenanceMode ||
            defaultAccount !== ADMIN_SETTINGS.defaultAccount ||
            notificationEmail !== ADMIN_SETTINGS.notificationEmail ||
            retentionDays !== ADMIN_SETTINGS.retentionDays ||
            usageReportsEnabled !== ADMIN_SETTINGS.enableUsageReports;
    }, [defaultAccount, maintenanceMode, notificationEmail, retentionDays, usageReportsEnabled]);

    return (
        <Card>
            <CardTitle>{_('Cluster settings')}</CardTitle>
            <CardBody>
                {saved && (
                    <Alert variant="success" title={_('Settings prepared')}>
                        {_('The updated admin settings are ready for the bridge contract.')}
                    </Alert>
                )}

                <Form isHorizontal>
                    <FormGroup label={_('Maintenance mode')} fieldId="maintenance-mode">
                        <Switch
                            id="maintenance-mode"
                            label={_('Enabled')}
                            labelOff={_('Disabled')}
                            isChecked={maintenanceMode}
                            onChange={(_event, checked) => {
                                setMaintenanceMode(checked);
                                setSaved(false);
                            }}
                        />
                    </FormGroup>
                    <FormGroup label={_('Default account')} fieldId="default-account">
                        <TextInput
                            id="default-account"
                            value={defaultAccount}
                            onChange={(_event, value) => {
                                setDefaultAccount(value);
                                setSaved(false);
                            }}
                        />
                    </FormGroup>
                    <FormGroup label={_('Notification e-mail')} fieldId="notification-email">
                        <TextInput
                            id="notification-email"
                            value={notificationEmail}
                            onChange={(_event, value) => {
                                setNotificationEmail(value);
                                setSaved(false);
                            }}
                        />
                    </FormGroup>
                    <FormGroup label={_('Retention days')} fieldId="retention-days">
                        <TextInput
                            id="retention-days"
                            value={retentionDays}
                            onChange={(_event, value) => {
                                setRetentionDays(value);
                                setSaved(false);
                            }}
                        />
                    </FormGroup>
                    <FormGroup fieldId="usage-reports">
                        <Checkbox
                            id="usage-reports"
                            label={_('Enable scheduled usage reports')}
                            isChecked={usageReportsEnabled}
                            onChange={(_event, checked) => {
                                setUsageReportsEnabled(checked);
                                setSaved(false);
                            }}
                        />
                    </FormGroup>
                </Form>

                <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem' }}>
                    <Button variant="primary" onClick={() => setSaved(true)} isDisabled={!hasChanges}>
                        {_('Save settings')}
                    </Button>
                    <Button
                        variant="secondary"
                        onClick={() => {
                            setMaintenanceMode(ADMIN_SETTINGS.maintenanceMode);
                            setDefaultAccount(ADMIN_SETTINGS.defaultAccount);
                            setNotificationEmail(ADMIN_SETTINGS.notificationEmail);
                            setRetentionDays(ADMIN_SETTINGS.retentionDays);
                            setUsageReportsEnabled(ADMIN_SETTINGS.enableUsageReports);
                            setSaved(false);
                        }}
                    >
                        {_('Reset')}
                    </Button>
                </div>
            </CardBody>
        </Card>
    );
};
