import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const now = "2026-07-18T12:00:00Z";
const authority = {
  resources: [{ type: "drive_folder", id: "board", actions: ["read", "write_draft"] }],
  forbidden_actions: ["send_external"],
};
const mission = {
  mission_id: "mission-id",
  mission_ref: "mref-board",
  tenant_id: "demo",
  state: "active",
  version: 2,
  principal: { subject: "alice@example.com", issuer: "https://idp.example.com" },
  agent: { provider: "https://agents.example.com", client_id: "research-agent", instance_id: "inst_123" },
  purpose: { objective: "Prepare board packet", business_context: "Finance close" },
  authority_region: authority,
  conditions: [{ id: "close-open", expression: "finance.close.status == 'open'", evaluation: "per_action", on_failure: "suspend" }],
  lifecycle: { created_at: now, expires_at: "2026-07-25T12:00:00Z" },
  delegation: { permitted: true, max_depth: 2, current_depth: 0, attenuation: "strict_subset", cascade_revocation: true },
};
const proposal = {
  proposal_id: "proposal-1",
  status: "pending_approval",
  tenant_id: "demo",
  principal: mission.principal,
  agent: mission.agent,
  intent: mission.purpose,
  authority_region: authority,
  lifecycle: mission.lifecycle,
  delegation: mission.delegation,
  created_at: now,
};
const expansion = {
  expansion_id: "expansion-1",
  mission_ref: mission.mission_ref,
  mission_version_seen: 1,
  tenant_id: "demo",
  requester: { agent_instance_id: "inst_123", client_id: "research-agent" },
  action: { type: "tool_call", resource: { type: "slack_channel", id: "board" }, operation: "post_update" },
  requested_authority: { resources: [{ type: "slack_channel", id: "board", actions: ["post_update"] }] },
  justification: "Publish the approved update",
  status: "pending",
  created_at: now,
};
const agent = {
  agent_id: "agent-1",
  tenant_id: "demo",
  agent: mission.agent,
  public_key: "public-key",
  key_thumbprint: "sha256:key",
  status: "active",
  created_at: now,
};
const containment = {
  rule_id: "rule-1",
  tenant_id: "demo",
  target_type: "mission",
  target_id: mission.mission_ref,
  status: "active",
  reason: "Incident review",
  created_at: now,
};
const projection = {
  projection_id: "projection-1",
  mission_ref: mission.mission_ref,
  mission_version: 2,
  tenant_id: "demo",
  type: "oauth_claims",
  actor: { agent_instance_id: "inst_123", client_id: "research-agent" },
  status: "active",
  issued_at: now,
  expires_at: "2026-07-18T12:05:00Z",
};
const event = {
  event_id: "event-1",
  mission_ref: mission.mission_ref,
  tenant_id: "demo",
  type: "mission.approved",
  occurred_at: now,
  payload: { proposal_id: proposal.proposal_id },
};
const policyBundle = {
  bundle_id: "policy-1",
  tenant_id: "demo",
  version: "mission-policy/custom",
  name: "Enterprise guardrail",
  status: "draft",
  rules: [{ rule_id: "guardrail-001", effect: "deny" }],
  bundle_hash: "sha256:test",
  created_at: now,
};

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", "x-request-id": "req-workflow" },
  });
}

