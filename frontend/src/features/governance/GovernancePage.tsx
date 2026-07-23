import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Braces, Plus, Scale, ShieldCheck, X } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useApi } from "../../shared/auth/SessionProvider";
import { ErrorState, LoadingState } from "../../shared/components/AsyncState";
import { PageHeader } from "../../shared/components/PageHeader";
import { formatDate } from "../../shared/formatting";

export function GovernancePage() {
  const api = useApi();
  const client = useQueryClient();
  const [mode, setMode] = useState<"rule" | "tool" | "policy" | null>(null);
  const [ruleError, setRuleError] = useState("");
  const [toolError, setToolError] = useState("");
  const [policyError, setPolicyError] = useState("");
  const [rule, setRule] = useState({ tenant_id: "demo", applies_to: "expansion", resource_type: "", operation: "", required_approvals: 2, allowed_subjects: "alice@example.com, bob@example.com" });
  const [tool, setTool] = useState({ tool_name: "", resource_type: "", resource_id_param: "resource_id", operation: "read", required_context: "" });
  const [policy, setPolicy] = useState({
    tenant_id: "demo",
    version: "mission-policy/custom",
    name: "Enterprise guardrail",
    rule_id: "guardrail-001",
    effect: "deny",
    operations: "delete, send_external",
    resource_types: "",
    condition_expression: "context.risk == 'high'",
    reason_codes: "POLICY_ENTERPRISE_GUARDRAIL",
    human_reason: "The action is blocked by enterprise policy.",
  });
  const rules = useQuery({ queryKey: ["approval-rules"], queryFn: () => api.listApprovalRules() });
  const tools = useQuery({ queryKey: ["tool-contracts"], queryFn: () => api.listToolContracts({ limit: 100 }) });
  const policies = useQuery({ queryKey: ["policy-bundles"], queryFn: () => api.listPolicyBundles() });
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
  const createPolicy = useMutation({
    mutationFn: () => api.createPolicyBundle({
      tenant_id: emptyToUndefined(policy.tenant_id),
      version: policy.version.trim(),
      name: emptyToUndefined(policy.name),
      combining_algorithm: "first_applicable",
      rules: [{
        rule_id: policy.rule_id.trim(),
        priority: 10,
        effect: policy.effect as "allow" | "deny" | "require_approval" | "require_expansion" | "allow_with_constraints",
        match: {
          operations: splitList(policy.operations),
          resource_types: splitList(policy.resource_types),
          base_decisions: ["allow"],
        },
        conditions: policy.condition_expression.trim() ? [{ id: "policy-condition", expression: policy.condition_expression.trim() }] : undefined,
        reason_codes: splitList(policy.reason_codes),
        human_reason: emptyToUndefined(policy.human_reason),
      }],
    }),
    onSuccess: async () => {
      setMode(null);
      setPolicyError("");
      await client.invalidateQueries({ queryKey: ["policy-bundles"] });
    },
  });
  const activatePolicy = useMutation({
    mutationFn: (bundleID: string) => api.activatePolicyBundle(bundleID, "Activated from operator console"),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["policy-bundles"] });
    },
  });

  function changeMode(nextMode: "rule" | "tool" | "policy") {
    setMode((current) => current === nextMode ? null : nextMode);
    setRuleError("");
    setToolError("");
    setPolicyError("");
    createRule.reset();
    createTool.reset();
    createPolicy.reset();
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

  function submitPolicy() {
    createPolicy.reset();
    if (!policy.version.trim() || !policy.rule_id.trim() || !policy.effect.trim()) {
      setPolicyError("Policy version, rule ID, and effect are required.");
      return;
    }
    if (!splitList(policy.operations).length && !splitList(policy.resource_types).length) {
      setPolicyError("Provide at least one operation or resource type.");
      return;
    }
    setPolicyError("");
    createPolicy.mutate();
  }

  return (
    <>
      <PageHeader
        eyebrow="Policy controls"
        title="Governance"
        description="Policy bundles, approval requirements, and tool authorization contracts."
        actions={
          <>
            <button className="button button-secondary" onClick={() => changeMode("rule")}>{mode === "rule" ? <X size={16} /> : <Plus size={16} />}Approval rule</button>
            <button className="button button-secondary" onClick={() => changeMode("tool")}>{mode === "tool" ? <X size={16} /> : <Plus size={16} />}Tool contract</button>
            <button className="button button-primary" onClick={() => changeMode("policy")}>{mode === "policy" ? <X size={16} /> : <Plus size={16} />}Policy bundle</button>
          </>
        }
      />
      {mode === "policy" ? (
        <section className="create-band">
          <div className="create-band-heading"><ShieldCheck size={21} /><div><h2>New policy bundle</h2><p>Create a versioned rule bundle that can be activated after review.</p></div></div>
          <div className="inline-form">
            <Field label="Tenant"><input value={policy.tenant_id} onChange={(event) => setPolicy({ ...policy, tenant_id: event.target.value })} /></Field>
            <Field label="Version"><input value={policy.version} onChange={(event) => setPolicy({ ...policy, version: event.target.value })} /></Field>
            <Field label="Name"><input value={policy.name} onChange={(event) => setPolicy({ ...policy, name: event.target.value })} /></Field>
            <Field label="Effect"><select value={policy.effect} onChange={(event) => setPolicy({ ...policy, effect: event.target.value })}><option value="deny">Deny</option><option value="require_approval">Require approval</option><option value="require_expansion">Require expansion</option><option value="allow_with_constraints">Allow with constraints</option></select></Field>
            <Field label="Rule ID"><input value={policy.rule_id} onChange={(event) => setPolicy({ ...policy, rule_id: event.target.value })} /></Field>
            <Field label="Operations"><input value={policy.operations} onChange={(event) => setPolicy({ ...policy, operations: event.target.value })} /></Field>
            <Field label="Resource types"><input value={policy.resource_types} onChange={(event) => setPolicy({ ...policy, resource_types: event.target.value })} /></Field>
            <Field label="Reason codes"><input value={policy.reason_codes} onChange={(event) => setPolicy({ ...policy, reason_codes: event.target.value })} /></Field>
            <Field label="Condition" wide><input value={policy.condition_expression} onChange={(event) => setPolicy({ ...policy, condition_expression: event.target.value })} /></Field>
            <Field label="Human reason" wide><input value={policy.human_reason} onChange={(event) => setPolicy({ ...policy, human_reason: event.target.value })} /></Field>
            <button className="button button-primary" onClick={submitPolicy} disabled={createPolicy.isPending}>{createPolicy.isPending ? "Creating..." : "Create bundle"}</button>
          </div>
          <FormError message={policyError || createPolicy.error?.message} />
        </section>
      ) : null}
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
          <div className="section-heading"><div><span className="section-kicker">Policy-as-code</span><h2>Policy bundles</h2></div></div>
          {policies.isLoading ? <LoadingState /> : policies.isError ? <ErrorState error={policies.error} /> : <div className="rule-list">{policies.data!.policy_bundles.map((item) => <div className="tool-row" key={item.bundle_id}><ShieldCheck size={18} /><div><strong>{item.name || item.version}</strong><span>{item.status} · {item.tenant_id || "All tenants"}</span><small>{item.rules.length} rules · {item.bundle_hash || "Unsigned draft"}</small></div>{item.status === "draft" ? <button className="button button-secondary" onClick={() => activatePolicy.mutate(item.bundle_id)} disabled={activatePolicy.isPending}>Activate</button> : null}</div>)}{!policies.data!.policy_bundles.length ? <div className="compact-empty">No policy bundles.</div> : null}</div>}
          <FormError message={activatePolicy.error?.message} />
        </section>
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

function splitList(value: string) {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim();
  return trimmed ? trimmed : undefined;
}
