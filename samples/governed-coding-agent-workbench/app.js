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

const state = {
  agentKey: "codex",
  mission: structuredCloneMission(initialMission),
  currentIndex: 0,
  selectedActionId: actionPlan[0].id,
  decisions: new Map(),
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
  approvalPanel: document.getElementById("approval-panel"),
  approvalCount: document.getElementById("approval-count"),
  githubCheck: document.getElementById("github-check"),
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

  renderApprovals();
  renderGithubCheck();
  renderContainment();
  renderAudit();
  els.runStatus.textContent = state.currentIndex >= actionPlan.length ? "complete" : `step ${state.currentIndex + 1} of ${actionPlan.length}`;
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

function renderGithubCheck() {
  let status = "neutral";
  let title = "Authority check waiting";
  let body = "Run agent actions to produce a GitHub status check.";

  if (state.mission.containment) {
    status = "failure";
    title = "Auth Scope blocked this run";
    body = "Containment is active. Branch protection should block merge.";
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
  els.githubCheck.innerHTML = `
    <strong>${escapeHtml(title)}</strong>
    <span>${escapeHtml(body)}</span>
    <span class="mono">check_run.name = auth-scope/mission-authority</span>
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

function renderActionSelection() {
  document.querySelectorAll("[data-action-id]").forEach((button) => {
    button.classList.toggle("action-row-active", button.dataset.actionId === state.selectedActionId);
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