function operatorApi(input: RequestInfo | URL, init?: RequestInit) {
  const url = new URL(String(input), "http://localhost");
  const path = url.pathname.replace(/^\/api/, "");
  const method = init?.method ?? "GET";
  if (path === "/v1/admin/session") return Promise.resolve(response({ principal: mission.principal, capabilities: { approve: true }, api_version: "v1" }));
  if (path === "/v1/operations/summary") return Promise.resolve(response({ missions_total: 1, missions_by_state: { active: 1 }, pending_proposals: 1, pending_expansions: 1, active_containments: 1, active_agents: 1, active_projections: 1, recent_event_count: 1, service_capabilities: {} }));
  if (path === "/v1/missions") return Promise.resolve(response({ items: [mission], total: 1 }));
  if (path === `/v1/missions/${mission.mission_ref}/introspect`) return Promise.resolve(response(mission));
  if (path === `/v1/missions/${mission.mission_ref}/lineage`) return Promise.resolve(response({ nodes: [{ id: mission.mission_ref, type: "mission", label: mission.purpose.objective }, { id: agent.agent_id, type: "agent", label: agent.agent.client_id }], edges: [{ from: agent.agent_id, to: mission.mission_ref, type: "executes" }] }));
  if (path === `/v1/missions/${mission.mission_ref}/complete` || path === `/v1/missions/${mission.mission_ref}/revoke`) return Promise.resolve(response({ ...mission, state: path.endsWith("complete") ? "completed" : "revoked" }));
  if (path === "/v1/mission-proposals" && method === "POST") return Promise.resolve(response({ proposal_id: proposal.proposal_id, status: "pending_approval", approval_url: `/approvals/proposals/${proposal.proposal_id}` }, 201));
  if (path === "/v1/mission-proposals") return Promise.resolve(response({ items: [proposal], total: 1 }));
  if (path === `/v1/mission-proposals/${proposal.proposal_id}/approve`) return Promise.resolve(response({ mission_ref: mission.mission_ref, mission_version: 2, state: "active" }));
  if (path === `/v1/mission-proposals/${proposal.proposal_id}`) return Promise.resolve(response(proposal));
  if (path === "/v1/expansion-requests") return Promise.resolve(response({ items: [expansion], total: 1 }));
  if (path === `/v1/expansion-requests/${expansion.expansion_id}/approvals`) return Promise.resolve(response({ expansion_id: expansion.expansion_id, status: "approved", approvals_required: 1, approvals_received: 1, mission_ref: mission.mission_ref }));
  if (path === `/v1/expansion-requests/${expansion.expansion_id}/deny`) return Promise.resolve(response({ ...expansion, status: "denied" }));
  if (path === `/v1/expansion-requests/${expansion.expansion_id}`) return Promise.resolve(response(expansion));
  if (path === "/v1/agents") return Promise.resolve(response({ items: [agent], total: 1 }));
  if (path === `/v1/agents/${agent.agent_id}/lineage`) return Promise.resolve(response({ nodes: [{ id: agent.agent_id, type: "agent", label: agent.agent.client_id }], edges: [] }));
  if (path === `/v1/agents/${agent.agent_id}/revoke`) return Promise.resolve(response({ ...agent, status: "revoked", revoked_at: now }));
  if (path === `/v1/agents/${agent.agent_id}`) return Promise.resolve(response(agent));
  if (path === "/v1/containment-rules" && method === "POST") return Promise.resolve(response(containment, 201));
  if (path === "/v1/containment-rules") return Promise.resolve(response({ containment_rules: [containment] }));
  if (path === `/v1/containment-rules/${containment.rule_id}/lift`) return Promise.resolve(response({ ...containment, status: "lifted" }));
  if (path === `/v1/containment-rules/${containment.rule_id}/blast-radius`) return Promise.resolve(response({ rule: containment, missions: [mission], agents: [agent], projections: [projection], expansion_requests: [expansion], leases: [{}], tool_contracts: [{}] }));
  if (path === `/v1/containment-rules/${containment.rule_id}`) return Promise.resolve(response(containment));
  if (path === "/v1/approval-rules" && method === "POST") return Promise.resolve(response({ rule_id: "approval-rule-2", created_at: now }));
  if (path === "/v1/approval-rules") return Promise.resolve(response({ approval_rules: [{ rule_id: "approval-rule-1", tenant_id: "demo", applies_to: "expansion", required_approvals: 2, allowed_subjects: ["alice@example.com", "bob@example.com"], created_at: now }] }));
  if (path === "/v1/policy-bundles" && method === "POST") return Promise.resolve(response({ ...policyBundle, bundle_id: "policy-2" }, 201));
  if (path === `/v1/policy-bundles/${policyBundle.bundle_id}/activate`) return Promise.resolve(response({ ...policyBundle, status: "active", signature: "hs256:test" }));
  if (path === "/v1/policy-bundles") return Promise.resolve(response({ policy_bundles: [policyBundle] }));
  if (path === "/v1/tool-contracts" && method === "POST") return Promise.resolve(response({ tool_name: "slack.post" }, 201));
  if (path === "/v1/tool-contracts") return Promise.resolve(response({ items: [{ tool_name: "drive.read", resource_type: "drive_folder", operation: "read", required_context: ["finance.close.status"] }], total: 1 }));
  if (path === `/v1/projections/${projection.projection_id}/revoke`) return Promise.resolve(response({ ...projection, status: "revoked" }));
  if (path === "/v1/projections") return Promise.resolve(response({ items: [projection], total: 1 }));
  if (path === "/v1/events") return Promise.resolve(response({ items: [event], total: 1 }));
  if (path === "/v1/decision-artifacts/verify") return Promise.resolve(response({ valid: true, mission_ref: mission.mission_ref }));
  if (path === "/v1/projections/verify") return Promise.resolve(response({ valid: true, projection_id: projection.projection_id }));
  return Promise.resolve(response({ code: "not_found", message: `Unhandled ${method} ${path}` }, 404));
}

