import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft, CheckCircle2, ShieldAlert } from "lucide-react";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { ConfirmDialog } from "../../shared/components/ConfirmDialog";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate } from "../../shared/formatting";

export function ContainmentDetailPage() {
  const { ruleId } = useParams({ from: "/containment/$ruleId" });
  const api = useApi();
  const client = useQueryClient();
  const rule = useQuery({ queryKey: ["containment", ruleId], queryFn: () => api.getContainment(ruleId) });
  const blast = useQuery({ queryKey: ["blast-radius", ruleId], queryFn: () => api.getBlastRadius(ruleId) });
  const lift = useMutation({ mutationFn: (reason: string) => api.liftContainment(ruleId, reason), onSuccess: async () => { await Promise.all([client.invalidateQueries({ queryKey: ["containment"] }), client.invalidateQueries({ queryKey: ["containment", ruleId] }), client.invalidateQueries({ queryKey: ["summary"] })]); } });
  if (rule.isLoading) return <LoadingState />;
  if (rule.isError) {
    return (
      <>
        <PageHeader
          eyebrow="Containment rule"
          title="Containment unavailable"
          description="The requested containment rule could not be loaded. Return to containment or retry the request."
          actions={<Link to="/containment" className="button button-secondary"><ArrowLeft size={16} />Containment</Link>}
        />
        <ErrorState error={rule.error} onRetry={() => rule.refetch()} />
      </>
    );
  }
  const data = rule.data!;
  const counts = blast.data ? [{ label: "Missions", value: blast.data.missions?.length ?? 0 }, { label: "Agents", value: blast.data.agents?.length ?? 0 }, { label: "Projections", value: blast.data.projections?.length ?? 0 }, { label: "Expansions", value: blast.data.expansion_requests?.length ?? 0 }, { label: "Leases", value: blast.data.leases?.length ?? 0 }, { label: "Tools", value: blast.data.tool_contracts?.length ?? 0 }] : [];
  return <><PageHeader eyebrow="Containment rule" title={`${data.target_type}: ${data.target_id}`} description={data.reason || "No reason recorded."} actions={<><Link to="/containment" className="button button-secondary"><ArrowLeft size={16} />Containment</Link>{data.status === "active" ? <ConfirmDialog title="Lift containment" description="Affected authority may become usable immediately after this rule is lifted." confirmLabel="Lift containment" tone="primary" onConfirm={(reason) => lift.mutateAsync(reason)} trigger={<button className="button button-primary"><CheckCircle2 size={16} />Lift rule</button>} /> : null}</>} /><div className="entity-status-line"><StatusBadge value={data.status} /><code>{data.rule_id}</code><span>Created {formatDate(data.created_at)}</span><span>Expires {formatDate(data.expires_at)}</span></div><section className="content-section blast-section"><div className="section-heading"><div><span className="section-kicker">Consequence map</span><h2>Blast radius</h2></div><ShieldAlert size={20} /></div>{blast.isLoading ? <LoadingState /> : blast.isError ? <ErrorState error={blast.error} /> : <div className="blast-grid">{counts.map((item) => <div key={item.label}><strong>{item.value}</strong><span>{item.label}</span></div>)}</div>}</section>{blast.data?.missions?.length ? <section className="content-section"><div className="section-heading"><h2>Affected missions</h2></div><div className="entity-link-list">{blast.data.missions.map((mission) => <Link to="/missions/$missionRef" params={{ missionRef: mission.mission_ref }} key={mission.mission_ref}><div><strong>{mission.purpose.objective}</strong><span>{mission.mission_ref}</span></div><StatusBadge value={mission.state} /></Link>)}</div></section> : null}</>;
}
