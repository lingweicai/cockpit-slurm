import cockpit from 'cockpit';

import type { EntityTableRowAction } from '../components/EntityTable';
import type { TransientAlert } from '../hooks/useTransientAlert';
import { copyTextToClipboard } from './clipboard';

const _ = cockpit.gettext;

type RunCopyActionOptions = {
    value: string;
    successTitle: string;
    failureTitle: string;
    showAlert: (alert: TransientAlert) => void;
};

export async function runCopyAction({ value, successTitle, failureTitle, showAlert }: RunCopyActionOptions) {
    let copied = false;

    try {
        copied = await copyTextToClipboard(value);
    } catch {
        copied = false;
    }

    showAlert({
        variant: copied ? 'success' : 'danger',
        title: copied ? successTitle : failureTitle,
    });
}

type BuildDetailsRowActionOptions = {
    id?: string;
    label?: React.ReactNode;
    onClick: () => void;
};

export function buildDetailsRowAction<TRow>({
    id = 'details',
    label = _('Details'),
    onClick,
}: BuildDetailsRowActionOptions): EntityTableRowAction<TRow> {
    return {
        id,
        label,
        onClick: () => onClick(),
    };
}

type BuildToggleDetailsRowActionOptions = {
    id?: string;
    label?: React.ReactNode;
    onSelect: () => void;
    onToggle: () => void;
};

export function buildToggleDetailsRowAction<TRow>({
    id = 'details',
    label = _('Details'),
    onSelect,
    onToggle,
}: BuildToggleDetailsRowActionOptions): EntityTableRowAction<TRow> {
    return {
        id,
        label,
        onClick: () => {
            onSelect();
            onToggle();
        },
    };
}

type BuildCopyNameRowActionOptions = {
    id: string;
    label: React.ReactNode;
    value: string;
    successTitle: string;
    failureTitle: string;
    showAlert: (alert: TransientAlert) => void;
};

export function buildCopyNameRowAction<TRow>({
    id,
    label,
    value,
    successTitle,
    failureTitle,
    showAlert,
}: BuildCopyNameRowActionOptions): EntityTableRowAction<TRow> {
    return {
        id,
        label,
        onClick: () => {
            void runCopyAction({
                value,
                successTitle,
                failureTitle,
                showAlert,
            });
        },
    };
}