function emptyOperatorApi(input: RequestInfo | URL, init?: RequestInit) {
  const url = new URL(String(input), "http://localhost");
  const path = url.pathname.replace(/^\/api/, "");
  if (path === "/v1/operations/summary") return Promise.resolve(response({ missions_total: 0, missions_by_state: {}, pending_proposals: 0, pending_expansions: 0, active_containments: 0, active_agents: 0, active_projections: 0, recent_event_count: 0, service_capabilities: {} }));
  if (path === "/v1/events" || path === "/v1/expansion-requests" || path === "/v1/missions" || path === "/v1/agents" || path === "/v1/projections" || path === "/v1/tool-contracts" || path === "/v1/mission-proposals") return Promise.resolve(response({ items: [], total: 0 }));
  if (path === "/v1/containment-rules") return Promise.resolve(response({ containment_rules: [] }));
  if (path === "/v1/approval-rules") return Promise.resolve(response({ approval_rules: [] }));
  if (path === "/v1/policy-bundles") return Promise.resolve(response({ policy_bundles: [] }));
  return operatorApi(input, init);
}

function terminalMissionApi(input: RequestInfo | URL, init?: RequestInit) {
  const path = new URL(String(input), "http://localhost").pathname.replace(/^\/api/, "");
  if (path === `/v1/missions/${mission.mission_ref}/introspect`) {
    return Promise.resolve(response({
      ...mission,
      state: "completed",
      purpose: { objective: mission.purpose.objective },
      conditions: [],
      delegation: { ...mission.delegation, attenuation: undefined, cascade_revocation: false },
    }));
  }
  return operatorApi(input, init);
}

function filteredAuditApi(input: RequestInfo | URL, init?: RequestInit) {
  const url = new URL(String(input), "http://localhost");
  const path = url.pathname.replace(/^\/api/, "");
  if (path === "/v1/events" && (url.searchParams.get("q") || url.searchParams.get("type"))) {
    return Promise.resolve(response({ items: [], total: 0 }));
  }
  return operatorApi(input, init);
}

function failingGovernanceApi(input: RequestInfo | URL, init?: RequestInit) {
  const path = new URL(String(input), "http://localhost").pathname.replace(/^\/api/, "");
  const method = init?.method ?? "GET";
  if (path === "/v1/tool-contracts" && method === "POST") {
    return Promise.resolve(response({ code: "invalid_request", message: "tool_name is already registered" }, 409));
  }
  return operatorApi(input, init);
}

function missingDetailApi(input: RequestInfo | URL, init?: RequestInit) {
  const path = new URL(String(input), "http://localhost").pathname.replace(/^\/api/, "");
  const missingPaths = new Set([
    "/v1/missions/missing/introspect",
    "/v1/mission-proposals/missing",
    "/v1/expansion-requests/missing",
    "/v1/agents/missing",
    "/v1/containment-rules/missing",
  ]);
  if (missingPaths.has(path)) {
    return Promise.resolve(response({ code: "not_found", message: "not found" }, 404));
  }
  return operatorApi(input, init);
}

async function openConsole(path: string) {
  window.history.pushState({}, "", path);
  const user = userEvent.setup();
  render(<App />);
  await user.type(screen.getByLabelText("Bearer token"), "dev-token");
  await user.click(screen.getByRole("button", { name: "Open console" }));
  return user;
}

