const agentProfiles = {
  codex: {
    label: "Codex",
    agentId: "codex-agent",
    instance: "inst_codex_42",
    thumbprint: "sha256:codex-demo",
    clientId: "codex-cli",
  },
  opencode: {
    label: "OpenCode",
    agentId: "opencode-agent",
    instance: "inst_opencode_42",
    thumbprint: "sha256:opencode-demo",
    clientId: "opencode",
  },
};

const initialMission = {
  ref: "mref_coding_demo",
  version: 1,
  containment: false,
  approvedExpansions: new Set(),
  grants: [
    {
      type: "repo_path",
      id: "frontend/src/features/missions/**",
      actions: ["read_file", "edit_file"],
      constraints: { max_files: 3, requires_tests: true },
    },
    {
      type: "command",
      id: "npm test",
      actions: ["run_test"],
      constraints: { cwd: "frontend" },
    },
    {
      type: "github_pull_request",
      id: "tauliang/auth-scope",
      actions: ["open_pull_request"],
      constraints: { target_branch: "main", requires_check: true },
    },
    {
      type: "identity_context",
      id: "okta:authscope.okta.com",
      actions: ["resolve_claims"],
      constraints: { required_group: "mission-operators" },
    },
    {
      type: "identity_context",
      id: "entra:authscope.onmicrosoft.com",
      actions: ["resolve_claims"],
      constraints: { required_role: "AgentOperator" },
    },
    {
      type: "slack_channel",
      id: "C05MISSION",
      actions: ["post_message"],
      constraints: { workspace_id: "T024AUTH" },
    },
    {
      type: "jira_project",
      id: "MAS",
      actions: ["transition_issue", "comment_issue"],
      constraints: { site_url: "https://authscope.atlassian.net" },
    },
    {
      type: "confluence_space",
      id: "ENG",
      actions: ["update_page"],
      constraints: { site_url: "https://authscope.atlassian.net" },
    },
    {
      type: "servicenow_change",
      id: "CHG0030142",
      actions: ["read_change", "add_work_note"],
      constraints: { state: "implement" },
    },
  ],
};

const actionPlan = [
  {
    id: "read-mission-detail",
    label: "Read mission detail code",
    short: "RD",
    operation: "read_file",
    resourceType: "repo_path",
    resourceId: "frontend/src/features/missions/MissionDetailPage.tsx",
    risk: "low",
    summary: "Inspect the mission detail component before editing.",
  },
  {
    id: "edit-filter",
    label: "Edit mission filter behavior",
    short: "ED",
    operation: "edit_file",
    resourceType: "repo_path",
    resourceId: "frontend/src/features/missions/MissionsPage.tsx",
    risk: "low",
    summary: "Patch the bug inside the approved frontend mission surface.",
  },
  {
    id: "run-tests",
    label: "Run frontend tests",
    short: "TS",
    operation: "run_test",
    resourceType: "command",
    resourceId: "npm test",
    risk: "low",
    summary: "Run tests in the authorized frontend working directory.",
  },
  {
    id: "install-package",
    label: "Install package for table filtering",
    short: "PK",
    operation: "install_dependency",
    resourceType: "package_manifest",
    resourceId: "frontend/package.json",
    risk: "medium",
    expansionKey: "dependency_install",
    summary: "Package changes are outside the original mission and need approval.",
  },
  {
    id: "edit-workflow",
    label: "Edit deployment workflow",
    short: "CI",
    operation: "edit_file",
    resourceType: "repo_path",
    resourceId: ".github/workflows/deploy.yml",
    risk: "high",
    forbidden: true,
    summary: "Deployment workflow edits are forbidden by the mission.",
  },
  {
    id: "open-pr",
    label: "Open pull request",
    short: "PR",
    operation: "open_pull_request",
    resourceType: "github_pull_request",
    resourceId: "tauliang/auth-scope",
    risk: "medium",
    summary: "Create a PR with a required authority check.",
  },
  {
    id: "deploy-preview",
    label: "Deploy preview environment",
    short: "DP",
    operation: "deploy_environment",
    resourceType: "deployment",
    resourceId: "preview/auth-scope",
    risk: "high",
    expansionKey: "preview_deploy",
    summary: "Deployment requires an explicit expansion or containment denies it.",
  },
];

