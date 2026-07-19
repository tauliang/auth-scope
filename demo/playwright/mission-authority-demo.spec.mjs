import playwrightTest from "../../frontend/node_modules/@playwright/test/index.js";
import {
  actorFromState,
  adminRequest,
  adminTokensFromEnv,
  apiUrlFromEnv,
  loadState,
  nonce,
  publicRequest,
  saveState,
  signedRequest,
} from "../lib/auth-scope-demo.mjs";

const { expect, test } = playwrightTest;

test.describe.configure({ mode: "serial" });

test("primary mission authority demo flow", async ({ page }) => {
  const tokens = adminTokensFromEnv();
  const apiUrl = apiUrlFromEnv();
  let state = await loadState();

  await loginAsAlice(page, tokens.alice);
  await expect(page.getByRole("heading", { name: "Authority overview" })).toBeVisible();
  await expect(page.getByText("Pending approvals")).toBeVisible();

  await approvePendingExpansion(page, state);
  state = await completePostApprovalRuntimeEvidence({ apiUrl, token: tokens.alice, state });

  await openMissionAuthority(page, state, tokens.alice);
  await openAgentLineage(page, state);
  await openProjectionInventory(page, state);
  await openContainmentBlastRadius(page, state);
  await verifyEvidenceInWorkbench(page, state);
  await openAuditTrail(page, state);
});

async function loginAsAlice(page, token, path = "/") {
  await page.goto(path);
  const loginHeading = page.getByRole("heading", { name: "Administrator access" });
  if (await loginHeading.isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.getByLabel("Bearer token").fill(token);
    await page.getByRole("button", { name: "Open console" }).click();
  }
}

async function approvePendingExpansion(page, state) {
  const expansion = state.expansion;
  await page.getByRole("link", { name: "Approvals" }).click();
  await expect(page.getByRole("heading", { name: "Approval queue" })).toBeVisible();
  await page.getByRole("link", { name: new RegExp(escapeRegex(expansion.action.resource.id)) }).click();
  await expect(page.getByRole("heading", {
    name: `${expansion.action.operation} on ${expansion.action.resource.id}`,
  })).toBeVisible();
  await expect(page.getByRole("link", { name: "Approval queue" })).toBeVisible();

  const approveButton = page.getByRole("button", { name: "Approve" });
  await page.getByLabel("Decision reason").fill(state.scenario.ui.approval_reason);
  if (await approveButton.isEnabled().catch(() => false)) {
    await approveButton.click();
    await expect(page.getByRole("heading", { name: "Approval queue" })).toBeVisible();
    await expect(page.getByText(expansion.action.resource.id)).toBeHidden();
  } else {
    await expect(page.getByText("approved")).toBeVisible();
  }
}

async function completePostApprovalRuntimeEvidence({ apiUrl, token, state }) {
  const actor = actorFromState(state);
  const mission = await adminRequest({
    apiUrl,
    token,
    path: `/v1/missions/${state.mission.mission_ref}/introspect`,
  });

  if (!state.post_approval?.tool_decision) {
    const toolDecision = await signedRequest({
      apiUrl,
      agentId: state.agent.agent_id,
      privateKeyPem: state.agent.demo_only_private_key_pem,
      path: "/v1/tool-calls/authorize",
      nonce: nonce(state.run_id, "tool-authorize-after-approval"),
      body: {
        mission_ref: state.mission.mission_ref,
        mission_version_seen: mission.version,
        actor,
        tool_name: state.tool_contract.tool_name,
        arguments: {
          channel_id: state.expansion.action.resource.id,
          message: "Demo board packet status is ready for review.",
        },
        context: state.scenario.contexts.tool_gateway,
      },
    });

    const staleLease = await signedRequest({
      apiUrl,
      agentId: state.agent.agent_id,
      privateKeyPem: state.agent.demo_only_private_key_pem,
      path: `/v1/leases/${state.lease.lease_id}/refresh`,
      nonce: nonce(state.run_id, "refresh-stale-lease"),
      body: {
        actor,
        ttl_seconds: state.scenario.lease.ttl_seconds,
      },
    });

    state = {
      ...state,
      mission: {
        ...state.mission,
        mission_version_after_approval: mission.version,
      },
      post_approval: {
        tool_decision: toolDecision,
        stale_lease_refresh: staleLease,
      },
    };
  }

  if (!state.containment?.rule) {
    const containmentRule = await adminRequest({
      apiUrl,
      token,
      method: "POST",
      path: "/v1/containment-rules",
      body: {
        tenant_id: state.tenant_id,
        target_type: state.scenario.containment.target_type,
        target_id: state.agent.instance_id,
        reason: state.scenario.containment.reason,
        metadata: state.scenario.containment.metadata,
      },
    });

    const containedDecision = await signedRequest({
      apiUrl,
      agentId: state.agent.agent_id,
      privateKeyPem: state.agent.demo_only_private_key_pem,
      path: `/v1/missions/${state.mission.mission_ref}/evaluate`,
      nonce: nonce(state.run_id, "evaluate-contained-agent"),
      body: {
        mission_version_seen: state.mission.mission_version_after_approval ?? state.mission.mission_version,
        actor,
        action: state.scenario.actions.in_scope,
        context: state.scenario.contexts.in_scope,
      },
    });

    const blastRadius = await adminRequest({
      apiUrl,
      token,
      path: `/v1/containment-rules/${containmentRule.rule_id}/blast-radius`,
    });

    const projectionVerification = await publicRequest({
      apiUrl,
      method: "POST",
      path: "/v1/projections/verify",
      body: { token: state.projection.token },
    });

    state = {
      ...state,
      containment: {
        rule: containmentRule,
        blast_radius: blastRadius,
        contained_decision: containedDecision,
        projection_verification_after_containment: projectionVerification,
      },
      demo_urls: {
        ...state.demo_urls,
        containment: `${state.frontend_url}/containment/${containmentRule.rule_id}`,
      },
    };
  }

  await saveState(state);
  return state;
}

