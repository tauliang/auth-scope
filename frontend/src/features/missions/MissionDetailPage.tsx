import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft, Ban, CheckCircle2, Clock3, Network, ShieldCheck, UserRound } from "lucide-react";
import { useState } from "react";
import { useApi } from "../../shared/auth/SessionProvider";
import { AuthorityView } from "../../shared/components/AuthorityView";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { ConfirmDialog } from "../../shared/components/ConfirmDialog";
import { JsonBlock } from "../../shared/components/JsonBlock";
import { LineageGraph } from "../../shared/components/LineageGraph";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate, formatRelative, titleCase } from "../../shared/formatting";

export function MissionDetailPage() {
  const { missionRef } = useParams({ from: "/missions/$missionRef" });
  const api = useApi();
  const client = useQueryClient();
  const [tab, setTab] = useState<"overview" | "authority" | "lineage" | "events" | "raw">("overview");
  const mission = useQuery({ queryKey: ["mission", missionRef], queryFn: () => api.getMission(missionRef) });
  const lineage = useQuery({ queryKey: ["mission-lineage", missionRef], queryFn: () => api.missionLineage(missionRef), enabled: tab === "lineage" });
  const events = useQuery({ queryKey: ["events", { missionRef }], queryFn: () => api.listEvents({ q: missionRef, limit: 100 }), enabled: tab === "events" });
  const transition = useMutation({
    mutationFn: ({ action, reason }: { action: "revoke" | "complete"; reason: string }) => api.transitionMission(missionRef, action, reason),
    onSuccess: async () => { await Promise.all([client.invalidateQueries({ queryKey: ["mission", missionRef] }), client.invalidateQueries({ queryKey: ["missions"] }), client.invalidateQueries({ queryKey: ["summary"] })]); },
  });

  if (mission.isLoading) return <LoadingState label="Loading mission" />;
  if (mission.isError) {
    return (
      <>
        <PageHeader
          eyebrow="Mission detail"
          title="Mission unavailable"
          description="The requested mission could not be loaded. Return to the inventory or retry the request."
          actions={<Link to="/missions" className="button button-secondary"><ArrowLeft size={16} />Missions</Link>}
        />
        <ErrorState error={mission.error} onRetry={() => mission.refetch()} />
      </>
    );
  }
  const data = mission.data!;
  const canTransition = data.state === "active" || data.state === "suspended";
  return (
    <>
      <PageHeader eyebrow={`${data.tenant_id} · mission v${data.version}`} title={data.purpose.objective} description={data.purpose.business_context || data.mission_ref} actions={<><Link to="/missions" className="button button-secondary"><ArrowLeft size={16} />Missions</Link>{canTransition ? <ConfirmDialog title="Complete mission" description="Completion is terminal and cascades according to the mission delegation policy." confirmLabel="Complete mission" tone="primary" onConfirm={(reason) => transition.mutateAsync({ action: "complete", reason })} trigger={<button className="button button-secondary"><CheckCircle2 size={16} />Complete</button>} /> : null}{canTransition ? <ConfirmDialog title="Revoke mission" description="Revocation removes authority immediately and may cascade to child missions." confirmLabel="Revoke mission" onConfirm={(reason) => transition.mutateAsync({ action: "revoke", reason })} trigger={<button className="button button-danger"><Ban size={16} />Revoke</button>} /> : null}</>} />
      <div className="entity-status-line"><StatusBadge value={data.state} /><code>{data.mission_ref}</code><span>Version {data.version}</span><span>Expires {formatDate(data.lifecycle.expires_at)}</span></div>
      <div className="tabs" role="tablist" aria-label="Mission details">
        {(["overview", "authority", "lineage", "events", "raw"] as const).map((value) => <button role="tab" aria-selected={tab === value} className={tab === value ? "tab-active" : ""} onClick={() => setTab(value)} key={value}>{titleCase(value)}</button>)}
      </div>
      {tab === "overview" ? <div className="detail-grid">
        <section className="content-section"><div className="section-heading"><h2>Identity binding</h2></div><dl className="definition-grid"><div><dt><UserRound size={15} />Principal</dt><dd>{data.principal.subject}</dd><small>{data.principal.issuer}</small></div><div><dt><Network size={15} />Agent</dt><dd>{data.agent.client_id}</dd><small>{data.agent.instance_id}</small></div><div><dt><ShieldCheck size={15} />Tenant</dt><dd>{data.tenant_id}</dd></div><div><dt><Clock3 size={15} />Created</dt><dd>{formatDate(data.lifecycle.created_at)}</dd></div></dl></section>
        <section className="content-section"><div className="section-heading"><h2>Delegation</h2></div><dl className="definition-grid"><div><dt>Permitted</dt><dd>{data.delegation.permitted ? "Yes" : "No"}</dd></div><div><dt>Depth</dt><dd>{data.delegation.current_depth} / {data.delegation.max_depth}</dd></div><div><dt>Attenuation</dt><dd>{titleCase(data.delegation.attenuation || "strict_subset")}</dd></div><div><dt>Cascade</dt><dd>{data.delegation.cascade_revocation ? "Enabled" : "Disabled"}</dd></div></dl></section>
        <section className="content-section detail-span"><div className="section-heading"><h2>Conditions</h2></div>{data.conditions?.length ? <div className="condition-list">{data.conditions.map((condition) => <div key={condition.id}><code>{condition.expression}</code><span>{condition.evaluation || "per action"} · failure {condition.on_failure || "deny"}</span></div>)}</div> : <div className="compact-empty">No conditional authority checks.</div>}</section>
      </div> : null}
      {tab === "authority" ? <section className="content-section"><AuthorityView authority={data.authority_region} /></section> : null}
      {tab === "lineage" ? lineage.isLoading ? <LoadingState label="Loading lineage" /> : lineage.isError ? <ErrorState error={lineage.error} /> : <LineageGraph graph={lineage.data!} /> : null}
      {tab === "events" ? events.isLoading ? <LoadingState label="Loading mission events" /> : events.isError ? <ErrorState error={events.error} /> : <section className="content-section"><div className="event-list">{events.data!.items.map((event) => <div className="event-row" key={event.event_id}><div><strong>{titleCase(event.type)}</strong><span>{formatRelative(event.occurred_at)}</span></div><code>{event.event_id}</code></div>)}</div></section> : null}
      {tab === "raw" ? <JsonBlock value={data} label="Mission record" /> : null}
    </>
  );
}