const integrationScenarios = [
  {
    id: "github",
    label: "GitHub",
    short: "GH",
    category: "Source control",
    binding: "Repository binding: tauliang/auth-scope -> mref_coding_demo",
    contract: "POST /v1/integrations/github/check-runs/plan",
    surface: "Branch protection check",
    operation: "open_pull_request",
    resourceType: "github_pull_request",
    resourceId: "tauliang/auth-scope#248",
    outcome: "allow",
    summary: "Changed files are evaluated before the check run reports success.",
    facts: [
      "head_sha=abc123",
      "changed=frontend/src/features/missions/MissionsPage.tsx",
      "required_check=auth-scope/mission-authority",
    ],
  },
  {
    id: "okta",
    label: "Okta",
    short: "OK",
    category: "Identity",
    binding: "App binding: authscope.okta.com client coding-agent",
    contract: "POST /v1/integrations/okta/authority-context/resolve",
    surface: "OIDC claim resolver",
    operation: "resolve_claims",
    resourceType: "identity_context",
    resourceId: "okta:authscope.okta.com",
    outcome: "allow",
    summary: "Okta groups bind the human operator and agent session to the mission.",
    facts: [
      "sub=00u-codex-operator",
      "groups=mission-operators,coding-agents",
      "scope=openid profile mission.run",
    ],
  },
  {
    id: "entra",
    label: "Entra ID",
    short: "EN",
    category: "Identity",
    binding: "App registration: authscope.onmicrosoft.com agent-workbench",
    contract: "POST /v1/integrations/entra/authority-context/resolve",
    surface: "Microsoft identity claim resolver",
    operation: "resolve_claims",
    resourceType: "identity_context",
    resourceId: "entra:authscope.onmicrosoft.com",
    outcome: "allow",
    summary: "Tenant roles and app IDs are normalized into the same mission actor contract.",
    facts: [
      "tid=7f65-demo-tenant",
      "roles=AgentOperator,PullRequestAuthor",
      "appid=agent-workbench",
    ],
  },
  {
    id: "slack",
    label: "Slack",
    short: "SL",
    category: "Collaboration",
    binding: "Workspace binding: T024AUTH -> mref_coding_demo",
    contract: "POST /v1/integrations/slack/message-actions/authorize",
    surface: "Message action gate",
    operation: "post_message",
    resourceType: "slack_channel",
    resourceId: "C05MISSION",
    outcome: "allow",
    summary: "Agent updates are allowed only in the approved engineering channel.",
    facts: [
      "workspace_id=T024AUTH",
      "channel_id=C05MISSION",
      "user_email=alice@example.com",
    ],
  },
  {
    id: "jira",
    label: "Jira",
    short: "JI",
    category: "Work tracking",
    binding: "Atlassian site binding: MAS project",
    contract: "POST /v1/integrations/atlassian/jira/issues/authorize",
    surface: "Issue transition authorization",
    operation: "transition_issue",
    resourceType: "jira_project",
    resourceId: "MAS",
    outcome: "allow",
    summary: "Jira issue transitions are checked against the mission and project binding.",
    facts: [
      "site=https://authscope.atlassian.net",
      "project_key=MAS",
      "issue_key=MAS-184",
    ],
  },
  {
    id: "confluence",
    label: "Confluence",
    short: "CF",
    category: "Knowledge base",
    binding: "Atlassian site binding: ENG space",
    contract: "POST /v1/integrations/atlassian/confluence/pages/authorize",
    surface: "Page update authorization",
    operation: "update_page",
    resourceType: "confluence_space",
    resourceId: "ENG",
    outcome: "allow",
    summary: "Runbook updates are allowed in the engineering space, with evidence recorded.",
    facts: [
      "space_key=ENG",
      "page_id=712445",
      "label=mission-authority-demo",
    ],
  },
  {
    id: "servicenow",
    label: "ServiceNow",
    short: "SN",
    category: "ITSM",
    binding: "Change binding: CHG0030142 -> mref_coding_demo",
    contract: "ServiceNow.ResolveServiceNowAuthorityContext",
    surface: "Change-ticket authority context",
    operation: "add_work_note",
    resourceType: "servicenow_change",
    resourceId: "CHG0030142",
    outcome: "allow",
    summary: "Change-ticket state and assignment group become mission evaluation context.",
    facts: [
      "number=CHG0030142",
      "state=implement",
      "assignment_group=Platform Engineering",
    ],
  },
  {
    id: "salesforce",
    label: "Salesforce",
    short: "SF",
    category: "CRM",
    binding: "Org binding: authscope.my.salesforce.com",
    contract: "POST /v1/integrations/salesforce/records/authorize",
    surface: "Record action authorization",
    operation: "update_record",
    resourceType: "salesforce_record",
    resourceId: "Account:001Strategic",
    outcome: "deny",
    summary: "The agent can read stakeholder context but cannot alter strategic account data.",
    facts: [
      "object=Account",
      "record_id=001Strategic",
      "field=CustomerHealth__c",
    ],
  },
];

