export type MissionState =
  | "draft"
  | "pending_approval"
  | "active"
  | "suspended"
  | "completed"
  | "revoked"
  | "expired"
  | "rejected";

export interface Principal {
  subject: string;
  issuer: string;
  tenant_subject?: string;
  grant_ref?: string;
}

export interface Agent {
  provider: string;
  client_id: string;
  instance_id: string;
  key_thumbprint?: string;
}

export interface ResourceGrant {
  type: string;
  id: string;
  actions: string[];
  constraints?: Record<string, unknown>;
}

export interface AuthorityRegion {
  resources: ResourceGrant[];
  forbidden_actions?: string[];
}

export interface Condition {
  id: string;
  expression: string;
  evaluation?: string;
  on_failure?: string;
}

export interface Mission {
  mission_id: string;
  mission_ref: string;
  tenant_id: string;
  state: MissionState;
  version: number;
  principal: Principal;
  agent: Agent;
  purpose: {
    objective: string;
    business_context?: string;
    template?: string;
  };
  authority_region: AuthorityRegion;
  conditions?: Condition[];
  lifecycle: {
    created_at?: string;
    not_before?: string;
    expires_at?: string;
    terminal_events?: string[];
    on_expiry?: string;
  };
  delegation: {
    permitted: boolean;
    max_depth: number;
    current_depth: number;
    attenuation?: string;
    cascade_revocation: boolean;
    parent_mission_ref?: string;
  };
  risk?: {
    default_mode?: string;
    sync_required_for?: string[];
    fail_closed_for?: string[];
  };
  approval_evidence?: ApprovalEvidence;
}

export interface ApprovalEvidence {
  approver?: Principal;
  approved_at?: string;
  display_hash?: string;
  method?: string;
}

export interface MissionProposal {
  proposal_id: string;
  status: MissionState;
  tenant_id: string;
  principal: Principal;
  agent: Agent;
  intent: { objective: string; business_context?: string; template?: string };
  authority_region: AuthorityRegion;
  conditions?: Condition[];
  lifecycle: Mission["lifecycle"];
  delegation: Mission["delegation"];
  risk?: Mission["risk"];
  created_at: string;
}

export interface ExpansionRequest {
  expansion_id: string;
  mission_ref: string;
  mission_version_seen?: number;
  tenant_id: string;
  requester: Actor;
  action: Action;
  context?: Record<string, unknown>;
  requested_authority: AuthorityRegion;
  justification?: string;
  status: "pending" | "approved" | "denied";
  created_at: string;
  decided_at?: string;
  approver?: Principal;
  approval_evidence?: ApprovalEvidence;
}

export interface Actor {
  agent_instance_id: string;
  client_id: string;
  key_thumbprint?: string;
}

export interface Action {
  type?: string;
  name?: string;
  resource: { type: string; id: string };
  operation: string;
}

export interface AgentIdentity {
  agent_id: string;
  tenant_id: string;
  agent: Agent;
  public_key: string;
  key_thumbprint: string;
  status: "active" | "revoked";
  created_at: string;
  revoked_at?: string;
}

export interface ContainmentRule {
  rule_id: string;
  tenant_id?: string;
  target_type: "agent" | "principal" | "tool" | "resource" | "mission" | "tenant";
  target_id: string;
  status: "active" | "lifted";
  reason?: string;
  created_by?: Principal;
  metadata?: Record<string, unknown>;
  created_at: string;
  expires_at?: string;
  lifted_at?: string;
}

export interface Projection {
  projection_id: string;
  mission_ref: string;
  mission_version: number;
  tenant_id?: string;
  type: string;
  actor: Actor;
  claims?: Record<string, unknown>;
  status: "active" | "revoked" | "expired";
  issued_at: string;
  expires_at: string;
  revoked_at?: string;
}

export interface ApprovalRule {
  rule_id: string;
  tenant_id?: string;
  applies_to: string;
  resource_type?: string;
  resource_id?: string;
  operation?: string;
  risk_level?: string;
  required_approvals: number;
  allowed_subjects?: string[];
  allowed_issuers?: string[];
  created_by?: Principal;
  created_at: string;
}

export interface ToolContract {
  tool_name: string;
  resource_type: string;
  resource_id?: string;
  resource_id_param?: string;
  operation: string;
  operation_param?: string;
  action_type?: string;
  required_context?: string[];
  metadata?: Record<string, string>;
  created_by?: Principal;
  created_at?: string;
}

export interface EventRecord {
  event_id: string;
  mission_ref?: string;
  tenant_id?: string;
  type: string;
  actor?: Record<string, unknown>;
  payload?: Record<string, unknown>;
  version_before?: number;
  version_after?: number;
  occurred_at: string;
  causation_id?: string;
  correlation_id?: string;
}

export interface LineageNode {
  id: string;
  type: string;
  label: string;
  metadata?: Record<string, unknown>;
}

export interface LineageEdge {
  from: string;
  to: string;
  type: string;
  metadata?: Record<string, unknown>;
}

export interface LineageGraph {
  nodes: LineageNode[];
  edges: LineageEdge[];
}

export interface BlastRadius {
  rule: ContainmentRule;
  missions?: Mission[];
  projections?: Projection[];
  leases?: unknown[];
  expansion_requests?: ExpansionRequest[];
  agents?: AgentIdentity[];
  tool_contracts?: ToolContract[];
}

export interface CollectionPage<T> {
  items: T[];
  next_cursor?: string;
  total: number;
}

export interface AdminSession {
  principal: Principal;
  capabilities: Record<string, boolean>;
  api_version: string;
}

export interface OperationsSummary {
  missions_total: number;
  missions_by_state: Partial<Record<MissionState, number>>;
  pending_proposals: number;
  pending_expansions: number;
  active_containments: number;
  active_agents: number;
  active_projections: number;
  recent_event_count: number;
  service_capabilities: Record<string, boolean>;
}

export interface ListParams {
  tenant_id?: string;
  state?: string;
  status?: string;
  type?: string;
  q?: string;
  cursor?: string;
  limit?: number;
}

export interface ApiErrorBody {
  error?: string;
  code?: string;
  message?: string;
}
