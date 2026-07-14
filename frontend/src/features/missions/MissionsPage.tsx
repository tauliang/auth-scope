import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { ArrowRight, Plus, Search } from "lucide-react";
import { useMemo, useState } from "react";
import type { Mission } from "../../shared/api/types";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { DataTable } from "../../shared/components/DataTable";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate, shortID } from "../../shared/formatting";

export function MissionsPage() {
  const api = useApi();
  const [query, setQuery] = useState("");
  const [state, setState] = useState("");
  const missions = useQuery({ queryKey: ["missions", query, state], queryFn: () => api.listMissions({ q: query, state, limit: 100 }) });
  const columns = useMemo<ColumnDef<Mission>[]>(() => [
    { header: "Mission", cell: ({ row }) => <div className="primary-cell"><strong>{row.original.purpose.objective}</strong><code>{shortID(row.original.mission_ref, 14)}</code></div> },
    { header: "State", accessorKey: "state", cell: ({ getValue }) => <StatusBadge value={String(getValue())} /> },
    { header: "Principal", cell: ({ row }) => <div className="stacked-cell"><strong>{row.original.principal.subject}</strong><span>{row.original.tenant_id}</span></div> },
    { header: "Agent", cell: ({ row }) => <div className="stacked-cell"><strong>{row.original.agent.client_id}</strong><span>{row.original.agent.instance_id}</span></div> },
    { header: "Version", accessorKey: "version", cell: ({ getValue }) => <span className="version-chip">v{String(getValue())}</span> },
    { header: "Expires", cell: ({ row }) => formatDate(row.original.lifecycle.expires_at) },
    { id: "open", header: "", cell: ({ row }) => <Link className="icon-button" aria-label={`Open ${row.original.purpose.objective}`} to="/missions/$missionRef" params={{ missionRef: row.original.mission_ref }}><ArrowRight size={17} /></Link> },
  ], []);

  return (
    <>
      <PageHeader eyebrow="Authority inventory" title="Missions" description="Inspect effective authority, lifecycle, delegation, and evidence." actions={<Link to="/missions/new" className="button button-primary"><Plus size={16} />New proposal</Link>} />
      <div className="filter-bar">
        <label className="search-field"><Search size={16} /><span className="sr-only">Search missions</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search mission, objective, principal, or agent" /></label>
        <label className="select-field"><span className="sr-only">Filter by state</span><select value={state} onChange={(event) => setState(event.target.value)}><option value="">All states</option><option value="active">Active</option><option value="suspended">Suspended</option><option value="completed">Completed</option><option value="revoked">Revoked</option><option value="expired">Expired</option></select></label>
        <span className="result-count">{missions.data?.total ?? 0} missions</span>
      </div>
      <section className="content-section table-section">
        {missions.isLoading ? <LoadingState label="Loading missions" /> : missions.isError ? <ErrorState error={missions.error} onRetry={() => missions.refetch()} /> : <DataTable data={missions.data!.items} columns={columns} emptyTitle="No missions found" />}
      </section>
    </>
  );
}