const state = {
  agentKey: "codex",
  mission: structuredCloneMission(initialMission),
  currentIndex: 0,
  selectedActionId: actionPlan[0].id,
  selectedIntegrationId: integrationScenarios[0].id,
  decisions: new Map(),
  integrationDecisions: new Map(),
  approvals: new Map(),
  audit: [],
};

const els = {
  agentId: document.getElementById("agent-id"),
  agentInstance: document.getElementById("agent-instance"),
  agentThumbprint: document.getElementById("agent-thumbprint"),
  missionVersion: document.getElementById("mission-version"),
  grantList: document.getElementById("grant-list"),
  actionList: document.getElementById("action-list"),
  requestJson: document.getElementById("request-json"),
  responseJson: document.getElementById("response-json"),
  decisionPill: document.getElementById("decision-pill"),
  runStatus: document.getElementById("run-status"),
  integrationList: document.getElementById("integration-list"),
  integrationTitle: document.getElementById("integration-title"),
  integrationCategory: document.getElementById("integration-category"),
  integrationSummary: document.getElementById("integration-summary"),
  integrationContract: document.getElementById("integration-contract"),
  integrationBinding: document.getElementById("integration-binding"),
  integrationSurface: document.getElementById("integration-surface"),
  integrationFacts: document.getElementById("integration-facts"),
  integrationRunStatus: document.getElementById("integration-run-status"),
  simulateIntegration: document.getElementById("simulate-integration"),
  approvalPanel: document.getElementById("approval-panel"),
  approvalCount: document.getElementById("approval-count"),
  githubCheck: document.getElementById("github-check"),
  integrationPreviewTitle: document.getElementById("integration-preview-title"),
  checkStatus: document.getElementById("check-status"),
  containmentStatus: document.getElementById("containment-status"),
  containAgent: document.getElementById("contain-agent"),
  blastMissions: document.getElementById("blast-missions"),
  blastLeases: document.getElementById("blast-leases"),
  blastProjections: document.getElementById("blast-projections"),
  auditList: document.getElementById("audit-list"),
};

document.querySelectorAll("[data-agent]").forEach((button) => {
  button.addEventListener("click", () => {
    state.agentKey = button.dataset.agent;
    document.querySelectorAll("[data-agent]").forEach((item) => item.classList.toggle("segment-active", item === button));
    addAudit("agent.profile_selected", `Presenter selected ${agentProfiles[state.agentKey].label}.`);
    render();
  });
});

document.getElementById("run-next").addEventListener("click", runNextAction);
document.getElementById("reset-run").addEventListener("click", resetRun);
els.simulateIntegration.addEventListener("click", simulateSelectedIntegration);
els.containAgent.addEventListener("click", toggleContainment);

addAudit("mission.created", "Mission authority issued for frontend bug fix.");
render();
selectAction(actionPlan[0].id);

function runNextAction() {
  if (state.currentIndex >= actionPlan.length) return;
  const action = actionPlan[state.currentIndex];
  state.selectedActionId = action.id;
  const decision = evaluateAction(action);
  state.decisions.set(action.id, decision);

  if (decision.response.decision === "require_approval") {
    state.approvals.set(action.expansionKey, {
      action,
      reason: "Out-of-scope coding action requires human approval.",
      requestedAuthority: authorityForAction(action),
    });
    addAudit("mission.expansion_requested", `${action.operation} on ${action.resourceId} needs approval.`);
  } else {
    addAudit(`decision.${decision.response.decision}`, `${action.operation} on ${action.resourceId}`);
    state.currentIndex += 1;
  }

  render();
  selectAction(action.id);
}

function simulateSelectedIntegration() {
  const integration = currentIntegration();
  const decision = evaluateIntegration(integration);
  state.integrationDecisions.set(integration.id, decision);
  addAudit(`integration.${integration.id}`, `${integration.label} returned ${decision.response.decision}.`);
  render();
  selectIntegration(integration.id);
}

