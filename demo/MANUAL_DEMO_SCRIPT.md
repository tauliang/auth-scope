# Manual Stakeholder Demo Script

This runbook is for a presenter who wants to drive the Auth Scope Mission Authority Service manually in front of stakeholders. It uses the committed demo scenario data and the local Docker Compose stack.

## What This Demo Shows

By the end of the walkthrough, stakeholders should see that Auth Scope can:

1. Bind an AI agent to a cryptographic workload identity.
2. Represent the agent's authority as a mission, not as a broad token.
3. Require human approval when the agent asks to exceed its mission.
4. Commit the approval as versioned authority.
5. Project scoped authority into gateway credentials.
6. Contain an agent during an incident and inspect blast radius.
7. Verify signed decision evidence and audit the whole flow.

## Presenter Setup

Use a clean terminal at the repository root.

```bash
cd auth-scope
git switch test/demo
```

Start the local stack:

```bash
docker compose up --build
```

Wait until both services are healthy:

```bash
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:3000/healthz
```

Optional clean slate before a high-stakes demo:

```bash
docker compose down -v
docker compose up --build
```

`docker compose down -v` deletes the local demo PostgreSQL volume. Use it only when you want to remove prior demo runs.

## Seed The Demo Data

In a second terminal, seed a fresh mission, agent identity, projection, lease, approval rule, tool contract, and pending expansion:

```bash
node demo/seed-demo.mjs
```

The command prints the generated mission, agent, and pending expansion identifiers. It also writes a local state file:

```text
demo/.generated/mission-authority-state.json
```

Print the presenter values in a compact format:

```bash
node -e "const s=require('./demo/.generated/mission-authority-state.json'); console.log(JSON.stringify({run_id:s.run_id, mission_ref:s.mission.mission_ref, mission_objective:s.mission.objective, agent_id:s.agent.agent_id, agent_client_id:s.agent.client_id, agent_instance_id:s.agent.instance_id, key_thumbprint:s.agent.key_thumbprint, expansion_id:s.expansion.expansion_id, expansion_resource:s.expansion.action.resource.id, projection_id:s.projection.projection_id}, null, 2))"
```

For the Workbench step, print copy/paste values separately:

```bash
node -e "const s=require('./demo/.generated/mission-authority-state.json'); console.log(s.decisions.expansion_required.decision_artifact)"
node -e "const s=require('./demo/.generated/mission-authority-state.json'); console.log(s.projection.token)"
```

Keep this terminal open. You will copy values from it during the demo.

## Values To Use

Use these fixed values unless the step says to copy a generated value from the state file.

| UI field | Value |
| --- | --- |
| Console URL | `http://localhost:3000` |
| Bearer token | `dev-compose-admin-alice` |
| Approval decision reason | `Bob reviewed risk; Alice approves bounded Slack posting for the stakeholder demo.` |
| Containment tenant | `demo` |
| Containment target type | `Agent` |
| Containment target ID | Copy `.agent.instance_id` from `demo/.generated/mission-authority-state.json` |
| Containment reason | `Demo incident: hold this agent while an anomalous update is investigated.` |
| Containment expires at | Leave blank |
| Mission search | Copy `.mission.mission_ref` |
| Agent search | Copy `.agent.agent_id` or `.agent.client_id` |
| Projection search | Copy `.projection.projection_id` |
| Decision artifact textarea | Copy `.decisions.expansion_required.decision_artifact` |
| Projection token textarea | Copy `.projection.token` |

## Live Demo Flow

### 1. Open The Console

Open:

```text
http://localhost:3000
```

On the `Administrator access` screen:

1. Paste `dev-compose-admin-alice` into `Bearer token`.
2. Click `Open console`.

Talk track:

> We start with a human operator boundary. The console does not persist the bearer credential. It keeps it in browser memory, so a refresh clears access.

Expected result:

- The `Authority overview` page appears.
- The sidebar shows Alice as the operator.
- Summary cards show active missions, pending approvals, registered agents, live projections, and containment.

### 2. Show The Approval Queue

Click `Approvals` in the left navigation.

Find the expansion row whose title is:

```text
post_update on {expansion_resource}
```

The generated `{expansion_resource}` is printed by the seed script as `.expansion.action.resource.id`, usually like:

```text
board-updates-20260719013959-381e0c37
```

Click that row.

Talk track:

> The agent had authority to read and draft in a board packet folder. Posting to Slack is outside the original authority boundary, so the service created an expansion request instead of silently allowing the action.

Expected result:

- The `Expansion review` page opens.
- The left side shows current effective authority.
- The right side shows the requested Slack authority.

### 3. Approve The Expansion

In `Decision reason`, enter:

```text
Bob reviewed risk; Alice approves bounded Slack posting for the stakeholder demo.
```

Click `Approve`.

Talk track:

> This approval is not just a UI action. It is committed as mission authority. The mission version changes, and the approval evidence becomes part of the audit and lineage story.

Expected result:

- The UI returns to `Approval queue`.
- The current expansion is no longer pending.

### 4. Inspect Effective Mission Authority

Click `Missions`.

In `Search mission, objective, principal, or agent`, paste the generated mission ref:

```text
{mission_ref}
```

Click the arrow button for the mission row.

On the mission detail page:

1. Click the `Authority` tab.
2. Confirm the original Drive grant is present.
3. Confirm the approved Slack grant is present.

Expected authority:

- `drive_folder` resource with `read` and `write_draft`.
- `slack_channel` resource with `post_update`.
- Forbidden actions remain visible, including `send_external` and `delete_source`.

Talk track:

> The mission is the authority object. After approval, the mission version now includes the new Slack grant while preserving the forbidden actions.

