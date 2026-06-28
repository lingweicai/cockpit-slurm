import React from 'react';
import {
    Breadcrumb,
    BreadcrumbItem,
    Button,
    Nav,
    NavItem,
    NavList,
    Page,
    PageSection,
    PageSidebar,
    Title,
} from '@patternfly/react-core';

import cockpit from 'cockpit';

import type { AppPageId, AppRole, NavigationItem } from '../../app/navigation';
import { ConnectionBanner } from '../ConnectionBanner';

const _ = cockpit.gettext;

type AppShellProps = {
    role: AppRole;
    pageId: AppPageId;
    navigationItems: NavigationItem[];
    breadcrumbs: string[];
    activeCluster: string;
    clusterOptions: string[];
    onClusterChange: (clusterName: string) => void;
    onRefresh: () => void;
    onNavigate: (pageId: AppPageId) => void;
    children: React.ReactNode;
};

export const AppShell = ({
    role,
    pageId,
    navigationItems,
    breadcrumbs,
    activeCluster,
    clusterOptions,
    onClusterChange,
    onRefresh,
    onNavigate,
    children,
}: AppShellProps) => {
    return (
        <Page
            sidebar={(
                <PageSidebar>
                    <Nav aria-label={_('Primary navigation')}>
                        <NavList>
                            {navigationItems.map((item) => (
                                <NavItem
                                    key={item.id}
                                    itemId={item.id}
                                    isActive={item.id === pageId}
                                    onClick={() => onNavigate(item.id)}
                                >
                                    {item.label}
                                </NavItem>
                            ))}
                        </NavList>
                    </Nav>
                </PageSidebar>
            )}
        >
            <PageSection variant="light">
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem', alignItems: 'flex-end', flexWrap: 'wrap' }}>
                    <div>
                        <Title headingLevel="h1">{_('cockpit-slurm')}</Title>
                        <p>{cockpit.format(_('Current role: $0'), role)}</p>
                        <p>{cockpit.format(_('Active cluster: $0'), activeCluster)}</p>
                    </div>
                    <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'end', flexWrap: 'wrap' }}>
                        <label>
                            <div>{_('Cluster')}</div>
                            <select value={activeCluster} onChange={(event) => onClusterChange(event.target.value)}>
                                {clusterOptions.map((clusterName) => (
                                    <option key={clusterName} value={clusterName}>{clusterName}</option>
                                ))}
                            </select>
                        </label>
                        <Button variant="secondary" onClick={onRefresh}>
                            {_('Refresh bridge cache')}
                        </Button>
                    </div>
                </div>
            </PageSection>

            <PageSection>
                <Breadcrumb>
                    {breadcrumbs.map((item) => (
                        <BreadcrumbItem key={item} isActive={item === breadcrumbs[breadcrumbs.length - 1]}>
                            {item}
                        </BreadcrumbItem>
                    ))}
                </Breadcrumb>
                <ConnectionBanner />
                {children}
            </PageSection>
        </Page>
    );
};