function evaluateAction(action) {
  const actor = currentActor();
  const request = {
    mission_ref: state.mission.ref,
    mission_version_seen: state.mission.version,
    actor,
    action: {
      type: "coding_tool_call",
      name: action.operation,
      resource: { type: action.resourceType, id: action.resourceId },
      operation: action.operation,
    },
    context: {
      risk: action.risk,
      agent_profile: agentProfiles[state.agentKey].label,
      repository: "tauliang/auth-scope",
      branch: "agent/fix-dashboard-filter",
    },
  };

  const base = {
    mission_ref: state.mission.ref,
    mission_version: state.mission.version,
    decision_artifact: artifactFor(action),
  };

  if (state.mission.containment) {
    return {
      request,
      response: {
        ...base,
        decision: "deny",
        reason_codes: ["CONTAINMENT_ACTIVE"],
        human_reason: "Agent is contained. Tool calls fail closed until the rule is lifted.",
      },
    };
  }

  if (action.forbidden) {
    return {
      request,
      response: {
        ...base,
        decision: "deny",
        reason_codes: ["FORBIDDEN_ACTION", "OUTSIDE_MISSION_AUTHORITY"],
        human_reason: "The mission explicitly forbids deployment workflow edits.",
      },
    };
  }

  if (action.expansionKey && !state.mission.approvedExpansions.has(action.expansionKey)) {
    return {
      request,
      response: {
        ...base,
        decision: "require_approval",
        reason_codes: ["OUTSIDE_AUTHORITY_REGION", "HUMAN_APPROVAL_REQUIRED"],
        human_reason: "The action is plausible but outside the current mission boundary.",
        constraints: {
          expansion_request_id: `mex_${action.expansionKey}`,
          requested_authority: authorityForAction(action),
        },
        escalation: {
          type: "mission_expansion",
          url: `/v1/expansion-requests/mex_${action.expansionKey}`,
        },
      },
    };
  }

  return {
    request,
    response: {
      ...base,
      decision: "allow",
      reason_codes: ["IN_SCOPE", "AGENT_IDENTITY_BOUND"],
      human_reason: "Action is within the active mission authority region.",
      constraints: action.operation === "run_test" ? { lease_ttl_seconds: 120 } : {},
    },
  };
}

function evaluateIntegration(integration) {
  const actor = currentActor();
  const request = {
    contract: integration.contract,
    mission_ref: state.mission.ref,
    mission_version_seen: state.mission.version,
    binding: integration.binding,
    actor: integrationActor(integration, actor),
    evaluation: {
      actor,
      action: {
        type: "enterprise_integration",
        name: integration.operation,
        resource: { type: integration.resourceType, id: integration.resourceId },
        operation: integration.operation,
      },
      context: integrationContext(integration),
    },
  };

  const base = {
    integration: integration.id,
    mission_ref: state.mission.ref,
    mission_version: state.mission.version,
    decision_artifact: `demo-artifact.${state.mission.ref}.${state.mission.version}.integration.${integration.id}`,
  };

  if (state.mission.containment) {
    return {
      request,
      response: {
        ...base,
        decision: "deny",
        reason_codes: ["CONTAINMENT_ACTIVE"],
        human_reason: `${integration.label} is blocked because the agent is in active containment.`,
        integration_result: {
          surface: integration.surface,
          status: "blocked",
        },
      },
    };
  }

  if (integration.outcome === "deny") {
    return {
      request,
      response: {
        ...base,
        decision: "deny",
        reason_codes: ["INTEGRATION_BOUNDARY_VIOLATION", "OUTSIDE_MISSION_AUTHORITY"],
        human_reason: `${integration.label} request is outside the mission's approved enterprise boundary.`,
        integration_result: {
          surface: integration.surface,
          status: "blocked",
        },
      },
    };
  }

  if (integration.outcome === "require_approval") {
    return {
      request,
      response: {
        ...base,
        decision: "require_approval",
        reason_codes: ["INTEGRATION_APPROVAL_REQUIRED"],
        human_reason: `${integration.label} request is plausible but needs human approval before authority expands.`,
        constraints: {
          expansion_request_id: `mex_integration_${integration.id}`,
          requested_authority: authorityForIntegration(integration),
        },
        integration_result: {
          surface: integration.surface,
          status: "waiting_for_approval",
        },
      },
    };
  }

  return {
    request,
    response: {
      ...base,
      decision: "allow",
      reason_codes: ["INTEGRATION_BINDING_MATCHED", "MISSION_AUTHORITY_SATISFIED"],
      human_reason: `${integration.label} request matches an active integration binding and mission authority.`,
      integration_result: integrationResult(integration),
    },
  };
}

