import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { ArrowRight, Search } from "lucide-react";
import { useMemo, useState } from "react";
import type { AgentIdentity } from "../../shared/api/types";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { DataTable } from "../../shared/components/DataTable";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate, shortID } from "../../shared/formatting";

export function AgentsPage() {
  const api = useApi();
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("");
  const agents = useQuery({ queryKey: ["agents", query, status], queryFn: () => api.listAgents({ q: query, status, limit: 100 }) });
  const columns = useMemo<ColumnDef<AgentIdentity>[]>(() => [
    { header: "Identity", cell: ({ row }) => <div className="primary-cell"><strong>{row.original.agent.client_id}</strong><code>{shortID(row.original.agent_id, 14)}</code></div> },
    { header: "Status", cell: ({ row }) => <StatusBadge value={row.original.status} /> },
    { header: "Instance", cell: ({ row }) => <div className="stacked-cell"><strong>{row.original.agent.instance_id}</strong><span>{row.original.agent.provider}</span></div> },
    { header: "Tenant", accessorKey: "tenant_id" },
    { header: "Key binding", cell: ({ row }) => <code>{shortID(row.original.key_thumbprint, 16)}</code> },
    { header: "Registered", cell: ({ row }) => formatDate(row.original.created_at) },
    { id: "open", header: "", cell: ({ row }) => <Link className="icon-button" aria-label={`Open ${row.original.agent.client_id}`} to="/agents/$agentId" params={{ agentId: row.original.agent_id }}><ArrowRight size={17} /></Link> },
  ], []);
  return <><PageHeader eyebrow="Workload identity" title="Agents" description="Registered agent instances and their cryptographic authority bindings." /><div className="filter-bar"><label className="search-field"><Search size={16} /><span className="sr-only">Search agents</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search agent, instance, provider, or key" /></label><label className="select-field"><span className="sr-only">Filter status</span><select value={status} onChange={(event) => setStatus(event.target.value)}><option value="">All statuses</option><option value="active">Active</option><option value="revoked">Revoked</option></select></label><span className="result-count">{agents.data?.total ?? 0} agents</span></div><section className="content-section table-section">{agents.isLoading ? <LoadingState label="Loading agents" /> : agents.isError ? <ErrorState error={agents.error} /> : <DataTable data={agents.data!.items} columns={columns} emptyTitle="No agents found" />}</section></>;
}
