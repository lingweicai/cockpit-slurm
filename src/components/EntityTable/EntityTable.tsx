import React, { useMemo, useState } from 'react';
import { Dropdown, DropdownItem, MenuToggle, Pagination, TextInput } from '@patternfly/react-core';
import { ArrowDownIcon, ArrowUpIcon, ArrowsAltVIcon, EllipsisVIcon } from '@patternfly/react-icons';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';

import './EntityTable.css';

export type EntityTableColumn<TRow> = {
    header: React.ReactNode;
    dataLabel: string;
    cell: (row: TRow) => React.ReactNode;
    sortable?: {
        isActive: boolean;
        direction: 'asc' | 'desc';
        onSort: () => void;
    };
};

export type EntityTableRowAction<TRow> = {
    id: string;
    label: React.ReactNode;
    onClick: (row: TRow) => void;
};

type EntityTableProps<TRow> = {
    ariaLabel: string;
    columns: EntityTableColumn<TRow>[];
    rows: TRow[];
    rowKey: (row: TRow) => string;
    toolbar?: React.ReactNode;
    onRowClick?: (row: TRow) => void;
    selectedRowKey?: string | null;
    variant?: 'compact';
    expandable?: {
        expandedRowKey: string | null;
        onToggle: (row: TRow, rowKey: string) => void;
        renderExpandedContent: (row: TRow) => React.ReactNode;
    };
    pagination?: {
        page?: number;
        perPage?: number;
        onSetPage?: (page: number) => void;
        onPerPageSelect?: (perPage: number) => void;
        defaultPerPage?: number;
        perPageOptions?: number[];
    };
    emptyState?: React.ReactNode;
    rowActions?: (row: TRow) => React.ReactNode;
    rowActionItems?: (row: TRow) => EntityTableRowAction<TRow>[];
    rowActionsVariant?: 'inline' | 'menu';
    filter?: {
        placeholder: string;
        matches: (row: TRow, query: string) => boolean;
        defaultQuery?: string;
        query?: string;
        onQueryChange?: (query: string) => void;
        emptyState?: React.ReactNode;
    };
};

