import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Braces, Plus, Scale, X } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { PageHeader } from "../../shared/components/PageHeader";
import { formatDate } from "../../shared/formatting";

export function GovernancePage() {
  const api = useApi();
  const client = useQueryClient();
  const [mode, setMode] = useState<"rule" | "tool" | null>(null);
  const [ruleError, setRuleError] = useState("");
  const [toolError, setToolError] = useState("");
  const [rule, setRule] = useState({ tenant_id: "demo", applies_to: "expansion", resource_type: "", operation: "", required_approvals: 2, allowed_subjects: "alice@example.com, bob@example.com" });
  const [tool, setTool] = useState({ tool_name: "", resource_type: "", resource_id_param: "resource_id", operation: "read", required_context: "" });
  const rules = useQuery({ queryKey: ["approval-rules"], queryFn: () => api.listApprovalRules() });
  const tools = useQuery({ queryKey: ["tool-contracts"], queryFn: () => api.listToolContracts({ limit: 100 }) });
  const createRule = useMutation({
    mutationFn: () => api.createApprovalRule({ ...rule, allowed_subjects: rule.allowed_subjects.split(",").map((item) => item.trim()).filter(Boolean) } as any),
    onSuccess: async () => {
      setMode(null);
      setRuleError("");
      await client.invalidateQueries({ queryKey: ["approval-rules"] });
    },
  });
  const createTool = useMutation({
    mutationFn: () => api.createToolContract({ ...tool, required_context: tool.required_context.split(",").map((item) => item.trim()).filter(Boolean) }),
    onSuccess: async () => {
      setMode(null);
      setToolError("");
      await client.invalidateQueries({ queryKey: ["tool-contracts"] });
    },
  });

  function changeMode(nextMode: "rule" | "tool") {
    setMode((current) => current === nextMode ? null : nextMode);
    setRuleError("");
    setToolError("");
    createRule.reset();
    createTool.reset();
  }

  function submitRule() {
    createRule.reset();
    if (!Number.isFinite(rule.required_approvals) || rule.required_approvals < 1) {
      setRuleError("Required approvals must be at least 1.");
      return;
    }
    setRuleError("");
    createRule.mutate();
  }

  function submitTool() {
    createTool.reset();
    if (!tool.tool_name.trim() || !tool.resource_type.trim() || !tool.resource_id_param.trim() || !tool.operation.trim()) {
      setToolError("Tool name, resource type, resource ID parameter, and operation are required.");
      return;
    }
    setToolError("");
    createTool.mutate();
  }

  return (
    <>
      <PageHeader
        eyebrow="Policy controls"
        title="Governance"
        description="Multi-party approval requirements and tool authorization contracts."
        actions={
          <>
            <button className="button button-secondary" onClick={() => changeMode("rule")}>{mode === "rule" ? <X size={16} /> : <Plus size={16} />}Approval rule</button>
            <button className="button button-primary" onClick={() => changeMode("tool")}>{mode === "tool" ? <X size={16} /> : <Plus size={16} />}Tool contract</button>
          </>
        }
      />
      {mode === "rule" ? (
        <section className="create-band">
          <div className="create-band-heading"><Scale size={21} /><div><h2>New approval rule</h2><p>Require independent authenticated administrators for matching expansions.</p></div></div>
          <div className="inline-form">
            <Field label="Tenant"><input value={rule.tenant_id} onChange={(event) => setRule({ ...rule, tenant_id: event.target.value })} /></Field>
            <Field label="Resource type"><input value={rule.resource_type} onChange={(event) => setRule({ ...rule, resource_type: event.target.value })} /></Field>
            <Field label="Operation"><input value={rule.operation} onChange={(event) => setRule({ ...rule, operation: event.target.value })} /></Field>
            <Field label="Approvals"><input type="number" min={1} value={rule.required_approvals} onChange={(event) => setRule({ ...rule, required_approvals: Number(event.target.value) })} /></Field>
            <Field label="Allowed subjects" wide><input value={rule.allowed_subjects} onChange={(event) => setRule({ ...rule, allowed_subjects: event.target.value })} /></Field>
            <button className="button button-primary" onClick={submitRule} disabled={createRule.isPending}>{createRule.isPending ? "Creating..." : "Create rule"}</button>
          </div>
          <FormError message={ruleError || createRule.error?.message} />
        </section>
      ) : null}
      {mode === "tool" ? (
        <section className="create-band">
          <div className="create-band-heading"><Braces size={21} /><div><h2>New tool contract</h2><p>Map one tool invocation to a resource operation.</p></div></div>
          <div className="inline-form">
            <Field label="Tool name"><input value={tool.tool_name} onChange={(event) => setTool({ ...tool, tool_name: event.target.value })} /></Field>
            <Field label="Resource type"><input value={tool.resource_type} onChange={(event) => setTool({ ...tool, resource_type: event.target.value })} /></Field>
            <Field label="Resource ID parameter"><input value={tool.resource_id_param} onChange={(event) => setTool({ ...tool, resource_id_param: event.target.value })} /></Field>
            <Field label="Operation"><input value={tool.operation} onChange={(event) => setTool({ ...tool, operation: event.target.value })} /></Field>
            <Field label="Required context" wide><input value={tool.required_context} onChange={(event) => setTool({ ...tool, required_context: event.target.value })} /></Field>
            <button className="button button-primary" onClick={submitTool} disabled={createTool.isPending}>{createTool.isPending ? "Creating..." : "Create contract"}</button>
          </div>
          <FormError message={toolError || createTool.error?.message} />
        </section>
      ) : null}
      <div className="governance-grid">
        <section className="content-section">
          <div className="section-heading"><div><span className="section-kicker">Human controls</span><h2>Approval rules</h2></div></div>
          {rules.isLoading ? <LoadingState /> : rules.isError ? <ErrorState error={rules.error} /> : <div className="rule-list">{rules.data!.approval_rules.map((item) => <div className="rule-row" key={item.rule_id}><div className="rule-count">{item.required_approvals}</div><div><strong>{item.operation || "Any operation"} · {item.resource_type || "Any resource"}</strong><span>{item.tenant_id || "All tenants"} · created {formatDate(item.created_at)}</span><small>{item.allowed_subjects?.join(", ") || "Any authenticated administrator"}</small></div></div>)}{!rules.data!.approval_rules.length ? <div className="compact-empty">No approval rules.</div> : null}</div>}
        </section>
        <section className="content-section">
          <div className="section-heading"><div><span className="section-kicker">Gateway mapping</span><h2>Tool contracts</h2></div></div>
          {tools.isLoading ? <LoadingState /> : tools.isError ? <ErrorState error={tools.error} /> : <div className="rule-list">{tools.data!.items.map((item) => <div className="tool-row" key={item.tool_name}><Braces size={18} /><div><strong>{item.tool_name}</strong><span>{item.operation} · {item.resource_type}</span><small>{item.required_context?.join(", ") || "No required context"}</small></div></div>)}{!tools.data!.items.length ? <div className="compact-empty">No tool contracts.</div> : null}</div>}
        </section>
      </div>
    </>
  );
}

function Field({ label, wide, children }: { label: string; wide?: boolean; children: ReactNode }) {
  return <label className={`field ${wide ? "field-wide" : ""}`}><span>{label}</span>{children}</label>;
}

function FormError({ message }: { message?: string }) {
  return message ? <p className="form-error" role="alert">{message}</p> : null;
}
