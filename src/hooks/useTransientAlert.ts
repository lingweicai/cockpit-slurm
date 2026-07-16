import { useCallback, useEffect, useState } from 'react';

export type TransientAlert = {
    variant: 'success' | 'danger' | 'warning' | 'info';
    title: string;
};

export function useTransientAlert(timeoutMs = 3000) {
    const [alert, setAlert] = useState<TransientAlert | null>(null);

    useEffect(() => {
        if (!alert) {
            return;
        }

        const timer = window.setTimeout(() => {
            setAlert(null);
        }, timeoutMs);

        return () => {
            window.clearTimeout(timer);
        };
    }, [alert, timeoutMs]);

    const showAlert = useCallback((next: TransientAlert) => {
        setAlert(next);
    }, []);

    const clearAlert = useCallback(() => {
        setAlert(null);
    }, []);

    return {
        alert,
        showAlert,
        clearAlert,
    };
}
