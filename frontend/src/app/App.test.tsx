import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const now = "2026-07-18T12:00:00Z";
const authority = { resources: [{ type: "drive_folder", id: "board", actions: ["read", "write_draft"] }], forbidden_actions: ["send_external"] };
const mission = {
  mission_id: "mission-id", mission_ref: "mref-board", tenant_id: "demo", state: "active", version: 1,
  principal: { subject: "alice@example.com", issuer: "https://idp.example.com" },
  agent: { provider: "https://agents.example.com", client_id: "research-agent", instance_id: "inst_123" },
  purpose: { objective: "Prepare board packet", business_context: "Finance close" }, authority_region: authority,
  conditions: [{ id: "close-open", expression: "finance.close.status == 'open'", evaluation: "per_action", on_failure: "suspend" }],
  lifecycle: { created_at: now, expires_at: "2026-07-25T12:00:00Z" },
  delegation: { permitted: true, max_depth: 2, current_depth: 0, attenuation: "strict_subset", cascade_revocation: true },
};
const proposal = { proposal_id: "proposal-1", status: "pending_approval", tenant_id: "demo", principal: mission.principal, agent: mission.agent, intent: mission.purpose, authority_region: authority, lifecycle: mission.lifecycle, delegation: mission.delegation, created_at: now };
const expansion = { expansion_id: "expansion-1", mission_ref: mission.mission_ref, mission_version_seen: 1, tenant_id: "demo", requester: { agent_instance_id: "inst_123", client_id: "research-agent" }, action: { type: "tool_call", resource: { type: "slack_channel", id: "board" }, operation: "post_update" }, requested_authority: { resources: [{ type: "slack_channel", id: "board", actions: ["post_update"] }] }, justification: "Publish the approved update", status: "pending", created_at: now };
const agent = { agent_id: "agent-1", tenant_id: "demo", agent: mission.agent, public_key: "public-key", key_thumbprint: "sha256:key", status: "active", created_at: now };
const containment = { rule_id: "rule-1", tenant_id: "demo", target_type: "mission", target_id: mission.mission_ref, status: "active", reason: "Incident review", created_at: now };
const projection = { projection_id: "projection-1", mission_ref: mission.mission_ref, mission_version: 1, tenant_id: "demo", type: "oauth_claims", actor: { agent_instance_id: "inst_123", client_id: "research-agent" }, status: "active", issued_at: now, expires_at: "2026-07-18T12:05:00Z" };
const event = { event_id: "event-1", mission_ref: mission.mission_ref, tenant_id: "demo", type: "mission.approved", occurred_at: now, payload: { proposal_id: "proposal-1" } };
const policyBundle = { bundle_id: "policy-1", tenant_id: "demo", version: "mission-policy/custom", name: "Enterprise guardrail", status: "active", rules: [{ rule_id: "guardrail-001", effect: "deny" }], bundle_hash: "sha256:test", created_at: now };

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json", "x-request-id": "req-ui" } });
}