function approveExpansion(key) {
  const approval = state.approvals.get(key);
  if (!approval) return;
  state.mission.approvedExpansions.add(key);
  state.mission.version += 1;
  state.mission.grants.push(approval.requestedAuthority.resources[0]);
  state.approvals.delete(key);
  addAudit("mission.expansion_approved", `${approval.action.operation} approved by human reviewer.`);

  const decision = evaluateAction(approval.action);
  state.decisions.set(approval.action.id, decision);
  addAudit(`decision.${decision.response.decision}`, `${approval.action.operation} on ${approval.action.resourceId}`);
  state.currentIndex += 1;
  render();
  selectAction(approval.action.id);
}

function toggleContainment() {
  state.mission.containment = !state.mission.containment;
  addAudit(
    state.mission.containment ? "containment.created" : "containment.lifted",
    state.mission.containment ? "Agent placed in fail-closed containment." : "Containment lifted by operator.",
  );
  render();
}

function resetRun() {
  state.mission = structuredCloneMission(initialMission);
  state.currentIndex = 0;
  state.selectedActionId = actionPlan[0].id;
  state.decisions.clear();
  state.integrationDecisions.clear();
  state.approvals.clear();
  state.audit = [];
  addAudit("mission.created", "Mission authority issued for frontend bug fix.");
  render();
  selectAction(actionPlan[0].id);
}

function render() {
  const profile = agentProfiles[state.agentKey];
  els.agentId.textContent = profile.agentId;
  els.agentInstance.textContent = profile.instance;
  els.agentThumbprint.textContent = profile.thumbprint;
  els.missionVersion.textContent = `v${state.mission.version}`;

  els.grantList.innerHTML = state.mission.grants.map((grant) => `
    <div class="grant">
      <strong>${escapeHtml(grant.id)}</strong>
      <span>${escapeHtml(grant.type)}: ${grant.actions.map(escapeHtml).join(", ")}</span>
    </div>
  `).join("");

  els.actionList.innerHTML = actionPlan.map((action, index) => {
    const decision = state.decisions.get(action.id);
    const stateLabel = decision ? decision.response.decision : index === state.currentIndex ? "next" : index < state.currentIndex ? "done" : "queued";
    const tone = decisionTone(stateLabel);
    return `
      <button class="action-row ${action.id === state.selectedActionId ? "action-row-active" : ""}" type="button" data-action-id="${action.id}">
        <div class="action-icon">${action.short}</div>
        <div>
          <strong>${escapeHtml(action.label)}</strong>
          <span>${escapeHtml(action.summary)}</span>
          <span class="mono">${escapeHtml(action.operation)} -> ${escapeHtml(action.resourceId)}</span>
        </div>
        <span class="status ${tone}">${escapeHtml(stateLabel)}</span>
      </button>
    `;
  }).join("");

  document.querySelectorAll("[data-action-id]").forEach((button) => {
    button.addEventListener("click", () => selectAction(button.dataset.actionId));
  });

  renderIntegrations();
  renderApprovals();
  renderIntegrationPreview();
  renderContainment();
  renderAudit();
  els.runStatus.textContent = state.currentIndex >= actionPlan.length ? "complete" : `step ${state.currentIndex + 1} of ${actionPlan.length}`;
  els.integrationRunStatus.textContent = `${state.integrationDecisions.size} simulated`;
}

function renderIntegrations() {
  els.integrationList.innerHTML = integrationScenarios.map((integration) => {
    const decision = state.integrationDecisions.get(integration.id);
    const stateLabel = decision ? decision.response.decision : "ready";
    const tone = decisionTone(stateLabel);
    return `
      <button class="integration-row ${integration.id === state.selectedIntegrationId ? "integration-row-active" : ""}" type="button" data-integration-id="${integration.id}">
        <div class="integration-badge">${integration.short}</div>
        <div>
          <strong>${escapeHtml(integration.label)}</strong>
          <span>${escapeHtml(integration.category)}</span>
        </div>
        <span class="status ${tone}">${escapeHtml(stateLabel)}</span>
      </button>
    `;
  }).join("");

  document.querySelectorAll("[data-integration-id]").forEach((button) => {
    button.addEventListener("click", () => selectIntegration(button.dataset.integrationId));
  });

  renderIntegrationDetail(currentIntegration());
}

