import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { Ban, Network, Search } from "lucide-react";
import { useMemo, useState } from "react";
import type { Projection } from "../../shared/api/types";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { ConfirmDialog } from "../../shared/components/ConfirmDialog";
import { DataTable } from "../../shared/components/DataTable";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate, shortID, titleCase } from "../../shared/formatting";

export function ProjectionsPage() {
  const api = useApi(); const client = useQueryClient(); const [query, setQuery] = useState("");
  const projections = useQuery({ queryKey: ["projections", query], queryFn: () => api.listProjections({ q: query, limit: 100 }) });
  const revoke = useMutation({ mutationFn: ({ id, reason }: { id: string; reason: string }) => api.revokeProjection(id, reason), onSuccess: async () => { await Promise.all([client.invalidateQueries({ queryKey: ["projections"] }), client.invalidateQueries({ queryKey: ["summary"] })]); } });
  const columns = useMemo<ColumnDef<Projection>[]>(() => [
    { header: "Projection", cell: ({ row }) => <div className="primary-cell"><strong>{titleCase(row.original.type)}</strong><code>{shortID(row.original.projection_id, 14)}</code></div> },
    { header: "Status", cell: ({ row }) => <StatusBadge value={row.original.status} /> },
    { header: "Mission", cell: ({ row }) => <div className="stacked-cell"><strong>{shortID(row.original.mission_ref, 14)}</strong><span>Version {row.original.mission_version}</span></div> },
    { header: "Actor", cell: ({ row }) => <div className="stacked-cell"><strong>{row.original.actor.client_id}</strong><span>{row.original.actor.agent_instance_id}</span></div> },
    { header: "Expires", cell: ({ row }) => formatDate(row.original.expires_at) },
    { id: "actions", header: "", cell: ({ row }) => row.original.status === "active" ? <ConfirmDialog title="Revoke projection" description="The external credential will fail verification immediately." confirmLabel="Revoke projection" onConfirm={(reason) => revoke.mutateAsync({ id: row.original.projection_id, reason })} trigger={<button className="icon-button icon-button-danger" aria-label="Revoke projection"><Ban size={16} /></button>} /> : null },
  ], [revoke]);
  return <><PageHeader eyebrow="External authority" title="Projections" description="Short-lived mission authority projected into gateway credentials." /><div className="filter-bar"><label className="search-field"><Search size={16} /><span className="sr-only">Search projections</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search projection, mission, or actor" /></label><span className="result-count"><Network size={14} />{projections.data?.total ?? 0} projections</span></div><section className="content-section table-section">{projections.isLoading ? <LoadingState /> : projections.isError ? <ErrorState error={projections.error} /> : <DataTable data={projections.data!.items} columns={columns} emptyTitle="No projections" />}</section></>;
}
