# Governed Coding Agent Workbench

This sample app shows how Auth Scope can govern a coding agent such as Codex or OpenCode.

The app is intentionally static and dependency-free. It simulates the policy enforcement point that would sit between a coding agent and tools such as file edit, shell/test execution, package installation, pull request creation, and deployment.

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
5. Continue the run and inspect the GitHub check preview.
6. Click `Contain agent` to show fail-closed behavior and blast radius.
7. Use the request/response panels to explain how the sample maps coding actions to Auth Scope mission authority.

## What It Demonstrates

- Mission-scoped authority for coding agents.
- Signed workload identity as the runtime actor.
- Per-action evaluation before tool execution.
- Human approval for out-of-scope work.
- Versioned authority after an approval.
- GitHub-style status checks as an integration point.
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
