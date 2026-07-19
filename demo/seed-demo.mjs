#!/usr/bin/env node
import { pathToFileURL } from "node:url";
import {
  actorFromState,
  adminRequest,
  adminTokensFromEnv,
  apiUrlFromEnv,
  createEd25519AgentKeys,
  defaultStatePath,
  frontendUrlFromEnv,
  loadScenario,
  nonce,
  publicRequest,
  requireString,
  saveState,
  short,
  signedRequest,
  waitForHealth,
} from "./lib/auth-scope-demo.mjs";

export async function seedDemo({
  apiUrl = apiUrlFromEnv(),
  frontendUrl = frontendUrlFromEnv(),
  statePath = process.env.AUTH_SCOPE_DEMO_STATE ?? defaultStatePath,
  runId,
  log = true,
} = {}) {
  const { scenario, runId: materializedRunId } = await loadScenario({ runId });
  const tokens = adminTokensFromEnv();
  const keys = createEd25519AgentKeys();

  await waitForHealth(apiUrl);

  const registeredAgent = await adminRequest({
    apiUrl,
    token: tokens.alice,
    method: "POST",
    path: "/v1/agents",
    body: {
      tenant_id: scenario.tenant_id,
      agent: scenario.agent,
      public_key: keys.publicKey,
    },
  });

  const proposalRequest = buildProposalRequest(scenario, registeredAgent.key_thumbprint);
  const proposal = await adminRequest({
    apiUrl,
    token: tokens.alice,
    method: "POST",
    path: "/v1/mission-proposals",
    body: proposalRequest,
  });

  const mission = await adminRequest({
    apiUrl,
    token: tokens.alice,
    method: "POST",
    path: `/v1/mission-proposals/${proposal.proposal_id}/approve`,
    body: {
      approval_evidence: {
        method: "demo_seed",
        display_hash: `sha256:demo-${materializedRunId}`,
      },
    },
  });

  const baseState = {
    schema_version: 1,
    run_id: materializedRunId,
    scenario_name: scenario.scenario_name,
    created_at: new Date().toISOString(),
    api_url: apiUrl,
    frontend_url: frontendUrl,
    tenant_id: scenario.tenant_id,
    operators: scenario.operators,
    agent: {
      agent_id: registeredAgent.agent_id,
      provider: scenario.agent.provider,
      client_id: scenario.agent.client_id,
      instance_id: scenario.agent.instance_id,
      key_thumbprint: registeredAgent.key_thumbprint,
      public_key: keys.publicKey,
      demo_only_private_key_pem: keys.privateKeyPem,
    },
    mission: {
      proposal_id: proposal.proposal_id,
      mission_ref: mission.mission_ref,
      mission_version: mission.mission_version,
      objective: scenario.mission.intent.objective,
      principal: scenario.mission.principal,
      base_authority: scenario.mission.authority_region,
    },
    scenario,
  };

  const actor = actorFromState(baseState);

  const inScopeDecision = await signedRequest({
    apiUrl,
    agentId: registeredAgent.agent_id,
    privateKeyPem: keys.privateKeyPem,
    path: `/v1/missions/${mission.mission_ref}/evaluate`,
    nonce: nonce(materializedRunId, "evaluate-in-scope"),
    body: {
      mission_version_seen: mission.mission_version,
      actor,
      action: scenario.actions.in_scope,
      context: scenario.contexts.in_scope,
    },
  });

  const projection = await signedRequest({
    apiUrl,
    agentId: registeredAgent.agent_id,
    privateKeyPem: keys.privateKeyPem,
    path: `/v1/missions/${mission.mission_ref}/projections`,
    nonce: nonce(materializedRunId, "create-projection"),
    body: {
      mission_version_seen: mission.mission_version,
      actor,
      type: scenario.projection.type,
      ttl_seconds: scenario.projection.ttl_seconds,
      claims: scenario.projection.claims,
    },
  });

  const projectionVerification = await publicRequest({
    apiUrl,
    method: "POST",
    path: "/v1/projections/verify",
    body: { token: projection.token },
  });

  const lease = await signedRequest({
    apiUrl,
    agentId: registeredAgent.agent_id,
    privateKeyPem: keys.privateKeyPem,
    path: `/v1/missions/${mission.mission_ref}/leases`,
    nonce: nonce(materializedRunId, "create-lease"),
    body: {
      mission_version_seen: mission.mission_version,
      actor,
      ttl_seconds: scenario.lease.ttl_seconds,
    },
  });

  const approvalRule = await adminRequest({
    apiUrl,
    token: tokens.alice,
    method: "POST",
    path: "/v1/approval-rules",
    body: {
      tenant_id: scenario.tenant_id,
      ...scenario.approval_rule,
    },
  });

  const expansionDecision = await signedRequest({
    apiUrl,
    agentId: registeredAgent.agent_id,
    privateKeyPem: keys.privateKeyPem,
    path: `/v1/missions/${mission.mission_ref}/evaluate`,
    nonce: nonce(materializedRunId, "evaluate-expansion"),
    body: {
      mission_version_seen: mission.mission_version,
      actor,
      action: scenario.actions.expansion,
      context: scenario.contexts.expansion,
    },
  });
  const expansionId = requireString(expansionDecision.constraints?.expansion_request_id, "expansion_request_id");
  const expansion = await adminRequest({
    apiUrl,
    token: tokens.alice,
    path: `/v1/expansion-requests/${expansionId}`,
  });

  const bobApproval = await adminRequest({
    apiUrl,
    token: tokens.bob,
    method: "POST",
    path: `/v1/expansion-requests/${expansionId}/approvals`,
    body: {
      reason: "Demo first approval from risk review.",
      approval_evidence: { method: "demo_seed_bob" },
    },
  });

  const toolContract = await adminRequest({
    apiUrl,
    token: tokens.alice,
    method: "POST",
    path: "/v1/tool-contracts",
    body: scenario.tool_contract,
  });

  const negotiation = await signedRequest({
    apiUrl,
    agentId: registeredAgent.agent_id,
    privateKeyPem: keys.privateKeyPem,
    path: `/v1/missions/${mission.mission_ref}/authority/negotiations`,
    nonce: nonce(materializedRunId, "authority-negotiation"),
    body: {
      mission_version_seen: mission.mission_version,
      actor,
      requested_authority: scenario.actions.negotiation_request,
      context: { "demo.step": "authority-negotiation" },
    },
  });

  const artifactVerification = await publicRequest({
    apiUrl,
    method: "POST",
    path: "/v1/decision-artifacts/verify",
    body: { decision_artifact: expansionDecision.decision_artifact },
  });

  const state = {
    ...baseState,
    approval_rule: approvalRule,
    decisions: {
      in_scope: inScopeDecision,
      expansion_required: expansionDecision,
    },
    expansion: {
      expansion_id: expansionId,
      status: expansion.status,
      action: scenario.actions.expansion,
      requested_authority: expansion.requested_authority,
      first_approval: bobApproval,
    },
    projection: {
      ...projection,
      verification_before_containment: projectionVerification,
    },
    lease,
    tool_contract: toolContract,
    negotiation,
    verification: {
      expansion_decision_artifact: artifactVerification,
    },
    demo_urls: {
      overview: `${frontendUrl}/`,
      approval: `${frontendUrl}/approvals/expansions/${expansionId}`,
      mission: `${frontendUrl}/missions/${mission.mission_ref}`,
      agent: `${frontendUrl}/agents/${registeredAgent.agent_id}`,
      workbench: `${frontendUrl}/workbench`,
    },
  };

  await saveState(state, statePath);

  if (log) {
    console.log(`Seeded ${scenario.scenario_name}`);
    console.log(`Run: ${materializedRunId}`);
    console.log(`Mission: ${mission.mission_ref} (${scenario.mission.intent.objective})`);
    console.log(`Agent: ${registeredAgent.agent_id} ${short(registeredAgent.key_thumbprint, 24)}`);
    console.log(`Pending expansion: ${expansionId} (${bobApproval.approvals_received}/${bobApproval.approvals_required} approvals)`);
    console.log(`State: ${statePath}`);
  }

  return state;
}

function buildProposalRequest(scenario, keyThumbprint) {
  const expiresAt = new Date(Date.now() + Number(scenario.mission.duration_hours ?? 168) * 60 * 60 * 1000);
  return {
    tenant_id: scenario.tenant_id,
    principal: scenario.mission.principal,
    agent: {
      ...scenario.agent,
      key_thumbprint: keyThumbprint,
    },
    intent: scenario.mission.intent,
    authority_region: scenario.mission.authority_region,
    conditions: scenario.mission.conditions,
    lifecycle: {
      expires_at: expiresAt.toISOString(),
    },
    delegation: scenario.mission.delegation,
    risk: scenario.mission.risk,
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  seedDemo().catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
}
