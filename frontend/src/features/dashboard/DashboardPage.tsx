import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Activity, ArrowRight, Bot, Clock3, Network, OctagonAlert, ScrollText, ShieldCheck } from "lucide-react";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatRelative, shortID, titleCase } from "../../shared/formatting";

export function DashboardPage() {
  const api = useApi();
  const summary = useQuery({ queryKey: ["summary"], queryFn: () => api.getSummary() });
  const events = useQuery({ queryKey: ["events", { limit: 8 }], queryFn: () => api.listEvents({ limit: 8 }) });
  const expansions = useQuery({ queryKey: ["expansions", "pending"], queryFn: () => api.listExpansions({ status: "pending", limit: 5 }) });

  if (summary.isLoading) return <LoadingState label="Loading operations" />;
  if (summary.isError) return <ErrorState error={summary.error} onRetry={() => summary.refetch()} />;
  const data = summary.data!;
  const metrics = [
    { label: "Active missions", value: data.missions_by_state.active ?? 0, icon: ShieldCheck, tone: "green" },
    { label: "Pending approvals", value: data.pending_proposals + data.pending_expansions, icon: ScrollText, tone: "amber" },
    { label: "Active containment", value: data.active_containments, icon: OctagonAlert, tone: "red" },
    { label: "Registered agents", value: data.active_agents, icon: Bot, tone: "blue" },
    { label: "Live projections", value: data.active_projections, icon: Network, tone: "violet" },
  ];

  return (
    <>
      <PageHeader eyebrow="Operations" title="Authority overview" description="Current work requiring human attention across the mission authority boundary." />
      <section className="metric-grid" aria-label="Authority summary">
        {metrics.map(({ label, value, icon: Icon, tone }) => (
          <div className="metric" key={label}>
            <div className={`metric-icon metric-${tone}`}><Icon size={19} /></div>
            <div><strong>{value}</strong><span>{label}</span></div>
          </div>
        ))}
      </section>

      <div className="dashboard-grid">
        <section className="content-section attention-section">
          <div className="section-heading"><div><span className="section-kicker">Action queue</span><h2>Pending expansions</h2></div><Link to="/approvals" className="text-link">Open queue <ArrowRight size={15} /></Link></div>
          {expansions.isLoading ? <LoadingState /> : expansions.isError ? <ErrorState error={expansions.error} /> : expansions.data!.items.length ? (
            <div className="work-list">
              {expansions.data!.items.map((item) => (
                <Link key={item.expansion_id} to="/approvals/expansions/$expansionId" params={{ expansionId: item.expansion_id }} className="work-row">
                  <div className="work-icon"><Clock3 size={17} /></div>
                  <div className="work-main"><strong>{item.action.operation} on {item.action.resource.id}</strong><span>{shortID(item.mission_ref)} · {item.justification || "No justification supplied"}</span></div>
                  <StatusBadge value={item.status} />
                  <ArrowRight size={16} className="row-arrow" />
                </Link>
              ))}
            </div>
          ) : <div className="compact-empty"><ShieldCheck size={19} /><span>No expansion decisions waiting.</span></div>}
        </section>

        <section className="content-section activity-section">
          <div className="section-heading"><div><span className="section-kicker">Ledger</span><h2>Recent authority events</h2></div><Link to="/audit" className="text-link">View audit <ArrowRight size={15} /></Link></div>
          {events.isLoading ? <LoadingState /> : events.isError ? <ErrorState error={events.error} /> : (
            <div className="timeline">
              {events.data!.items.map((event) => (
                <div className="timeline-row" key={event.event_id}>
                  <div className="timeline-dot"><Activity size={13} /></div>
                  <div><strong>{titleCase(event.type)}</strong><span>{event.mission_ref ? shortID(event.mission_ref) : "System"} · {formatRelative(event.occurred_at)}</span></div>
                </div>
              ))}
              {!events.data!.items.length ? <div className="compact-empty">No events recorded.</div> : null}
            </div>
          )}
        </section>
      </div>

      <section className="state-strip" aria-label="Mission state distribution">
        <div><span className="section-kicker">Mission posture</span><strong>{data.missions_total} total</strong></div>
        {Object.entries(data.missions_by_state).map(([state, count]) => <div className="state-strip-item" key={state}><StatusBadge value={state} /><strong>{count}</strong></div>)}
      </section>
    </>
  );
}
