export type AppRole = 'user' | 'operator' | 'admin';

export type AppPageId =
    | 'dashboard'
    | 'cluster-overview'
    | 'partitions'
    | 'nodes'
    | 'jobs'
    | 'my-jobs'
    | 'submit-job'
    | 'my-reservations'
    | 'my-files'
    | 'users'
    | 'accounts'
    | 'qos'
    | 'reservations'
    | 'reports'
    | 'settings';

export type NavigationItem = {
    id: AppPageId;
    label: string;
    roles: AppRole[];
};

export const NAVIGATION_ITEMS: NavigationItem[] = [
    { id: 'dashboard', label: 'Dashboard', roles: ['user', 'operator', 'admin'] },
    { id: 'cluster-overview', label: 'Cluster Overview', roles: ['operator', 'admin'] },
    { id: 'partitions', label: 'Partitions', roles: ['operator', 'admin'] },
    { id: 'nodes', label: 'Nodes', roles: ['operator', 'admin'] },
    { id: 'jobs', label: 'Jobs Queue', roles: ['user', 'operator', 'admin'] },
    { id: 'my-jobs', label: 'My Jobs', roles: ['user'] },
    { id: 'submit-job', label: 'Submit Job', roles: ['user'] },
    { id: 'my-reservations', label: 'My Reservations', roles: ['user'] },
    { id: 'my-files', label: 'My Files', roles: ['user'] },
    { id: 'users', label: 'Users', roles: ['admin'] },
    { id: 'accounts', label: 'Accounts', roles: ['admin'] },
    { id: 'qos', label: 'QOS', roles: ['admin'] },
    { id: 'reservations', label: 'Reservations', roles: ['operator', 'admin'] },
    { id: 'reports', label: 'Reports', roles: ['admin'] },
    { id: 'settings', label: 'Settings', roles: ['admin'] },
];

export function getRoleOverride(): AppRole | null {
    const globalAny = globalThis as { COCKPIT_SLURM_ROLE?: unknown };
    const role = globalAny.COCKPIT_SLURM_ROLE;
    if (role === 'admin' || role === 'operator' || role === 'user') {
        return role;
    }

    return null;
}

export function getCurrentRole(): AppRole {
    return getRoleOverride() ?? 'user';
}

export function getNavigationItems(role: AppRole): NavigationItem[] {
    return NAVIGATION_ITEMS.filter((item) => item.roles.includes(role));
}

export function getDefaultPageId(role: AppRole): AppPageId {
    if (role === 'admin') {
        return 'dashboard';
    }

    if (role === 'operator') {
        return 'dashboard';
    }

    return 'dashboard';
}

export function normalizePageId(value: string | null | undefined, role: AppRole): AppPageId {
    const allowed = new Set(getNavigationItems(role).map((item) => item.id));
    if (value && allowed.has(value as AppPageId)) {
        return value as AppPageId;
    }

    return getDefaultPageId(role);
}

export function getPageTitle(pageId: AppPageId): string {
    const item = NAVIGATION_ITEMS.find((entry) => entry.id === pageId);
    return item?.label ?? 'Dashboard';
}

export function getBreadcrumbTrail(role: AppRole, pageId: AppPageId): string[] {
    const page = getPageTitle(pageId);
    const roleLabel = role.charAt(0).toUpperCase() + role.slice(1);
    if (pageId === 'dashboard') {
        return [roleLabel, page];
    }

    return [roleLabel, page];
}
