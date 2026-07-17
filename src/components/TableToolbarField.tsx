import React from 'react';

type TableToolbarFieldProps = {
    label: React.ReactNode;
    children: React.ReactNode;
};

export const TableToolbarField = ({ label, children }: TableToolbarFieldProps) => {
    return (
        <label>
            <div>{label}</div>
            {children}
        </label>
    );
};
