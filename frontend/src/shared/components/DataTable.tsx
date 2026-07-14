import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from "@tanstack/react-table";
import { Inbox } from "lucide-react";

export function DataTable<T>({ data, columns, emptyTitle = "No records", emptyText = "Nothing matches this view." }: {
  data: T[];
  columns: ColumnDef<T>[];
  emptyTitle?: string;
  emptyText?: string;
}) {
  const table = useReactTable({ data, columns, getCoreRowModel: getCoreRowModel() });
  if (data.length === 0) {
    return (
      <div className="empty-state table-empty">
        <Inbox size={24} aria-hidden="true" />
        <strong>{emptyTitle}</strong>
        <span>{emptyText}</span>
      </div>
    );
  }
  return (
    <div className="table-scroll">
      <table className="data-table">
        <thead>
          {table.getHeaderGroups().map((group) => (
            <tr key={group.id}>
              {group.headers.map((header) => (
                <th key={header.id}>{header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}</th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
