# Governed Coding Agent Workbench

This sample app shows how Auth Scope can govern a coding agent such as Codex or OpenCode and coordinate that authority across mission-critical enterprise services.

The app is intentionally static and dependency-free. It simulates the policy enforcement point that would sit between a coding agent and tools such as file edit, shell/test execution, package installation, pull request creation, deployment, and external SaaS integration actions.

## Run

Open the HTML file directly:

```bash
open samples/governed-coding-agent-workbench/index.html
```

No build step or package install is required.

## Demo Flow

1. Choose `Codex` or `OpenCode` as the agent profile.
2. Click `Run next action` to evaluate each proposed coding action.
3. Watch low-risk actions receive `allow`.
4. When package installation requires approval, click `Approve expansion`.
5. Select services in `Mission-critical integrations` and click `Simulate integration`.
6. Show how GitHub, Okta, Entra ID, Slack, Jira, Confluence, ServiceNow, and Salesforce are normalized into the same mission evaluation contract.
7. Continue the run and inspect the integration preview.
8. Click `Contain agent` to show fail-closed behavior and blast radius.
9. Use the request/response panels to explain how the sample maps coding actions and integration events to Auth Scope mission authority.

## What It Demonstrates

- Mission-scoped authority for coding agents.
- Signed workload identity as the runtime actor.
- Per-action evaluation before tool execution.
- Human approval for out-of-scope work.
- Versioned authority after an approval.
- GitHub-style status checks as an integration point.
- Okta and Entra ID claim resolution into mission actor context.
- Slack message authorization for approved collaboration channels.
- Atlassian Jira issue and Confluence page authorization.
- ServiceNow change-ticket context as authority evidence.
- Salesforce record action enforcement with a fail-closed denial example.
- Emergency containment with affected missions, leases, and projections.
- Audit evidence for each decision.

## Mapping To The MVP Service

This sample is a front-end simulation. In a production integration, the workbench's policy calls would map to:

- `POST /v1/missions/{mission_ref}/evaluate`
- `POST /v1/expansion-requests/{expansion_id}/approvals`
- `POST /v1/tool-calls/authorize`
- `POST /v1/containment-rules`
- `GET /v1/missions/{mission_ref}/lineage`
- `GET /v1/events`
- `POST /v1/integrations/github/check-runs/plan`
- `POST /v1/integrations/okta/authority-context/resolve`
- `POST /v1/integrations/entra/authority-context/resolve`
- `POST /v1/integrations/slack/message-actions/authorize`
- `POST /v1/integrations/atlassian/jira/issues/authorize`
- `POST /v1/integrations/atlassian/confluence/pages/authorize`
- `POST /v1/integrations/salesforce/records/authorize`

The ServiceNow scenario mirrors the service-layer `ResolveServiceNowAuthorityContext` contract so stakeholders can see how ITSM change context participates in the same decision model.