describe("operator workflows", () => {
  beforeEach(() => vi.stubGlobal("fetch", vi.fn(operatorApi)));
  afterEach(() => {
    cleanup();
    window.history.pushState({}, "", "/");
  });

  it("inspects mission authority, lineage, events, raw data, and completes the mission", async () => {
    const user = await openConsole(`/missions/${mission.mission_ref}`);
    expect(await screen.findByRole("heading", { name: mission.purpose.objective })).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Authority" }));
    expect(screen.getByText("Forbidden")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Lineage" }));
    expect(await screen.findByText("Accessible lineage list")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Events" }));
    expect(await screen.findByText("Mission Approved")).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Raw" }));
    expect(screen.getByRole("button", { name: "Copy Mission record" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Complete" }));
    await user.type(screen.getByLabelText("Reason"), "Objective delivered");
    await user.click(screen.getByRole("button", { name: "Complete mission" }));
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining("/complete"), expect.objectContaining({ method: "POST" }));
  });

  it("creates and approves a bounded mission proposal", async () => {
    const user = await openConsole("/missions/new");
    expect(await screen.findByRole("heading", { name: "Define bounded authority" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Create proposal" }));
    expect(await screen.findByText("Objective must be at least 5 characters")).toBeInTheDocument();
    await user.type(screen.getByRole("textbox", { name: /Objective/ }), "Prepare board packet");
    await user.click(screen.getByRole("button", { name: "Create proposal" }));
    expect(await screen.findByRole("heading", { name: "Prepare board packet" })).toBeInTheDocument();
    const evidence = screen.getByLabelText("Evidence method");
    await user.clear(evidence);
    await user.type(evidence, "change_ticket");
    await user.click(screen.getByRole("button", { name: "Approve and activate" }));
    expect(await screen.findByRole("heading", { name: mission.purpose.objective })).toBeInTheDocument();
    const proposalCall = vi.mocked(fetch).mock.calls.find((call) => String(call[0]).endsWith("/v1/mission-proposals"));
    expect(JSON.parse(String(proposalCall?.[1]?.body))).toEqual(expect.objectContaining({ tenant_id: "demo" }));
  });

  it("shows expansion version drift and records both decision paths", async () => {
    let user = await openConsole(`/approvals/expansions/${expansion.expansion_id}`);
    expect(await screen.findByText("Mission version changed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    await user.type(screen.getByLabelText("Decision reason"), "Reviewed against current scope");
    await user.click(screen.getByRole("button", { name: "Approve" }));
    expect(await screen.findByRole("heading", { name: "Approval queue" })).toBeInTheDocument();
    cleanup();

    user = await openConsole(`/approvals/expansions/${expansion.expansion_id}`);
    await screen.findByText("Mission version changed");
    await user.type(screen.getByLabelText("Decision reason"), "Scope is too broad");
    await user.click(screen.getByRole("button", { name: "Deny" }));
    expect(await screen.findByRole("heading", { name: "Approval queue" })).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining("/deny"), expect.objectContaining({ method: "POST" }));
  });

  it("revokes an agent and lifts containment after inspecting blast radius", async () => {
    let user = await openConsole(`/agents/${agent.agent_id}`);
    expect(await screen.findByRole("heading", { name: agent.agent.client_id })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Revoke" }));
    await user.type(screen.getByLabelText("Reason"), "Key rotation");
    await user.click(screen.getByRole("button", { name: "Revoke agent" }));
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining("/revoke"), expect.objectContaining({ method: "POST" }));
    cleanup();

    user = await openConsole(`/containment/${containment.rule_id}`);
    expect(await screen.findByRole("heading", { name: `mission: ${mission.mission_ref}` })).toBeInTheDocument();
    expect(await screen.findByText("Affected missions")).toBeInTheDocument();
    expect(screen.getAllByText("1")).toHaveLength(6);
    await user.click(screen.getByRole("button", { name: "Lift rule" }));
    await user.type(screen.getByLabelText("Reason"), "Incident resolved");
    await user.click(screen.getByRole("button", { name: "Lift containment" }));
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining("/lift"), expect.objectContaining({ method: "POST" }));
  });

  it("creates containment and governance controls", async () => {
    let user = await openConsole("/containment");
    await screen.findByRole("heading", { name: "Containment" });
    await user.click(screen.getByRole("button", { name: "New containment" }));
    const band = screen.getByRole("heading", { name: "Create containment rule" }).closest("section")!;
    await user.type(within(band).getByLabelText("Target ID"), mission.mission_ref);
    await user.type(within(band).getByLabelText("Reason"), "Emergency stop");
    await user.click(within(band).getByRole("button", { name: "Activate containment" }));
    expect(fetch).toHaveBeenCalledWith(expect.stringMatching(/containment-rules$/), expect.objectContaining({ method: "POST" }));
    cleanup();

    user = await openConsole("/governance");
    await screen.findByRole("heading", { name: "Governance" });
    await user.click(screen.getByRole("button", { name: "Approval rule" }));
    await user.click(screen.getByRole("button", { name: "Create rule" }));
    expect(fetch).toHaveBeenCalledWith(expect.stringMatching(/approval-rules$/), expect.objectContaining({ method: "POST" }));
    await user.click(screen.getByRole("button", { name: "Tool contract" }));
    await user.type(screen.getByLabelText("Tool name"), "slack.post");
    await user.type(screen.getByLabelText("Resource type"), "slack_channel");
    await user.click(screen.getByRole("button", { name: "Create contract" }));
    expect(fetch).toHaveBeenCalledWith(expect.stringMatching(/tool-contracts$/), expect.objectContaining({ method: "POST" }));
  });

  it("revokes a projection and verifies both evidence formats", async () => {
    let user = await openConsole("/projections");
    await screen.findByRole("heading", { name: "Projections" });
    await user.click(await screen.findByRole("button", { name: "Revoke projection" }));
    await user.type(screen.getByLabelText("Reason"), "Credential superseded");
    await user.click(screen.getByRole("button", { name: "Revoke projection" }));
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining(`/projections/${projection.projection_id}/revoke`), expect.objectContaining({ method: "POST" }));
    cleanup();

    user = await openConsole("/workbench");
    await screen.findByRole("heading", { name: "Authority workbench" });
    await user.type(screen.getByLabelText("Signed decision artifact"), "artifact-value");
    await user.click(screen.getByRole("button", { name: "Verify artifact" }));
    expect(await screen.findByText(/"mission_ref": "mref-board"/)).toBeInTheDocument();
    await user.type(screen.getByLabelText("Projection token"), "projection-value");
    await user.click(screen.getByRole("button", { name: "Verify projection" }));
    expect(await screen.findByText(/"projection_id": "projection-1"/)).toBeInTheDocument();
  });

  it("opens mobile navigation, reaches the not-found state, and signs out", async () => {
    const user = await openConsole("/");
    await screen.findByRole("heading", { name: "Authority overview" });
    await user.click(screen.getByRole("button", { name: "Open navigation" }));
    expect(screen.getAllByRole("button", { name: "Close navigation" })).toHaveLength(2);
    await user.click(screen.getByRole("button", { name: "Sign out" }));
    expect(await screen.findByRole("heading", { name: "Administrator access" })).toBeInTheDocument();
    cleanup();

    await openConsole("/missing");
    expect(await screen.findByText("Page not found")).toBeInTheDocument();
  });

  it("renders empty operational states without inventing authority data", async () => {
    vi.stubGlobal("fetch", vi.fn(emptyOperatorApi));
    const user = await openConsole("/");
    expect(await screen.findByText("No expansion decisions waiting.")).toBeInTheDocument();
    expect(screen.getByText("No events recorded.")).toBeInTheDocument();
    expect(screen.getByText("0 total")).toBeInTheDocument();

    await user.click(screen.getByRole("link", { name: "Missions" }));
    expect(await screen.findByText("No missions found")).toBeInTheDocument();
    await user.click(screen.getByRole("link", { name: "Agents" }));
    expect(await screen.findByText("No agents found")).toBeInTheDocument();
    await user.click(screen.getByRole("link", { name: "Containment" }));
    expect(await screen.findByText("No containment rules recorded.")).toBeInTheDocument();
    await user.click(screen.getByRole("link", { name: "Governance" }));
    expect(await screen.findByText("No approval rules.")).toBeInTheDocument();
    expect(screen.getByText("No tool contracts.")).toBeInTheDocument();
    await user.click(screen.getByRole("link", { name: "Projections" }));
    expect(await screen.findByText("No projections")).toBeInTheDocument();
  });

  it("opens audit evidence and exercises optional system metadata", async () => {
    const user = await openConsole("/audit");
    await screen.findByRole("heading", { name: "Audit events" });
    await user.type(screen.getByPlaceholderText("Search event, mission, causation, or correlation"), "board");
    await user.type(screen.getByPlaceholderText("Exact event type"), "mission.approved");
    await user.click(await screen.findByRole("button", { name: /Mission Approved/ }));
    expect(screen.getAllByText("Not set", { selector: "dd" })).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Copy Event payload" })).toBeInTheDocument();
  });

  it("keeps audit filtering recoverable when no events match", async () => {
    vi.stubGlobal("fetch", vi.fn(filteredAuditApi));
    const user = await openConsole("/audit");
    await screen.findByRole("heading", { name: "Audit events" });
    await user.type(screen.getByPlaceholderText("Search event, mission, causation, or correlation"), "no matching evidence");
    expect(await screen.findByText("No events match these filters")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(await screen.findByText("Mission Approved")).toBeInTheDocument();
  });

  it("shows governance validation and server failures inline", async () => {
    vi.stubGlobal("fetch", vi.fn(failingGovernanceApi));
    const user = await openConsole("/governance");
    await screen.findByRole("heading", { name: "Governance" });
    await user.click(screen.getByRole("button", { name: "Tool contract" }));
    await user.click(screen.getByRole("button", { name: "Create contract" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Tool name, resource type, resource ID parameter, and operation are required.");
    await user.type(screen.getByLabelText("Tool name"), "drive.read");
    await user.type(screen.getByLabelText("Resource type"), "drive_folder");
    await user.click(screen.getByRole("button", { name: "Create contract" }));
    expect(await screen.findByText("tool_name is already registered")).toBeInTheDocument();
  });

  it("keeps local exits and retry actions on failed detail routes", async () => {
    vi.stubGlobal("fetch", vi.fn(missingDetailApi));
    await openConsole("/missions/missing");
    expect(await screen.findByRole("heading", { name: "Mission unavailable" })).toBeInTheDocument();
    expect(within(screen.getByRole("main")).getByRole("link", { name: "Missions" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    cleanup();

    await openConsole("/approvals/proposals/missing");
    expect(await screen.findByRole("heading", { name: "Proposal unavailable" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Approval queue" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    cleanup();

    await openConsole("/approvals/expansions/missing");
    expect(await screen.findByRole("heading", { name: "Expansion unavailable" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Approval queue" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    cleanup();

    await openConsole("/agents/missing");
    expect(await screen.findByRole("heading", { name: "Agent unavailable" })).toBeInTheDocument();
    expect(within(screen.getByRole("main")).getByRole("link", { name: "Agents" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
    cleanup();

    await openConsole("/containment/missing");
    expect(await screen.findByRole("heading", { name: "Containment unavailable" })).toBeInTheDocument();
    expect(within(screen.getByRole("main")).getByRole("link", { name: "Containment" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  }, 12_000);

  it("renders a terminal mission with absent optional policy fields", async () => {
    vi.stubGlobal("fetch", vi.fn(terminalMissionApi));
    await openConsole(`/missions/${mission.mission_ref}`);
    expect(await screen.findByText("No conditional authority checks.")).toBeInTheDocument();
    expect(screen.getByText("Strict Subset")).toBeInTheDocument();
    expect(screen.getByText("Disabled")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Complete" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Revoke" })).not.toBeInTheDocument();
  });

  it("validates the administrator credential before connecting", async () => {
    window.history.pushState({}, "", "/");
    const user = userEvent.setup();
    render(<App />);
    await user.click(screen.getByRole("button", { name: "Open console" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Administrator token is required");
    expect(fetch).not.toHaveBeenCalled();
  });
});