async function openMissionAuthority(page, state, token) {
  await loginAsAlice(page, token, `/missions/${state.mission.mission_ref}`);
  await expect(page.getByRole("heading", { name: state.mission.objective })).toBeVisible();
  await page.getByRole("tab", { name: "Authority" }).click();
  await expect(page.getByText(state.scenario.actions.in_scope.resource.id)).toBeVisible();
  await expect(page.getByText(state.expansion.action.resource.id)).toBeVisible();
  await expect(page.getByText("post_update")).toBeVisible();
  await page.getByRole("tab", { name: "Lineage" }).click();
  await expect(page.getByText("Accessible lineage list")).toBeVisible();
}

async function openAgentLineage(page, state) {
  await page.getByRole("link", { name: "Agents" }).click();
  await expect(page.getByRole("heading", { name: "Agents" })).toBeVisible();
  await page.getByPlaceholder("Search agent, instance, provider, or key").fill(state.agent.agent_id);
  await expect(page.getByText(state.agent.client_id)).toBeVisible();
  await page.getByRole("link", { name: `Open ${state.agent.client_id}` }).click();
  await expect(page.getByRole("heading", { name: state.agent.client_id })).toBeVisible();
  await expect(page.getByText("Authority lineage")).toBeVisible();
  await expect(page.getByText(state.agent.key_thumbprint)).toBeVisible();
}

async function openProjectionInventory(page, state) {
  await page.getByRole("link", { name: "Projections" }).click();
  await expect(page.getByRole("heading", { name: "Projections" })).toBeVisible();
  await page.getByPlaceholder("Search projection, mission, or actor").fill(state.projection.projection_id);
  await expect(page.getByText(state.agent.client_id)).toBeVisible();
  await expect(page.getByText("Mcp Context")).toBeVisible();
}

async function openContainmentBlastRadius(page, state) {
  await page.getByRole("link", { name: "Containment" }).click();
  await expect(page.getByRole("heading", { name: "Containment" })).toBeVisible();
  await expect(page.getByText(`${state.scenario.containment.target_type}: ${state.agent.instance_id}`)).toBeVisible();
  await page.getByRole("link", { name: new RegExp(escapeRegex(state.agent.instance_id)) }).click();
  await expect(page.getByRole("heading", { name: `${state.scenario.containment.target_type}: ${state.agent.instance_id}` })).toBeVisible();
  await expect(page.getByText("Blast radius")).toBeVisible();
  await expect(page.getByText("Affected missions")).toBeVisible();
  await expect(page.getByText(state.mission.objective)).toBeVisible();
}

async function verifyEvidenceInWorkbench(page, state) {
  await page.getByRole("link", { name: "Workbench" }).click();
  await expect(page.getByRole("heading", { name: "Authority workbench" })).toBeVisible();

  await page.getByLabel("Signed decision artifact").fill(state.decisions.expansion_required.decision_artifact);
  await page.getByRole("button", { name: "Verify artifact" }).click();
  await expect(page.getByText("Verification complete").first()).toBeVisible();
  await expect(page.getByText("\"decision\": \"require_approval\"")).toBeVisible();

  await page.getByLabel("Projection token").fill(state.projection.token);
  await page.getByRole("button", { name: "Verify projection" }).click();
  await expect(page.getByText("projection blocked by containment rule")).toBeVisible();
}

async function openAuditTrail(page, state) {
  await page.getByRole("link", { name: "Audit" }).click();
  await expect(page.getByRole("heading", { name: "Audit events" })).toBeVisible();
  await page.getByPlaceholder("Search event, mission, causation, or correlation").fill(state.mission.mission_ref);
  await expect(page.getByText("Mission Expansion Approved")).toBeVisible();
}

function escapeRegex(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