function apiFetch(input: RequestInfo | URL, init?: RequestInit) {
  const url = new URL(String(input), "http://localhost");
  const path = url.pathname.replace(/^\/api/, "");
  const method = init?.method ?? "GET";
  if (path === "/v1/admin/session") return Promise.resolve(response({ principal: mission.principal, capabilities: { approve: true }, api_version: "v1" }));
  if (path === "/v1/operations/summary") return Promise.resolve(response({ missions_total: 1, missions_by_state: { active: 1 }, pending_proposals: 1, pending_expansions: 1, active_containments: 1, active_agents: 1, active_projections: 1, recent_event_count: 1, service_capabilities: {} }));
  if (path === "/v1/missions") return Promise.resolve(response({ items: [mission], total: 1 }));
  if (path === `/v1/missions/${mission.mission_ref}/introspect`) return Promise.resolve(response(mission));
  if (path === `/v1/missions/${mission.mission_ref}/lineage`) return Promise.resolve(response({ nodes: [{ id: mission.mission_ref, type: "mission", label: mission.purpose.objective }], edges: [] }));
  if (path === "/v1/mission-proposals") return Promise.resolve(response(method === "POST" ? { proposal_id: proposal.proposal_id, status: "pending_approval" } : { items: [proposal], total: 1 }, method === "POST" ? 201 : 200));
  if (path === `/v1/mission-proposals/${proposal.proposal_id}`) return Promise.resolve(response(proposal));
  if (path === "/v1/expansion-requests") return Promise.resolve(response({ items: [expansion], total: 1 }));
  if (path === `/v1/expansion-requests/${expansion.expansion_id}`) return Promise.resolve(response(expansion));
  if (path === "/v1/agents") return Promise.resolve(response({ items: [agent], total: 1 }));
  if (path === `/v1/agents/${agent.agent_id}`) return Promise.resolve(response(agent));
  if (path === `/v1/agents/${agent.agent_id}/lineage`) return Promise.resolve(response({ nodes: [{ id: agent.agent_id, type: "agent", label: agent.agent.client_id }], edges: [] }));
  if (path === "/v1/containment-rules") return Promise.resolve(response({ containment_rules: [containment] }));
  if (path === `/v1/containment-rules/${containment.rule_id}`) return Promise.resolve(response(containment));
  if (path === `/v1/containment-rules/${containment.rule_id}/blast-radius`) return Promise.resolve(response({ rule: containment, missions: [mission], agents: [agent], projections: [projection] }));
  if (path === "/v1/approval-rules") return Promise.resolve(response({ approval_rules: [{ rule_id: "approval-rule-1", tenant_id: "demo", applies_to: "expansion", required_approvals: 2, allowed_subjects: ["alice@example.com", "bob@example.com"], created_at: now }] }));
  if (path === "/v1/policy-bundles") return Promise.resolve(response({ policy_bundles: [policyBundle] }));
  if (path === "/v1/tool-contracts") return Promise.resolve(response({ items: [{ tool_name: "drive.read", resource_type: "drive_folder", operation: "read", required_context: ["finance.close.status"] }], total: 1 }));
  if (path === "/v1/projections") return Promise.resolve(response({ items: [projection], total: 1 }));
  if (path === "/v1/events") return Promise.resolve(response({ items: [event], total: 1 }));
  return Promise.resolve(response({ message: `Unhandled ${method} ${path}` }, 404));
}

describe("operator console", () => {
  beforeEach(() => {
    window.history.pushState({}, "", "/");
    vi.stubGlobal("fetch", vi.fn(apiFetch));
  });

  it("authenticates and renders every primary operator area", async () => {
    const user = userEvent.setup();
    render(<App />);
    expect(screen.getByRole("heading", { name: "Administrator access" })).toBeInTheDocument();
    await user.type(screen.getByLabelText("Bearer token"), "dev-token");
    await user.click(screen.getByRole("button", { name: "Open console" }));
    expect(await screen.findByRole("heading", { name: "Authority overview" })).toBeInTheDocument();
    expect(screen.getByText("post_update on board")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Missions" }));
    expect(await screen.findByRole("heading", { name: "Missions" })).toBeInTheDocument();
    expect(await screen.findByText("Prepare board packet")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Approvals" }));
    expect(await screen.findByRole("heading", { name: "Approval queue" })).toBeInTheDocument();
    expect((await screen.findAllByText("post_update on board")).length).toBeGreaterThan(0);

    await user.click(screen.getByRole("link", { name: "Agents" }));
    expect(await screen.findByRole("heading", { name: "Agents" })).toBeInTheDocument();
    expect(await screen.findByText("inst_123")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Containment" }));
    expect(await screen.findByRole("heading", { name: "Containment" })).toBeInTheDocument();
    expect(await screen.findByText("Incident review")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Governance" }));
    expect(await screen.findByRole("heading", { name: "Governance" })).toBeInTheDocument();
    expect(await screen.findByText("drive.read")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Projections" }));
    expect(await screen.findByRole("heading", { name: "Projections" })).toBeInTheDocument();
    expect(await screen.findByText("OAuth Claims")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Audit" }));
    expect(await screen.findByRole("heading", { name: "Audit events" })).toBeInTheDocument();
    expect(await screen.findByText("Mission Approved")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Workbench" }));
    expect(await screen.findByRole("heading", { name: "Authority workbench" })).toBeInTheDocument();
  }, 15000);

  it("rejects an invalid administrator token", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ code: "authentication_required", message: "Authentication required" }, 401)));
    const user = userEvent.setup(); render(<App />);
    await user.type(screen.getByLabelText("Bearer token"), "bad-token");
    await user.click(screen.getByRole("button", { name: "Open console" }));
    expect(await screen.findByText("Authentication required")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Administrator access" })).toBeInTheDocument();
  });
});
