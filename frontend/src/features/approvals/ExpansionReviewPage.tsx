import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "@tanstack/react-router";
import { ArrowLeft, Ban, CheckCircle2, ShieldAlert } from "lucide-react";
import { useState } from "react";
import { useApi } from "../../shared/auth/SessionProvider";
import { AuthorityView } from "../../shared/components/AuthorityView";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate } from "../../shared/formatting";

export function ExpansionReviewPage() {
  const { expansionId } = useParams({ from: "/approvals/expansions/$expansionId" });
  const api = useApi();
  const client = useQueryClient();
  const navigate = useNavigate();
  const [reason, setReason] = useState("");
  const expansion = useQuery({ queryKey: ["expansion", expansionId], queryFn: () => api.getExpansion(expansionId) });
  const mission = useQuery({ queryKey: ["mission", expansion.data?.mission_ref], queryFn: () => api.getMission(expansion.data!.mission_ref), enabled: Boolean(expansion.data?.mission_ref) });
  const finish = async () => { await Promise.all([client.invalidateQueries({ queryKey: ["expansions"] }), client.invalidateQueries({ queryKey: ["expansion", expansionId] }), client.invalidateQueries({ queryKey: ["missions"] }), client.invalidateQueries({ queryKey: ["summary"] })]); navigate({ to: "/approvals" }); };
  const approve = useMutation({ mutationFn: () => api.approveExpansion(expansionId, { reason, approval_evidence: { method: "operator_console" } }), onSuccess: finish });
  const deny = useMutation({ mutationFn: () => api.denyExpansion(expansionId, reason), onSuccess: finish });
  if (expansion.isLoading) return <LoadingState label="Loading expansion" />;
  if (expansion.isError) {
    return (
      <>
        <PageHeader
          eyebrow="Expansion review"
          title="Expansion unavailable"
          description="The requested authority expansion could not be loaded. Return to the queue or retry the request."
          actions={<Link to="/approvals" className="button button-secondary"><ArrowLeft size={16} />Approval queue</Link>}
        />
        <ErrorState error={expansion.error} onRetry={() => expansion.refetch()} />
      </>
    );
  }
  const data = expansion.data!;
  const versionDrift = mission.data && mission.data.version !== data.mission_version_seen;
  return (
    <>
      <PageHeader eyebrow="Expansion review" title={`${data.action.operation} on ${data.action.resource.id}`} description={data.justification || "No justification supplied."} actions={<Link to="/approvals" className="button button-secondary"><ArrowLeft size={16} />Approval queue</Link>} />
      <div className="entity-status-line"><StatusBadge value={data.status} /><code>{data.expansion_id}</code><span>Mission {data.mission_ref}</span><span>Created {formatDate(data.created_at)}</span></div>
      {versionDrift ? <div className="warning-banner"><ShieldAlert size={18} /><div><strong>Mission version changed</strong><span>Request saw v{data.mission_version_seen}; current authority is v{mission.data?.version}. The service will reject stale approval.</span></div></div> : null}
      <div className="authority-compare">
        <section className="content-section"><div className="section-heading"><div><span className="section-kicker">Current</span><h2>Effective authority</h2></div></div>{mission.isLoading ? <LoadingState /> : mission.isError ? <ErrorState error={mission.error} /> : <AuthorityView authority={mission.data!.authority_region} />}</section>
        <section className="content-section requested-authority"><div className="section-heading"><div><span className="section-kicker">Requested addition</span><h2>Expansion</h2></div></div><AuthorityView authority={data.requested_authority} /></section>
      </div>
      <section className="decision-bar"><div><h2>Record decision</h2><p>Approval is committed atomically with the mission version change.</p></div><label className="field decision-reason"><span>Decision reason</span><input value={reason} onChange={(event) => setReason(event.target.value)} /></label><div className="decision-actions"><button className="button button-danger" disabled={!reason.trim() || deny.isPending || data.status !== "pending"} onClick={() => deny.mutate()}><Ban size={16} />Deny</button><button className="button button-primary" disabled={!reason.trim() || approve.isPending || data.status !== "pending"} onClick={() => approve.mutate()}><CheckCircle2 size={16} />Approve</button></div>{approve.error || deny.error ? <p className="form-error" role="alert">{(approve.error ?? deny.error)?.message}</p> : null}</section>
    </>
  );
}
