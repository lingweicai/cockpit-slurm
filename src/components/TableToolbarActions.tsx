import React from 'react';

type TableToolbarActionsProps = {
    children: React.ReactNode;
};

export const TableToolbarActions = ({ children }: TableToolbarActionsProps) => {
    return <div className="entity-table__toolbar-actions">{children}</div>;
};
