import type {
  AdminSession,
  AgentIdentity,
  ApprovalRule,
  BlastRadius,
  CollectionPage,
  ContainmentRule,
  EventRecord,
  ExpansionRequest,
  LineageGraph,
  ListParams,
  Mission,
  MissionProposal,
  OperationsSummary,
  Projection,
  ToolContract,
  ApiErrorBody,
} from "./types";

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string,
    readonly requestId: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export class ApiClient {
  constructor(
    private readonly token: string,
    private readonly basePath = "/api",
  ) {}

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("accept", "application/json");
    headers.set("authorization", `Bearer ${this.token}`);
    headers.set("x-request-id", crypto.randomUUID());
    if (init.body && !headers.has("content-type")) {
      headers.set("content-type", "application/json");
    }
    const response = await fetch(`${this.basePath}${path}`, { ...init, headers });
    const requestId = response.headers.get("x-request-id") ?? "unavailable";
    if (!response.ok) {
      let body: ApiErrorBody;
      try {
        body = (await response.json()) as ApiErrorBody;
      } catch {
        body = {};
      }
      throw new ApiError(
        body.message ?? body.error ?? `Request failed with status ${response.status}`,
        response.status,
        body.code ?? "request_failed",
        requestId,
      );
    }
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  private query(path: string, params: ListParams = {}) {
    const search = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== "") search.set(key, String(value));
    });
    const suffix = search.size ? `?${search.toString()}` : "";
    return `${path}${suffix}`;
  }

  getSession() {
    return this.request<AdminSession>("/v1/admin/session");
  }

  getSummary(params: ListParams = {}) {
    return this.request<OperationsSummary>(this.query("/v1/operations/summary", params));
  }

  listMissions(params: ListParams = {}) {
    return this.request<CollectionPage<Mission>>(this.query("/v1/missions", params));
  }

  getMission(ref: string) {
    return this.request<Mission>(`/v1/missions/${encodeURIComponent(ref)}/introspect`);
  }

  createProposal(body: unknown) {
    return this.request<{ proposal_id: string; status: string; approval_url: string }>("/v1/mission-proposals", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  listProposals(params: ListParams = {}) {
    return this.request<CollectionPage<MissionProposal>>(this.query("/v1/mission-proposals", params));
  }

  getProposal(id: string) {
    return this.request<MissionProposal>(`/v1/mission-proposals/${encodeURIComponent(id)}`);
  }

  approveProposal(id: string, method: string) {
    return this.request<{ mission_ref: string; mission_version: number; state: string }>(
      `/v1/mission-proposals/${encodeURIComponent(id)}/approve`,
      { method: "POST", body: JSON.stringify({ approval_evidence: { method } }) },
    );
  }

  listExpansions(params: ListParams = {}) {
    return this.request<CollectionPage<ExpansionRequest>>(this.query("/v1/expansion-requests", params));
  }

  getExpansion(id: string) {
    return this.request<ExpansionRequest>(`/v1/expansion-requests/${encodeURIComponent(id)}`);
  }

  approveExpansion(id: string, body: { reason?: string; approval_evidence?: { method: string } }) {
    return this.request<{ expansion_id: string; status: string; approvals_required: number; approvals_received: number; mission_ref: string }>(
      `/v1/expansion-requests/${encodeURIComponent(id)}/approvals`,
      { method: "POST", body: JSON.stringify(body) },
    );
  }

  denyExpansion(id: string, reason: string) {
    return this.request(`/v1/expansion-requests/${encodeURIComponent(id)}/deny`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  listAgents(params: ListParams = {}) {
    return this.request<CollectionPage<AgentIdentity>>(this.query("/v1/agents", params));
  }

  getAgent(id: string) {
    return this.request<AgentIdentity>(`/v1/agents/${encodeURIComponent(id)}`);
  }

  revokeAgent(id: string, reason: string) {
    return this.request<AgentIdentity>(`/v1/agents/${encodeURIComponent(id)}/revoke`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  listContainment() {
    return this.request<{ containment_rules: ContainmentRule[] }>("/v1/containment-rules");
  }

  getContainment(id: string) {
    return this.request<ContainmentRule>(`/v1/containment-rules/${encodeURIComponent(id)}`);
  }

  createContainment(body: Partial<ContainmentRule>) {
    return this.request<ContainmentRule>("/v1/containment-rules", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  liftContainment(id: string, reason: string) {
    return this.request<ContainmentRule>(`/v1/containment-rules/${encodeURIComponent(id)}/lift`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  getBlastRadius(id: string) {
    return this.request<BlastRadius>(`/v1/containment-rules/${encodeURIComponent(id)}/blast-radius`);
  }

  listApprovalRules() {
    return this.request<{ approval_rules: ApprovalRule[] }>("/v1/approval-rules");
  }

  createApprovalRule(body: Partial<ApprovalRule>) {
    return this.request<ApprovalRule>("/v1/approval-rules", { method: "POST", body: JSON.stringify(body) });
  }

  listToolContracts(params: ListParams = {}) {
    return this.request<CollectionPage<ToolContract>>(this.query("/v1/tool-contracts", params));
  }

  createToolContract(body: Partial<ToolContract>) {
    return this.request<ToolContract>("/v1/tool-contracts", { method: "POST", body: JSON.stringify(body) });
  }

  listProjections(params: ListParams = {}) {
    return this.request<CollectionPage<Projection>>(this.query("/v1/projections", params));
  }

  revokeProjection(id: string, reason: string) {
    return this.request(`/v1/projections/${encodeURIComponent(id)}/revoke`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  listEvents(params: ListParams = {}) {
    return this.request<CollectionPage<EventRecord>>(this.query("/v1/events", params));
  }

  missionLineage(ref: string) {
    return this.request<LineageGraph>(`/v1/missions/${encodeURIComponent(ref)}/lineage`);
  }

  agentLineage(id: string) {
    return this.request<LineageGraph>(`/v1/agents/${encodeURIComponent(id)}/lineage`);
  }

  transitionMission(ref: string, action: "revoke" | "complete", reason: string) {
    return this.request<Mission>(`/v1/missions/${encodeURIComponent(ref)}/${action}`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  verifyDecisionArtifact(decisionArtifact: string) {
    return this.request<Record<string, unknown>>("/v1/decision-artifacts/verify", {
      method: "POST",
      body: JSON.stringify({ decision_artifact: decisionArtifact }),
    });
  }

  verifyProjection(token: string) {
    return this.request<Record<string, unknown>>("/v1/projections/verify", {
      method: "POST",
      body: JSON.stringify({ token }),
    });
  }
}
