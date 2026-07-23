import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";

function jsonResponse(body: unknown, status = 200, requestId = "req-test") {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json", "x-request-id": requestId } });
}

describe("ApiClient", () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    fetchMock.mockReset();
    fetchMock.mockImplementation(() => Promise.resolve(jsonResponse({ items: [], total: 0 })));
    vi.stubGlobal("fetch", fetchMock);
  });

  it("adds security and correlation headers", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ principal: { subject: "alice", issuer: "issuer" }, capabilities: {}, api_version: "v1" }));
    const client = new ApiClient("secret");
    await client.getSession();
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/admin/session");
    const headers = init.headers as Headers;
    expect(headers.get("authorization")).toBe("Bearer secret");
    expect(headers.get("x-request-id")).toBeTruthy();
    expect(headers.get("accept")).toBe("application/json");
  });

  it("normalizes API failures without exposing credentials", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ code: "conflict", message: "Version changed" }, 409, "req-conflict"));
    const client = new ApiClient("top-secret");
    const error = await client.getMission("mref").catch((caught) => caught as ApiError);
    expect(error).toEqual(expect.objectContaining({
      message: "Version changed", status: 409, code: "conflict", requestId: "req-conflict",
    }));
    expect(String(error)).not.toContain("top-secret");
  });

  it("covers all operator endpoints and query serialization", async () => {
    const client = new ApiClient("token", "/custom");
    const calls = [
      client.getSummary({ tenant_id: "demo" }), client.listMissions({ q: "board", limit: 20 }), client.getMission("m/ref"),
      client.createProposal({ objective: "test" }), client.listProposals(), client.getProposal("proposal"), client.approveProposal("proposal", "console"),
      client.listExpansions({ status: "pending" }), client.getExpansion("expansion"), client.approveExpansion("expansion", { reason: "yes" }), client.denyExpansion("expansion", "no"),
      client.listAgents(), client.getAgent("agent"), client.revokeAgent("agent", "rotate"),
      client.listContainment(), client.getContainment("rule"), client.createContainment({ target_id: "mission" }), client.liftContainment("rule", "resolved"), client.getBlastRadius("rule"),
      client.listApprovalRules(), client.createApprovalRule({ required_approvals: 2 }),
      client.listPolicyBundles(), client.createPolicyBundle({ version: "mission-policy/test", rules: [{ rule_id: "deny-delete", effect: "deny" }] }),
      client.activatePolicyBundle("policy/bundle", "approved"), client.simulatePolicyBundle("policy/bundle", { mission_ref: "mission", evaluation: { actor: { agent_instance_id: "inst", client_id: "agent" }, action: { type: "tool_call", resource: { type: "drive", id: "board" }, operation: "read" } } }),
      client.listToolContracts(), client.createToolContract({ tool_name: "drive.read" }),
      client.listProjections(), client.revokeProjection("projection", "stale"), client.listEvents({ type: "mission.created" }),
      client.missionLineage("mission"), client.agentLineage("agent"), client.transitionMission("mission", "complete", "done"),
      client.verifyDecisionArtifact("artifact"), client.verifyProjection("projection-token"),
    ];
    await Promise.all(calls);
    expect(fetchMock).toHaveBeenCalledTimes(calls.length);
    const urls = fetchMock.mock.calls.map((call) => String(call[0]));
    expect(urls).toContain("/custom/v1/missions?q=board&limit=20");
    expect(urls).toContain("/custom/v1/missions/m%2Fref/introspect");
    expect(urls).toContain("/custom/v1/policy-bundles/policy%2Fbundle/activate");
    expect(urls).toContain("/custom/v1/policy-bundles/policy%2Fbundle/simulate");
    expect(fetchMock.mock.calls.some(([, init]) => (init as RequestInit).method === "POST")).toBe(true);
  });

  it("falls back for malformed errors", async () => {
    fetchMock.mockResolvedValueOnce(new Response("not json", { status: 500 }));
    await expect(new ApiClient("token").getSummary()).rejects.toEqual(expect.objectContaining({ code: "request_failed", status: 500 }));
  });

  it("supports empty responses and legacy error envelopes", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(new ApiClient("token").revokeProjection("projection", "done")).resolves.toBeUndefined();
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: "Legacy denial" }, 403, ""));
    await expect(new ApiClient("token").getSession()).rejects.toEqual(expect.objectContaining({ message: "Legacy denial", code: "request_failed" }));
  });
});