function renderIntegrationDetail(integration) {
  els.integrationCategory.textContent = integration.short;
  els.integrationTitle.textContent = integration.label;
  els.integrationSummary.textContent = integration.summary;
  els.integrationContract.textContent = integration.contract;
  els.integrationBinding.textContent = integration.binding;
  els.integrationSurface.textContent = integration.surface;
  els.integrationFacts.innerHTML = integration.facts.map((fact) => `
    <span>${escapeHtml(fact)}</span>
  `).join("");
}

function renderApprovals() {
  const approvals = Array.from(state.approvals.values());
  els.approvalCount.textContent = String(approvals.length);
  if (!approvals.length) {
    els.approvalPanel.className = "empty-panel";
    els.approvalPanel.innerHTML = "No pending approvals.";
    return;
  }
  els.approvalPanel.className = "";
  els.approvalPanel.innerHTML = approvals.map((approval) => `
    <div class="approval-item">
      <div>
        <strong>${escapeHtml(approval.action.label)}</strong>
        <span>${escapeHtml(approval.reason)}</span>
      </div>
      <button class="button button-primary" type="button" data-approve="${approval.action.expansionKey}">Approve expansion</button>
    </div>
  `).join("");
  document.querySelectorAll("[data-approve]").forEach((button) => {
    button.addEventListener("click", () => approveExpansion(button.dataset.approve));
  });
}

function renderIntegrationPreview() {
  const integration = currentIntegration();
  const decision = state.integrationDecisions.get(integration.id);
  let status = "neutral";
  let title = `${integration.label} waiting`;
  let body = "Select and simulate an integration to preview the external control signal.";
  let meta = integration.contract;

  if (state.mission.containment) {
    status = "failure";
    title = "Auth Scope blocked this run";
    body = "Containment is active. Integrated systems should fail closed.";
  } else if (decision) {
    status = integrationStatus(decision.response.decision);
    title = `${integration.label}: ${decision.response.decision}`;
    body = decision.response.human_reason;
  } else if (state.approvals.size > 0) {
    status = "action_required";
    title = "Human approval required";
    body = "At least one proposed coding action exceeds the mission.";
  } else if (state.currentIndex >= actionPlan.length) {
    status = "success";
    title = "Mission authority satisfied";
    body = "Allowed and denied actions are explained by policy evidence.";
  } else if (state.currentIndex > 0) {
    status = "in_progress";
    title = "Authority check in progress";
    body = "The coding agent is still asking before each tool call.";
  }

  els.checkStatus.textContent = status;
  els.checkStatus.className = `status ${status === "failure" ? "status-red" : status === "action_required" ? "status-amber" : status === "success" ? "status-green" : ""}`;
  els.integrationPreviewTitle.textContent = integration.surface;
  els.githubCheck.innerHTML = `
    <strong>${escapeHtml(title)}</strong>
    <span>${escapeHtml(body)}</span>
    <span class="mono">${escapeHtml(meta)}</span>
  `;
}

function renderContainment() {
  els.containmentStatus.textContent = state.mission.containment ? "active" : "inactive";
  els.containmentStatus.className = `status ${state.mission.containment ? "status-red" : "status-green"}`;
  els.containAgent.textContent = state.mission.containment ? "Lift containment" : "Contain agent";
  els.containAgent.className = `button ${state.mission.containment ? "button-secondary" : "button-secondary"}`;
  els.blastMissions.textContent = state.mission.containment ? "1" : "0";
  els.blastLeases.textContent = state.mission.containment ? "1" : "0";
  els.blastProjections.textContent = state.mission.containment ? "1" : "0";
}

function renderAudit() {
  els.auditList.innerHTML = state.audit.slice().reverse().map((event) => `
    <div class="audit-event">
      <strong>${escapeHtml(event.type)}</strong>
      <span>${escapeHtml(event.detail)}</span>
      <span class="mono">${escapeHtml(event.time)}</span>
    </div>
  `).join("");
}

function selectAction(actionId) {
  state.selectedActionId = actionId;
  const action = actionPlan.find((item) => item.id === actionId);
  const decision = state.decisions.get(actionId) || evaluateAction(action);
  els.requestJson.textContent = JSON.stringify(decision.request, null, 2);
  els.responseJson.textContent = JSON.stringify(decision.response, null, 2);
  els.decisionPill.textContent = decision.response.decision;
  els.decisionPill.className = `decision-pill decision-${decision.response.decision}`;
  renderActionSelection();
}

