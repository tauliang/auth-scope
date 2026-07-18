import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft, Ban, Fingerprint, KeyRound, Network } from "lucide-react";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { ConfirmDialog } from "../../shared/components/ConfirmDialog";
import { LineageGraph } from "../../shared/components/LineageGraph";
import { PageHeader } from "../../shared/components/PageHeader";
import { StatusBadge } from "../../shared/components/StatusBadge";
import { formatDate } from "../../shared/formatting";

export function AgentDetailPage() {
  const { agentId } = useParams({ from: "/agents/$agentId" });
  const api = useApi();
  const client = useQueryClient();
  const agent = useQuery({ queryKey: ["agent", agentId], queryFn: () => api.getAgent(agentId) });
  const lineage = useQuery({ queryKey: ["agent-lineage", agentId], queryFn: () => api.agentLineage(agentId) });
  const revoke = useMutation({ mutationFn: (reason: string) => api.revokeAgent(agentId, reason), onSuccess: async () => { await Promise.all([client.invalidateQueries({ queryKey: ["agent", agentId] }), client.invalidateQueries({ queryKey: ["agents"] }), client.invalidateQueries({ queryKey: ["summary"] })]); } });
  if (agent.isLoading) return <LoadingState label="Loading agent" />;
  if (agent.isError) {
    return (
      <>
        <PageHeader
          eyebrow="Workload identity"
          title="Agent unavailable"
          description="The requested agent identity could not be loaded. Return to the registry or retry the request."
          actions={<Link to="/agents" className="button button-secondary"><ArrowLeft size={16} />Agents</Link>}
        />
        <ErrorState error={agent.error} onRetry={() => agent.refetch()} />
      </>
    );
  }
  const data = agent.data!;
  return <><PageHeader eyebrow={`${data.tenant_id} · workload identity`} title={data.agent.client_id} description={data.agent.instance_id} actions={<><Link to="/agents" className="button button-secondary"><ArrowLeft size={16} />Agents</Link>{data.status === "active" ? <ConfirmDialog title="Revoke agent identity" description="Signed requests from this identity will be rejected immediately." confirmLabel="Revoke agent" onConfirm={(reason) => revoke.mutateAsync(reason)} trigger={<button className="button button-danger"><Ban size={16} />Revoke</button>} /> : null}</>} /><div className="entity-status-line"><StatusBadge value={data.status} /><code>{data.agent_id}</code><span>Registered {formatDate(data.created_at)}</span></div><section className="content-section"><dl className="definition-grid"><div><dt><Network size={15} />Provider</dt><dd>{data.agent.provider}</dd></div><div><dt><Fingerprint size={15} />Instance</dt><dd>{data.agent.instance_id}</dd></div><div><dt><KeyRound size={15} />Thumbprint</dt><dd><code>{data.key_thumbprint}</code></dd></div><div><dt>Revoked</dt><dd>{formatDate(data.revoked_at)}</dd></div></dl></section><section className="content-section lineage-section"><div className="section-heading"><div><span className="section-kicker">Accountability</span><h2>Authority lineage</h2></div></div>{lineage.isLoading ? <LoadingState /> : lineage.isError ? <ErrorState error={lineage.error} /> : <LineageGraph graph={lineage.data!} />}</section></>;
}
