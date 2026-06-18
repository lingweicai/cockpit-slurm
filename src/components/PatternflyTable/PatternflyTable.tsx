import React, { useMemo, useState } from 'react';
import {
  Table,
  TableHeader,
  TableBody,
  IRow,
  ICell
} from '@patternfly/react-table';
import { Button, KebabToggle, Dropdown, DropdownItem, Pagination } from '@patternfly/react-core';

export type Column<T> = {
  title: string;
  cell: (item: T) => React.ReactNode;
};

export type RowAction<T> = {
  title: string;
  onClick: (item: T) => void;
};

export type PaginationState = {
  itemCount: number;
  perPage: number;
  page: number;
  onSetPage: (event: any, page: number) => void;
  onPerPageSelect: (event: any, perPage: number) => void;
};

type Props<T> = {
  data: T[];
  columns: Column<T>[];
  renderDetails?: (item: T) => React.ReactNode;
  actions?: (item: T) => RowAction<T>[];
  getRowId?: (item: T, index: number) => string;
  pagination?: PaginationState;
  emptyState?: React.ReactNode;
};

function PatternflyTable<T>({
  data,
  columns,
  renderDetails,
  actions,
  getRowId,
  pagination,
  emptyState
}: Props<T>) {
  const [expandedMap, setExpandedMap] = useState<Record<string, boolean>>({});

  const idFor = (item: T, idx: number) => (getRowId ? getRowId(item, idx) : String(idx));

  const headers = useMemo(() => {
    const h: ICell[] = columns.map(c => ({ title: c.title }));
    if (actions) h.push({ title: '' });
    return h;
  }, [columns, actions]);

  const rows: IRow[] = [];

  data.forEach((item, idx) => {
    const id = idFor(item, idx);
    const isOpen = !!expandedMap[id];
    const cells: ICell[] = columns.map(c => ({ title: c.cell(item) }));

    if (actions) {
      // Render kebab/dropdown for actions
      const actionItems = actions(item);
      const actionNode = (
        <Dropdown
          onSelect={() => {}}
          toggle={<KebabToggle onToggle={() => {}} />}
          isPlain
          dropdownItems={actionItems.map((a, i) => (
            <DropdownItem key={i} onClick={() => a.onClick(item)}>
              {a.title}
            </DropdownItem>
          ))}
        />
      );
      cells.push({ title: actionNode });
    }

    // Main row
    rows.push({ isOpen, cells });

    // If expanded, push a details row that spans all columns
    if (isOpen && renderDetails) {
      const colSpan = columns.length + (actions ? 1 : 0);
      rows.push({
        cells: [
          {
            title: <div style={{ padding: 8 }}>{renderDetails(item)}</div>,
            props: { colSpan }
          }
        ]
      });
    }
  });

  const onCollapse = (_event: any, rowKey: number, isOpen: boolean) => {
    // rowKey refers to the index in the rows array; map back to data index
    // because expanded details insert extra rows, compute which data index this corresponds to
    // Walk rows up to rowKey and count the main rows to find their data index
    let mainRowCount = -1;
    let i = 0;
    for (i = 0; i <= rowKey && i < rows.length; i++) {
      // A main row is one without props or with cells length equal to headers length
      const maybe = rows[i];
      // We assumed main rows were added first then optional details row after them.
      // So every time we encounter a row whose cells length equals headers length, count it as main
      if (maybe.cells.length === headers.length) mainRowCount++;
    }
    const dataIndex = mainRowCount;
    if (dataIndex < 0 || dataIndex >= data.length) return;
    const id = idFor(data[dataIndex], dataIndex);
    setExpandedMap(prev => ({ ...prev, [id]: isOpen }));
  };

  if (data.length === 0) {
    return <div>{emptyState ?? 'No data'}</div>;
  }

  return (
    <div>
      <Table aria-label="Slurm table" cells={headers} rows={rows} onCollapse={onCollapse}>
        <TableHeader />
        <TableBody />
      </Table>
      {pagination ? (
        <Pagination
          itemCount={pagination.itemCount}
          perPage={pagination.perPage}
          page={pagination.page}
          onSetPage={pagination.onSetPage}
          onPerPageSelect={pagination.onPerPageSelect}
        />
      ) : null}
    </div>
  );
}

export default PatternflyTable;
