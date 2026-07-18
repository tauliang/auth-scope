import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { ArrowLeft, CheckCircle2, UserRound } from "lucide-react";
import { useState } from "react";
import { useApi } from "../../shared/auth/SessionProvider";
import { AuthorityView } from "../../shared/components/AuthorityView";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate } from "../../shared/formatting";

export function ProposalReviewPage() {
  const { proposalId } = useParams({ from: "/approvals/proposals/$proposalId" });
  const api = useApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [method, setMethod] = useState("operator_console");
  const proposal = useQuery({ queryKey: ["proposal", proposalId], queryFn: () => api.getProposal(proposalId) });
  const approve = useMutation({ mutationFn: () => api.approveProposal(proposalId, method), onSuccess: async (result) => { await Promise.all([queryClient.invalidateQueries({ queryKey: ["proposals"] }), queryClient.invalidateQueries({ queryKey: ["summary"] }), queryClient.invalidateQueries({ queryKey: ["missions"] })]); navigate({ to: "/missions/$missionRef", params: { missionRef: result.mission_ref } }); } });
  if (proposal.isLoading) return <LoadingState label="Loading proposal" />;
  if (proposal.isError) {
    return (
      <>
        <PageHeader
          eyebrow="Proposal review"
          title="Proposal unavailable"
          description="The requested proposal could not be loaded. Return to the queue or retry the request."
          actions={<Link to="/approvals" className="button button-secondary"><ArrowLeft size={16} />Approval queue</Link>}
        />
        <ErrorState error={proposal.error} onRetry={() => proposal.refetch()} />
      </>
    );
  }
  const data = proposal.data!;
  return (
    <>
      <PageHeader eyebrow="Proposal review" title={data.intent.objective} description={`Requested by ${data.principal.subject} for ${data.agent.client_id}.`} actions={<Link to="/approvals" className="button button-secondary"><ArrowLeft size={16} />Approval queue</Link>} />
      <div className="entity-status-line"><StatusBadge value={data.status} /><code>{data.proposal_id}</code><span>Created {formatDate(data.created_at)}</span></div>
      <div className="review-layout">
        <div>
          <section className="content-section"><div className="section-heading"><h2>Requested authority</h2></div><AuthorityView authority={data.authority_region} /></section>
          <section className="content-section"><div className="section-heading"><h2>Binding</h2></div><dl className="definition-grid"><div><dt><UserRound size={15} />Principal</dt><dd>{data.principal.subject}</dd><small>{data.principal.issuer}</small></div><div><dt>Agent</dt><dd>{data.agent.client_id}</dd><small>{data.agent.instance_id}</small></div><div><dt>Tenant</dt><dd>{data.tenant_id}</dd></div><div><dt>Expires</dt><dd>{formatDate(data.lifecycle.expires_at)}</dd></div></dl></section>
        </div>
        <aside className="decision-panel"><div className="decision-panel-icon"><CheckCircle2 size={22} /></div><h2>Approve mission</h2><p>Approval activates this exact authority region and records your authenticated principal.</p><label className="field"><span>Evidence method</span><input value={method} onChange={(event) => setMethod(event.target.value)} /></label>{approve.error ? <p className="form-error" role="alert">{approve.error.message}</p> : null}<button className="button button-primary button-wide" disabled={approve.isPending || data.status !== "pending_approval"} onClick={() => approve.mutate()}><CheckCircle2 size={16} />{approve.isPending ? "Approving..." : "Approve and activate"}</button></aside>
      </div>
    </>
  );
}