function selectIntegration(integrationId) {
  state.selectedIntegrationId = integrationId;
  const integration = currentIntegration();
  const decision = state.integrationDecisions.get(integrationId) || evaluateIntegration(integration);
  els.requestJson.textContent = JSON.stringify(decision.request, null, 2);
  els.responseJson.textContent = JSON.stringify(decision.response, null, 2);
  els.decisionPill.textContent = decision.response.decision;
  els.decisionPill.className = `decision-pill decision-${decision.response.decision}`;
  renderIntegrationDetail(integration);
  renderIntegrationSelection();
  renderIntegrationPreview();
}

function renderActionSelection() {
  document.querySelectorAll("[data-action-id]").forEach((button) => {
    button.classList.toggle("action-row-active", button.dataset.actionId === state.selectedActionId);
  });
}

function renderIntegrationSelection() {
  document.querySelectorAll("[data-integration-id]").forEach((button) => {
    button.classList.toggle("integration-row-active", button.dataset.integrationId === state.selectedIntegrationId);
  });
}

function currentActor() {
  const profile = agentProfiles[state.agentKey];
  return {
    agent_instance_id: profile.instance,
    client_id: profile.clientId,
    key_thumbprint: profile.thumbprint,
  };
}

function authorityForAction(action) {
  return {
    resources: [
      {
        type: action.resourceType,
        id: action.resourceId,
        actions: [action.operation],
      },
    ],
  };
}

function authorityForIntegration(integration) {
  return {
    resources: [
      {
        type: integration.resourceType,
        id: integration.resourceId,
        actions: [integration.operation],
      },
    ],
  };
}

function currentIntegration() {
  return integrationScenarios.find((integration) => integration.id === state.selectedIntegrationId) || integrationScenarios[0];
}

function integrationActor(integration, actor) {
  const base = {
    agent_instance_id: actor.agent_instance_id,
    client_id: actor.client_id,
    key_thumbprint: actor.key_thumbprint,
  };
  if (integration.id === "okta") return { ...base, issuer: "https://authscope.okta.com/oauth2/default", subject: "00u-codex-operator" };
  if (integration.id === "entra") return { ...base, issuer: "https://login.microsoftonline.com/7f65-demo-tenant/v2.0", subject: "alice@example.com" };
  if (integration.id === "slack") return { ...base, workspace_id: "T024AUTH", user_id: "U024ALICE" };
  if (integration.id === "jira" || integration.id === "confluence") return { ...base, account_id: "712020:alice" };
  if (integration.id === "salesforce") return { ...base, username: "alice@example.com", org_id: "00Ddemo" };
  if (integration.id === "servicenow") return { ...base, user_name: "alice@example.com" };
  return { ...base, login: "alice-authscope" };
}

function integrationContext(integration) {
  return {
    risk: integration.outcome === "deny" ? "high" : "low",
    integration: integration.id,
    surface: integration.surface,
    facts: integration.facts,
  };
}

function integrationResult(integration) {
  if (integration.id === "github") {
    return { surface: integration.surface, conclusion: "success", check_run: "auth-scope/mission-authority" };
  }
  if (integration.id === "okta" || integration.id === "entra") {
    return { surface: integration.surface, subject_bound: true, tenant_checked: true };
  }
  return { surface: integration.surface, status: "authorized" };
}

function integrationStatus(decision) {
  if (decision === "allow") return "success";
  if (decision === "deny") return "failure";
  if (decision === "require_approval" || decision === "require_expansion") return "action_required";
  return "neutral";
}

function artifactFor(action) {
  return `demo-artifact.${state.mission.ref}.${state.mission.version}.${action.id}`;
}

function addAudit(type, detail) {
  state.audit.push({
    type,
    detail,
    time: new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }),
  });
}

function decisionTone(label) {
  if (label === "allow" || label === "done") return "status-green";
  if (label === "require_approval" || label === "require_expansion" || label === "next") return "status-amber";
  if (label === "deny") return "status-red";
  return "";
}

function structuredCloneMission(mission) {
  return {
    ...mission,
    grants: mission.grants.map((grant) => ({
      ...grant,
      actions: [...grant.actions],
      constraints: { ...grant.constraints },
    })),
    approvedExpansions: new Set(),
  };
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
