import cockpit from 'cockpit';

export function getCurrentUserName(): string {
    return cockpit.info?.user?.name?.trim() || 'alice';
}

export function getCurrentUserLabel(): string {
    return cockpit.info?.user?.name?.trim() || cockpit.info?.user?.uid?.toString() || 'alice';
}