If the Slack grant does not appear immediately, refresh the page, log in again with `dev-compose-admin-alice`, and reopen the mission by mission ref.

### 5. Show Mission Lineage

On the mission detail page, click the `Lineage` tab.

Talk track:

> Lineage is how the operator can answer where authority came from and what changed it. This graph ties mission, agent, approvals, projection, lease, and governance events together.

Expected result:

- A lineage graph appears.
- The accessible lineage list is available below the graph.

### 6. Inspect The Agent Identity

Click `Agents`.

Search for either:

```text
{agent_id}
```

or:

```text
{agent_client_id}
```

Open the agent row.

Talk track:

> Runtime requests are not anonymous. The agent is registered with an Ed25519 public key, and signed requests bind actions back to this workload identity.

Expected result:

- Agent status is `active`.
- Provider is `https://agents.example.com`.
- Instance ID matches `.agent.instance_id`.
- Thumbprint matches `.agent.key_thumbprint`.

### 7. Inspect Projected Authority

Click `Projections`.

Search for the generated projection ID:

```text
{projection_id}
```

Talk track:

> Some tools and gateways cannot call the mission service on every operation. Auth Scope can issue short-lived projection tokens that carry only scoped mission authority.

Expected result:

- A projection row appears.
- Type is `Mcp Context`.
- Status is `active` until containment is created.
- Actor is the generated demo agent.

### 8. Create A Containment Rule

Click `Containment`.

Click `New containment`.

Fill the form:

| Field | Value |
| --- | --- |
| Tenant | `demo` |
| Target type | `Agent` |
| Target ID | Copy `.agent.instance_id` |
| Expires at | Leave blank |
| Reason | `Demo incident: hold this agent while an anomalous update is investigated.` |

Click `Activate containment`.

After the rule appears in the list, click the row whose title is:

```text
agent: {agent_instance_id}
```

Talk track:

> Containment is the emergency brake. Instead of hunting through individual tokens or tools, the operator can fail closed for an agent, mission, principal, tool, resource, or tenant and inspect impact.

Expected result:

- The containment detail page shows `active`.
- `Blast radius` counts affected missions, projections, leases, and related records.
- The affected mission appears under `Affected missions`.

### 9. Verify Signed Decision Evidence

Click `Workbench`.

In `Signed decision artifact`, paste the value printed by:

```bash
node -e "const s=require('./demo/.generated/mission-authority-state.json'); console.log(s.decisions.expansion_required.decision_artifact)"
```

Click `Verify artifact`.

Talk track:

> The service can verify its own signed decision artifacts. This artifact proves that the original Slack request required human approval and records the evidence ID.

Expected result:

- `Verification complete` appears.
- The JSON result includes `"valid": true`.
- The payload includes `"decision": "require_approval"`.

### 10. Verify Projection Fail-Closed Behavior

Still on `Workbench`, in `Projection token`, paste the value printed by:

```bash
node -e "const s=require('./demo/.generated/mission-authority-state.json'); console.log(s.projection.token)"
```

Click `Verify projection`.

Talk track:

> This projection was valid before containment. After containment, verification fails closed because the projection is blocked by the active containment rule.

Expected result:

- `Verification complete` appears.
- The JSON result includes `"valid": false`.
- The error mentions `projection blocked by containment rule`.

### 11. Show The Audit Trail

Click `Audit`.

In `Search event, mission, causation, or correlation`, paste:

```text
{mission_ref}
```

Click a row such as `Mission Expansion Approved` or `Mission Projection Created`.

Talk track:

> The audit trail is the durable story. It shows the proposal, approval, expansion, projection, lease, containment, and the actors that caused each event.

Expected result:

- Events are filtered to the generated mission.
- Selecting an event shows actor, payload, and version transition data.

## Optional: Run The Automated Demo

If you want a deterministic browser run instead of manual clicking:

```bash
node demo/run-demo.mjs
```

For an attended browser session:

```bash
node demo/run-demo.mjs --headed
```

The automated script performs the same core story and validates the UI flow with Playwright.

## Troubleshooting

### The approval queue has many pending items

Previous demo runs may still be in the local PostgreSQL volume. Use a clean slate:

```bash
docker compose down -v
docker compose up --build
node demo/seed-demo.mjs
```

### Login fails

Use the Docker Compose development token:

```text
dev-compose-admin-alice
```

If you are not using Docker Compose, check the API environment variables for `AUTH_SCOPE_ADMIN_TOKEN` or `AUTH_SCOPE_ADMIN_CREDENTIALS`.

### The seed command cannot reach the API

Confirm the API is healthy:

```bash
curl -fsS http://127.0.0.1:8080/healthz
```

If the API is on another port:

```bash
AUTH_SCOPE_API_URL=http://127.0.0.1:9090 node demo/seed-demo.mjs
```

### The Workbench values are hard to copy

Print them one at a time:

```bash
node -e "const s=require('./demo/.generated/mission-authority-state.json'); console.log(s.decisions.expansion_required.decision_artifact)"
node -e "const s=require('./demo/.generated/mission-authority-state.json'); console.log(s.projection.token)"
```

On macOS, copy directly to the clipboard:

```bash
node -e "const s=require('./demo/.generated/mission-authority-state.json'); process.stdout.write(s.decisions.expansion_required.decision_artifact)" | pbcopy
node -e "const s=require('./demo/.generated/mission-authority-state.json'); process.stdout.write(s.projection.token)" | pbcopy
```

### The console asks for the token again after refresh

That is expected. The console keeps the administrator token in browser memory only. Paste `dev-compose-admin-alice` again and continue.

## Reset After The Demo

Stop the stack:

```bash
docker compose down
```

Delete local demo data if needed:

```bash
docker compose down -v
```
