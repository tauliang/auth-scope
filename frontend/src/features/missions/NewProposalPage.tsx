import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { ArrowLeft, Plus, ShieldCheck } from "lucide-react";
import { useForm } from "react-hook-form";
import { useState, type ReactNode } from "react";
import { z } from "zod";
import { useApi } from "../../shared/auth/SessionProvider";
import { PageHeader } from "../../shared/components/PageHeader";

const schema = z.object({
  tenant: z.string().trim().min(1),
  objective: z.string().trim().min(5, "Objective must be at least 5 characters"),
  businessContext: z.string().trim(),
  principalSubject: z.string().trim().min(1),
  principalIssuer: z.string().trim().min(1),
  agentProvider: z.string().trim().min(1),
  agentClient: z.string().trim().min(1),
  agentInstance: z.string().trim().min(1),
  resourceType: z.string().trim().min(1),
  resourceID: z.string().trim().min(1),
  actions: z.string().trim().min(1),
  expiresAt: z.string().trim().min(1),
});
type Values = z.infer<typeof schema>;

export function NewProposalPage() {
  const api = useApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [defaultExpiry] = useState(() => new Date(Date.now() + 7 * 86_400_000).toISOString().slice(0, 16));
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: {
    tenant: "demo",
    objective: "",
    businessContext: "",
    principalSubject: "alice@example.com",
    principalIssuer: "https://idp.example.com",
    agentProvider: "https://agents.example.com",
    agentClient: "research-agent",
    agentInstance: "inst_123",
    resourceType: "drive_folder",
    resourceID: "board",
    actions: "read, write_draft",
    expiresAt: defaultExpiry,
  }});
  const create = useMutation({
    mutationFn: (values: Values) => api.createProposal({
      tenant_id: values.tenant,
      principal: { subject: values.principalSubject, issuer: values.principalIssuer },
      agent: { provider: values.agentProvider, client_id: values.agentClient, instance_id: values.agentInstance },
      intent: { objective: values.objective, business_context: values.businessContext },
      authority_region: { resources: [{ type: values.resourceType, id: values.resourceID, actions: values.actions.split(",").map((item) => item.trim()).filter(Boolean) }] },
      lifecycle: { expires_at: new Date(values.expiresAt).toISOString() },
      delegation: { permitted: true, max_depth: 1, cascade_revocation: true, attenuation: "strict_subset" },
    }),
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: ["proposals"] });
      navigate({ to: "/approvals/proposals/$proposalId", params: { proposalId: result.proposal_id } });
    },
  });

  return (
    <>
      <PageHeader eyebrow="Mission proposal" title="Define bounded authority" description="Create a reviewable mission request with one explicit resource grant." actions={<Link to="/missions" className="button button-secondary"><ArrowLeft size={16} />Missions</Link>} />
      <form className="form-layout" onSubmit={form.handleSubmit((values) => create.mutate(values))}>
        <section className="form-section"><div className="form-section-heading"><span>01</span><div><h2>Purpose and owner</h2><p>The business objective and human principal.</p></div></div><div className="form-grid">
          <Field label="Tenant" error={form.formState.errors.tenant?.message}><input {...form.register("tenant")} /></Field>
          <Field label="Objective" wide error={form.formState.errors.objective?.message}><input {...form.register("objective")} /></Field>
          <Field label="Business context" wide><input {...form.register("businessContext")} /></Field>
          <Field label="Principal subject"><input {...form.register("principalSubject")} /></Field>
          <Field label="Principal issuer"><input {...form.register("principalIssuer")} /></Field>
        </div></section>
        <section className="form-section"><div className="form-section-heading"><span>02</span><div><h2>Agent binding</h2><p>The workload receiving this mission.</p></div></div><div className="form-grid">
          <Field label="Agent provider" wide><input {...form.register("agentProvider")} /></Field>
          <Field label="Client ID"><input {...form.register("agentClient")} /></Field>
          <Field label="Instance ID"><input {...form.register("agentInstance")} /></Field>
        </div></section>
        <section className="form-section"><div className="form-section-heading"><span>03</span><div><h2>Authority boundary</h2><p>Resource, operations, and terminal time.</p></div></div><div className="form-grid">
          <Field label="Resource type"><input {...form.register("resourceType")} /></Field>
          <Field label="Resource ID"><input {...form.register("resourceID")} /></Field>
          <Field label="Actions" wide hint="Comma separated"><input {...form.register("actions")} /></Field>
          <Field label="Expires at"><input type="datetime-local" {...form.register("expiresAt")} /></Field>
        </div></section>
        {create.error ? <p className="form-error" role="alert">{create.error.message}</p> : null}
        <div className="form-submit"><button className="button button-primary" disabled={create.isPending}><Plus size={16} />{create.isPending ? "Creating..." : "Create proposal"}</button><span><ShieldCheck size={15} />Approval is required before authority becomes active.</span></div>
      </form>
    </>
  );
}

function Field({ label, error, hint, wide, children }: { label: string; error?: string; hint?: string; wide?: boolean; children: ReactNode }) {
  return <label className={`field ${wide ? "field-wide" : ""}`}><span>{label}{hint ? <small>{hint}</small> : null}</span>{children}{error ? <em>{error}</em> : null}</label>;
}