export function EntityTable<TRow>({
    ariaLabel,
    columns,
    rows,
    rowKey,
    toolbar,
    onRowClick,
    selectedRowKey,
    variant = 'compact',
    expandable,
    pagination,
    emptyState,
    rowActions,
    rowActionItems,
    rowActionsVariant = 'inline',
    filter,
}: EntityTableProps<TRow>) {
    const [internalPage, setInternalPage] = useState(1);
    const [internalPerPage, setInternalPerPage] = useState(pagination?.defaultPerPage ?? 10);
    const [openActionMenuRowKey, setOpenActionMenuRowKey] = useState<string | null>(null);
    const [internalFilterQuery, setInternalFilterQuery] = useState(filter?.defaultQuery ?? '');

    const hasPagination = Boolean(pagination);
    const hasActionMenu = rowActionsVariant === 'menu' && typeof rowActionItems === 'function';
    const hasInlineActions = rowActionsVariant === 'inline' && typeof rowActions === 'function';
    const hasActionsColumn = hasActionMenu || hasInlineActions;
    const filterQuery = filter?.query ?? internalFilterQuery;
    const filteredRows = useMemo(() => {
        if (!filter || !filterQuery.trim()) {
            return rows;
        }

        return rows.filter((row) => filter.matches(row, filterQuery));
    }, [filter, filterQuery, rows]);

    const currentPerPage = pagination?.perPage ?? internalPerPage;
    const maxPage = Math.max(1, Math.ceil(filteredRows.length / Math.max(currentPerPage, 1)));
    const resolvedPage = Math.min(pagination?.page ?? internalPage, maxPage);
    const perPageOptions = pagination?.perPageOptions ?? [10, 20, 50, 100];

    const pagedRows = useMemo(() => {
        if (!hasPagination) {
            return filteredRows;
        }

        const startIndex = (resolvedPage - 1) * currentPerPage;
        return filteredRows.slice(startIndex, startIndex + currentPerPage);
    }, [currentPerPage, filteredRows, hasPagination, resolvedPage]);

    if (rows.length === 0) {
        return <>{emptyState ?? null}</>;
    }

    if (filteredRows.length === 0) {
        return <>{filter?.emptyState ?? emptyState ?? null}</>;
    }

    const onSetPage = (page: number) => {
        if (pagination?.onSetPage) {
            pagination.onSetPage(page);
            return;
        }

        setInternalPage(page);
    };

    const onPerPageSelect = (perPage: number) => {
        if (pagination?.onPerPageSelect) {
            pagination.onPerPageSelect(perPage);
            return;
        }

        setInternalPerPage(perPage);
        setInternalPage(1);
    };

    const onFilterQueryChange = (value: string) => {
        if (filter?.onQueryChange) {
            filter.onQueryChange(value);
        } else {
            setInternalFilterQuery(value);
        }

        setInternalPage(1);
    };

    function renderSortIcon(isActive: boolean, direction: 'asc' | 'desc') {
        if (!isActive) {
            return <ArrowsAltVIcon aria-hidden="true" />;
        }

        if (direction === 'asc') {
            return <ArrowUpIcon aria-hidden="true" />;
        }

        return <ArrowDownIcon aria-hidden="true" />;
    }

    return (
        <div style={{ display: 'grid', gap: '0.75rem' }}>
            {(filter || toolbar) && (
                <div className="entity-table__toolbar">
                    {filter && (
                        <div className="entity-table__toolbar-filter">
                            <TextInput
                                value={filterQuery}
                                onChange={(_event, value) => onFilterQueryChange(value)}
                                aria-label={filter.placeholder}
                                placeholder={filter.placeholder}
                            />
                        </div>
                    )}
                    {toolbar && <div className="entity-table__toolbar-content">{toolbar}</div>}
                </div>
            )}
            <Table aria-label={ariaLabel} variant={variant}>
                <Thead>
                    <Tr>
                        {expandable && <Th screenReaderText="Expand row" />}
                        {columns.map((column) => (
                            <Th key={column.dataLabel}>
                                {column.sortable
                                    ? (
                                        <button
                                            className="entity-table__sort-button"
                                            type="button"
                                            onClick={column.sortable.onSort}
                                            aria-label={`${String(column.header)} ${column.sortable.isActive ? `sorted ${column.sortable.direction}` : 'sortable'}`}
                                        >
                                            <span>{column.header}</span>
                                            <span className={`entity-table__sort-icon ${column.sortable.isActive ? 'entity-table__sort-icon--active' : ''}`}>
                                                {renderSortIcon(column.sortable.isActive, column.sortable.direction)}
                                            </span>
                                        </button>
                                    )
                                    : column.header}
                            </Th>
                        ))}
                        {hasActionsColumn && <Th textCenter screenReaderText="Row actions" />}
                    </Tr>
                </Thead>
                <Tbody>
                    {pagedRows.map((row) => {
                        const key = rowKey(row);
                        const isExpanded = expandable?.expandedRowKey === key;
                        const isSelected = selectedRowKey === key;

                        return (
                            <React.Fragment key={key}>
                                <Tr
                                    className={isSelected ? 'entity-table__row--selected' : undefined}
                                    aria-selected={isSelected}
                                    onClick={onRowClick ? () => onRowClick(row) : undefined}
                                    style={onRowClick ? { cursor: 'pointer' } : undefined}
                                >
                                    {expandable && (
                                        <Td
                                            dataLabel="Expand"
                                            width={10}
                                            onClick={(event) => {
                                                event.stopPropagation();
                                                expandable.onToggle(row, key);
                                            }}
                                        >
                                            {isExpanded ? '-' : '+'}
                                        </Td>
                                    )}
                                    {columns.map((column) => (
                                        <Td key={column.dataLabel} dataLabel={column.dataLabel}>
                                            {column.cell(row)}
                                        </Td>
                                    ))}
                                    {hasActionsColumn && (
                                        <Td
                                            className="entity-table__actions-cell"
                                            dataLabel="Actions"
                                            textCenter
                                            onClick={(event) => event.stopPropagation()}
                                        >
                                            {hasActionMenu && rowActionItems ? (
                                                <Dropdown
                                                    isOpen={openActionMenuRowKey === key}
                                                    onSelect={() => setOpenActionMenuRowKey(null)}
                                                    isPlain
                                                    toggle={(toggleRef) => (
                                                        <MenuToggle
                                                            ref={toggleRef}
                                                            variant="plain"
                                                            aria-label="Actions"
                                                            isExpanded={openActionMenuRowKey === key}
                                                            onClick={() => {
                                                                setOpenActionMenuRowKey((current) => (current === key ? null : key));
                                                            }}
                                                        >
                                                            <EllipsisVIcon />
                                                        </MenuToggle>
                                                    )}
                                                    dropdownItems={rowActionItems(row).map((action) => (
                                                        <DropdownItem
                                                            key={action.id}
                                                            onClick={() => {
                                                                action.onClick(row);
                                                                setOpenActionMenuRowKey(null);
                                                            }}
                                                        >
                                                            {action.label}
                                                        </DropdownItem>
                                                    ))}
                                                />
                                            ) : rowActions ? rowActions(row) : null}
                                        </Td>
                                    )}
                                </Tr>
                                {isExpanded && expandable && (
                                    <Tr isExpanded>
                                        <Td colSpan={columns.length + (expandable ? 1 : 0) + (hasActionsColumn ? 1 : 0)}>
                                            {expandable.renderExpandedContent(row)}
                                        </Td>
                                    </Tr>
                                )}
                            </React.Fragment>
                        );
                    })}
                </Tbody>
            </Table>
            {hasPagination && (
                <Pagination
                    itemCount={filteredRows.length}
                    page={resolvedPage}
                    perPage={currentPerPage}
                    perPageOptions={perPageOptions.map((option) => ({ title: String(option), value: option }))}
                    onSetPage={(_event, page) => onSetPage(page)}
                    onPerPageSelect={(_event, perPage) => onPerPageSelect(perPage)}
                />
            )}
        </div>
    );
}
