import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { ArrowRight, Clock3, FileCheck2, ShieldAlert } from "lucide-react";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatRelative, shortID } from "../../shared/formatting";

export function ApprovalsPage() {
  const api = useApi();
  const proposals = useQuery({ queryKey: ["proposals", "pending"], queryFn: () => api.listProposals({ state: "pending_approval", limit: 100 }) });
  const expansions = useQuery({ queryKey: ["expansions", "pending"], queryFn: () => api.listExpansions({ status: "pending", limit: 100 }) });
  return (
    <>
      <PageHeader eyebrow="Human authority" title="Approval queue" description="Review requested authority and its consequences before it becomes effective." />
      <div className="approval-columns">
        <section className="content-section">
          <div className="section-heading"><div><span className="section-kicker">Mission creation</span><h2>Proposals</h2></div><span className="count-badge">{proposals.data?.total ?? 0}</span></div>
          {proposals.isLoading ? <LoadingState /> : proposals.isError ? <ErrorState error={proposals.error} /> : <div className="review-list">{proposals.data!.items.map((item) => <Link key={item.proposal_id} to="/approvals/proposals/$proposalId" params={{ proposalId: item.proposal_id }} className="review-row"><div className="review-icon review-icon-blue"><FileCheck2 size={18} /></div><div><strong>{item.intent.objective}</strong><span>{item.principal.subject} · {formatRelative(item.created_at)}</span></div><StatusBadge value={item.status} /><ArrowRight size={16} /></Link>)}{!proposals.data!.items.length ? <div className="compact-empty"><FileCheck2 size={18} />No pending proposals.</div> : null}</div>}
        </section>
        <section className="content-section">
          <div className="section-heading"><div><span className="section-kicker">Authority change</span><h2>Expansions</h2></div><span className="count-badge count-badge-warning">{expansions.data?.total ?? 0}</span></div>
          {expansions.isLoading ? <LoadingState /> : expansions.isError ? <ErrorState error={expansions.error} /> : <div className="review-list">{expansions.data!.items.map((item) => <Link key={item.expansion_id} to="/approvals/expansions/$expansionId" params={{ expansionId: item.expansion_id }} className="review-row"><div className="review-icon review-icon-amber"><ShieldAlert size={18} /></div><div><strong>{item.action.operation} on {item.action.resource.id}</strong><span>{shortID(item.mission_ref)} · {formatRelative(item.created_at)}</span></div><StatusBadge value={item.status} /><ArrowRight size={16} /></Link>)}{!expansions.data!.items.length ? <div className="compact-empty"><Clock3 size={18} />No pending expansions.</div> : null}</div>}
        </section>
      </div>
    </>
  );
}
