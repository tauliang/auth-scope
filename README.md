# auth-scope

`auth-scope` is an MVP Mission Authority Service for AI agents. It models mission-scoped delegated authority as a first-class object that can be approved, evaluated, delegated, completed, revoked, and audited independently of token validity.

## Features

- **Mission authority lifecycle:** proposals, approvals, synchronous evaluation, risky-action expansion requests, safe subset negotiation, resume checks, strict-subset delegation, completion, revocation, and cascade semantics.
- **Agent identity and runtime enforcement:** agent registry, Ed25519 request signatures, nonce replay protection, AuthZEN-compatible authorization, MCP-style tool contracts, brokered OAuth/MCP/tool credentials, signed external projections, short-lived mission leases, and fail-closed production startup checks.
- **Governance controls:** versioned policy-as-code bundles with dry-run simulation, multi-approver expansion rules, containment rules, blast-radius inspection, tenant-scoped administrator credentials, and runtime blocks for evaluation, delegation, expansion, projections, leases, and resume.
- **Evidence and audit:** signed decision artifacts, stored policy evidence with applied policy bundle/rule IDs, immutable event history, Server-Sent Events streaming, transactional outbox publishing, and mission/agent lineage graphs.
- **Identity and workflow integrations:** GitHub repository and check-run hooks for coding-agent PR governance, Okta application/group authority resolution, Microsoft Entra app registration/group/role authority resolution, Slack workspace/message-action authorization, Atlassian Jira/Confluence action authorization, and Salesforce CRM record-action authorization.
- **Operator experience:** Go HTTP API, in-memory and PostgreSQL-backed stores, embedded PostgreSQL migrations, React operator console, same-origin Docker Compose deployment with nginx `/api` proxy, OpenAPI contract, demo scripts, and a governed coding-agent workbench sample.
- **Discovery and interoperability:** well-known Mission Authority and AuthZEN discovery documents plus generated frontend TypeScript declarations from `openapi/auth-scope-v1.yaml`.

## Installation Instructions, Supported Platforms, And Testing

### Installation Instructions

The fastest installation path is Docker Compose, which starts PostgreSQL, the Go API, and the React operator console with one command:

```sh
docker compose up --build
```

After the stack starts, open `http://localhost:3000` and sign in with the development token `dev-compose-admin-alice`. The API is available at `http://localhost:8080`.

For local development without Docker, install:

- Go, using the version declared in [`go.mod`](go.mod).
- Node.js and pnpm, using the package manager version declared in [`frontend/package.json`](frontend/package.json).
- PostgreSQL, only when testing the persistent store; otherwise the API can run with the in-memory store.

Then run the API and frontend in separate terminals:

```sh
go run ./cmd/auth-scope
```

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

### Supported Platforms

Auth Scope is developed for containerized and local development on:

- macOS with Docker Desktop or local Go/Node tooling.
- Linux with Docker Engine/Compose or local Go/Node tooling.
- Windows through WSL2 or Docker Desktop.

The backend is a Go service and should run anywhere Go supports the target architecture. The operator console is a browser-based React application tested against Chromium through Playwright. Production deployments should use PostgreSQL and explicit administrator credentials instead of the development in-memory store and demo tokens.

### Instructions For Testing

Run the backend test suite:

```sh
go test ./...
```

Run backend coverage:

```sh
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Run frontend validation:

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm test:coverage
pnpm build
pnpm e2e
```

Validate the Docker Compose configuration:

```sh
docker compose config --quiet
```

Before opening or updating a PR, run the checks relevant to the files you changed. Frontend coverage enforces an 80% minimum for statements, branches, functions, and lines.

## Quick Start

### One-command stack

Start PostgreSQL, the API, and the operator console:

```sh
docker compose up --build
```

Open the operator console at `http://localhost:3000` and authenticate with the development token `dev-compose-admin-alice`. The API remains available at `http://localhost:8080`. The service applies embedded migrations automatically when `DATABASE_URL` is set.

Override any host port when needed:

```sh
AUTH_SCOPE_FRONTEND_PORT=3100 AUTH_SCOPE_PORT=9090 AUTH_SCOPE_POSTGRES_PORT=15432 docker compose up --build
```

### First API call

Check the API and discovery document once the stack is running:

```sh
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/.well-known/mission-authority
```

For governance endpoints, set the local administrator token used by the Docker Compose stack:

```sh
ADMIN_TOKEN=dev-compose-admin-alice
```

The longer examples below use this variable.

## Run Locally

### Local API process

Run the API locally without Docker:

```sh
go run ./cmd/auth-scope
```

The server listens on `:8080` by default and uses the in-memory store unless `DATABASE_URL` is set. Override the address with `AUTH_SCOPE_ADDR`. Decision artifacts and projection tokens are signed with `AUTH_SCOPE_DECISION_SECRET`; a development-only default is used when it is not set. GitHub webhooks are verified with `AUTH_SCOPE_GITHUB_WEBHOOK_SECRET` or `GITHUB_WEBHOOK_SECRET` when the GitHub integration endpoints are enabled. Okta and Microsoft Entra bindings resolve already-verified OIDC claims into mission authority context, Slack bindings resolve already-verified Slack user/workspace facts into message-action authorization context, Atlassian bindings resolve already-verified Jira/Confluence site, account, group, project, and space facts into mission authorization context, and Salesforce bindings resolve already-verified org, user, profile, permission-set, object, and record facts into mission authorization context. These integrations do not require a live provider network dependency in the runtime hot path.

Set `AUTH_SCOPE_MODE=production` or `AUTH_SCOPE_ENV=production` for fail-closed startup checks. Production mode requires `DATABASE_URL`, explicit administrator credentials, and a non-placeholder `AUTH_SCOPE_DECISION_SECRET` of at least 32 characters. The production binary also requires signed agent requests on runtime authority endpoints such as mission evaluation, AuthZEN evaluation, delegation, projections, leases, and tool-call authorization.

Governance and audit endpoints require a bearer token bound to an administrator principal. Configure one administrator with `AUTH_SCOPE_ADMIN_TOKEN`, `AUTH_SCOPE_ADMIN_SUBJECT`, and `AUTH_SCOPE_ADMIN_ISSUER`, or configure multiple independent approvers with `AUTH_SCOPE_ADMIN_CREDENTIALS`. Static credentials can include `tenant_subject`, `roles`, `groups`, and `permissions`; omitted role metadata keeps backward-compatible owner access for local development:

```sh
AUTH_SCOPE_ADMIN_CREDENTIALS='[{"token":"alice-secret","subject":"alice@example.com","issuer":"https://idp.example.com","roles":["approver"]},{"token":"ops-secret","subject":"ops@example.com","issuer":"https://idp.example.com","tenant_subject":"demo","roles":["operator"]}]' go run ./cmd/auth-scope
```

For enterprise SSO, configure OIDC/JWKS admin authentication instead of static tokens:

```sh
AUTH_SCOPE_ADMIN_OIDC_ISSUER=https://idp.example.com \
AUTH_SCOPE_ADMIN_OIDC_AUDIENCE=auth-scope-admin \
AUTH_SCOPE_ADMIN_OIDC_JWKS='{"keys":[...]}' \
AUTH_SCOPE_ADMIN_GROUP_ROLE_MAPPINGS='{"AuthScope Approvers":["approver"],"AuthScope Auditors":["auditor"],"AuthScope Operators":["operator"],"AuthScope Integration Admins":["integration-admin"]}' \
go run ./cmd/auth-scope
```

OIDC admin tokens must be RS256-signed, include `iss`, `aud`, `sub`, and `exp`, and are verified against the configured JWKS without calling the identity provider at request time. Optional claims default to `groups`, `roles`, `permissions`, and `tenant_subject`; override them with `AUTH_SCOPE_ADMIN_OIDC_GROUPS_CLAIM`, `AUTH_SCOPE_ADMIN_OIDC_ROLES_CLAIM`, `AUTH_SCOPE_ADMIN_OIDC_PERMISSIONS_CLAIM`, and `AUTH_SCOPE_ADMIN_OIDC_TENANT_CLAIM`. Built-in roles are `owner`, `approver`, `auditor`, `operator`, and `integration-admin`; protected admin writes emit `admin.action` audit events with actor, role, permission, path, request ID, and status code.

The request body cannot select its approver, containment administrator, or tenant when the administrator credential is tenant-bound. The service derives those identities from the bearer credential. Docker Compose includes development-only Alice and Bob credentials; use `dev-compose-admin-alice` and `dev-compose-admin-bob` when exercising the examples locally.

The Compose credentials are intentionally local-only. A production deployment should place the console and API behind the organization authentication boundary and supply administrator credentials from its identity integration; do not ship the static development tokens.

### Local frontend process

Run the console against a separately started local API:

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

Open `http://localhost:5173`. Vite proxies `/api` to `http://127.0.0.1:8080` by default; set `AUTH_SCOPE_API_URL` when the API is elsewhere. The local non-Compose administrator token is `auth-scope-dev-admin-token` unless overridden through the API environment variables above.

## Operator Console

The React console is an operational surface for people responsible for AI-agent authority, not a marketing site. It starts at the work queue and keeps the high-frequency paths close:

- Overview summarizes active missions, pending approvals, containment, agents, projections, and recent events.
- Missions lets operators search, filter, inspect effective authority, view lineage/events/raw evidence, and complete or revoke active authority.
- Approvals supports mission proposal review and expansion decisions, including version-drift warnings before committing changes.
- Agents shows workload identities, key bindings, lineage, and revocation.
- Containment creates incident controls, shows active rules, inspects blast radius, and lifts rules with recorded reasons.
- Governance manages approval rules and tool authorization contracts.
- Projections lists external credentials and supports revocation.
- Audit searches immutable events and opens full evidence payloads.
- Workbench verifies signed decision artifacts and projection tokens without retaining credential material.

The console keeps administrator bearer credentials in React memory only. Refreshing the browser clears the credential and returns the operator to the connection screen. Empty states, retry actions, not-found routes, and detail-load failures should always provide a path back to the relevant inventory or queue.

## Samples

The [`samples`](samples) directory contains small companion apps that demonstrate how Auth Scope can govern real agent workflows. Start with [`samples/governed-coding-agent-workbench`](samples/governed-coding-agent-workbench), a static sample that models a Codex/OpenCode-style coding agent asking for mission authority before file edits, tests, dependency changes, pull requests, and deployment.

## GitHub Pages Demo Hosting

The static Governed Coding Agent Workbench sample can be published with GitHub Pages using the workflow at [`.github/workflows/deploy-sample-pages.yml`](.github/workflows/deploy-sample-pages.yml). See [GitHub Pages Deployment](docs/github-pages.md) for the automated path and the remaining manual repository settings.

## Test

```sh
go test ./...
```

Coverage:

```sh
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Frontend checks:

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm test:coverage
pnpm build
pnpm e2e
```

If dependencies are already installed but `pnpm` is not on your shell path, the same package scripts can be run through npm:

```sh
cd frontend
npm run typecheck
npm run lint
npm run test
npm run build
npm run e2e
```

The frontend enforces 80% minimum coverage for statements, branches, functions, and lines. See [`frontend/README.md`](frontend/README.md) for local development and API proxy details.

## How to Contribute

Start from a topic branch and keep each change centered on one mission-authority capability. The service is intentionally organized around narrow ports and cohesive integration packages, so prefer adding behavior behind existing service/store/HTTP boundaries before introducing new cross-cutting abstractions.

When changing backend behavior, add focused unit tests and HTTP or e2e coverage under `internal/mission`. Storage-backed features should update both the in-memory store and PostgreSQL store, include forward and rollback migrations under `internal/mission/store/migrations`, and cover persistence behavior in `internal/mission/store`.

When changing the API contract, update [`openapi/auth-scope-v1.yaml`](openapi/auth-scope-v1.yaml), regenerate frontend declarations with `cd frontend && pnpm generate:api`, and keep this route inventory current. When changing the console, follow the existing operator workflow patterns: no dead-end states, clear empty/error paths, credentials held in memory only, and coverage at or above the 80% threshold.

When adding an external integration, keep provider-specific logic in a dedicated package such as `internal/mission/integrations/{provider}` and expose it through small mission-layer ports. Integration code should accept already-verified identity or webhook facts unless the feature explicitly owns verification, and it should record auditable events for governance decisions.

Before opening or updating a PR, run the relevant checks:

```sh
go test ./...
ruby -e 'require "yaml"; YAML.load_file("openapi/auth-scope-v1.yaml", aliases: true); puts "yaml ok"'
cd frontend
pnpm typecheck
pnpm lint
pnpm test:coverage
pnpm build
pnpm e2e
```

PR descriptions should summarize the user-facing capability, call out migrations or operational changes, list validation commands, and note any known MVP limitations.

## API

```text
GET  /healthz
GET  /.well-known/mission-authority
GET  /.well-known/authzen-configuration
POST /access/v1/evaluation
POST /access/v1/evaluations
POST /v1/agents
GET  /v1/admin/session
GET  /v1/operations/summary
GET  /v1/agents
GET  /v1/agents/{agent_id}
POST /v1/agents/{agent_id}/revoke
POST /v1/mission-proposals
GET  /v1/mission-proposals
GET  /v1/mission-proposals/{proposal_id}
POST /v1/mission-proposals/{proposal_id}/approve
GET  /v1/missions
POST /v1/missions/{mission_ref}/evaluate
POST /v1/missions/{mission_ref}/authority/negotiations
POST /v1/missions/{mission_ref}/expansion-requests
POST /v1/missions/{mission_ref}/resume
POST /v1/missions/{mission_ref}/delegate
POST /v1/missions/{mission_ref}/revoke
POST /v1/missions/{mission_ref}/complete
GET  /v1/missions/{mission_ref}/introspect
GET  /v1/missions/{mission_ref}/lineage
GET  /v1/agents/{agent_id}/lineage
GET  /v1/expansion-requests
GET  /v1/expansion-requests/{expansion_id}
POST /v1/expansion-requests/{expansion_id}/approve
POST /v1/expansion-requests/{expansion_id}/deny
GET  /v1/authority/negotiations/{negotiation_id}
POST /v1/decision-artifacts/verify
POST /v1/tool-contracts
GET  /v1/tool-contracts
GET  /v1/tool-contracts/{tool_name}
POST /v1/tool-calls/authorize
POST /v1/missions/{mission_ref}/projections
GET  /v1/projections
GET  /v1/projections/{projection_id}/status
POST /v1/projections/{projection_id}/revoke
POST /v1/projections/verify
POST /v1/projections/exchange
POST /v1/projections/credentials/verify
POST /v1/integrations/github/repositories
GET  /v1/integrations/github/repositories
POST /v1/integrations/github/webhooks
POST /v1/integrations/github/check-runs/plan
POST /v1/integrations/okta/app-bindings
GET  /v1/integrations/okta/app-bindings
POST /v1/integrations/okta/authority-context/resolve
POST /v1/integrations/entra/app-registrations
GET  /v1/integrations/entra/app-registrations
POST /v1/integrations/entra/authority-context/resolve
POST /v1/integrations/slack/workspace-bindings
GET  /v1/integrations/slack/workspace-bindings
POST /v1/integrations/slack/message-actions/authorize
POST /v1/integrations/atlassian/site-bindings
GET  /v1/integrations/atlassian/site-bindings
POST /v1/integrations/atlassian/jira/issues/authorize
POST /v1/integrations/atlassian/confluence/pages/authorize
POST /v1/integrations/salesforce/org-bindings
GET  /v1/integrations/salesforce/org-bindings
POST /v1/integrations/salesforce/records/authorize
POST /v1/missions/{mission_ref}/leases
POST /v1/leases/{lease_id}/refresh
POST /v1/approval-rules
GET  /v1/approval-rules
POST /v1/expansion-requests/{expansion_id}/approvals
POST /v1/policy-bundles
GET  /v1/policy-bundles
GET  /v1/policy-bundles/{bundle_id}
POST /v1/policy-bundles/{bundle_id}/activate
POST /v1/policy-bundles/{bundle_id}/simulate
POST /v1/containment-rules
GET  /v1/containment-rules
GET  /v1/containment-rules/{rule_id}
POST /v1/containment-rules/{rule_id}/lift
GET  /v1/containment-rules/{rule_id}/blast-radius
GET  /v1/events
GET  /v1/events/stream
```

The OpenAPI file at [`openapi/auth-scope-v1.yaml`](openapi/auth-scope-v1.yaml) documents the full MVP HTTP route inventory and is used to generate the frontend TypeScript declarations.

## Example

For Docker Compose governance calls, set:

```sh
ADMIN_TOKEN=dev-compose-admin-alice
```

Register an agent identity. `public_key` is a base64url-encoded raw Ed25519 public key:

```sh
curl -s http://localhost:8080/v1/agents \
  -H "authorization: Bearer $ADMIN_TOKEN" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "agent": {"provider": "https://agents.example.com", "client_id": "research-agent", "instance_id": "inst_123"},
    "public_key": "BASE64URL_ED25519_PUBLIC_KEY"
  }'
```

Signed runtime requests use:

```text
x-auth-scope-agent-id: {agent_id}
x-auth-scope-nonce: {unique_nonce}
x-auth-scope-signature: base64url(Ed25519-Sign(canonical_request))
```

The canonical request is:

```text
AUTH-SCOPE-SIGNATURE-V1
{HTTP_METHOD}
{REQUEST_URI}
{hex_sha256_body}
{nonce}
```

Create a proposal:

```sh
curl -s http://localhost:8080/v1/mission-proposals \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "principal": {"subject": "alice@example.com", "issuer": "https://idp.example.com"},
    "agent": {"provider": "https://agents.example.com", "client_id": "research-agent", "instance_id": "inst_123"},
    "intent": {"objective": "Prepare Q3 board packet", "business_context": "Finance close"},
    "authority_region": {
      "resources": [{"type": "drive_folder", "id": "board", "actions": ["read", "write_draft"]}],
      "forbidden_actions": ["send_external"]
    },
    "conditions": [{"id": "close-open", "expression": "finance.close.status == '\''open'\''", "evaluation": "per_action", "on_failure": "suspend"}],
    "lifecycle": {"expires_at": "2026-07-21T12:00:00Z"}
  }'
```

Approve it:

```sh
curl -s http://localhost:8080/v1/mission-proposals/{proposal_id}/approve \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{"approval_evidence": {"method": "demo"}}'
```

Evaluate an action. Runtime authority endpoints require signed agent headers: `x-auth-scope-agent-id`, `x-auth-scope-nonce`, and `x-auth-scope-signature`.

```sh
curl -s http://localhost:8080/v1/missions/{mission_ref}/evaluate \
  -H 'x-auth-scope-agent-id: {agent_id}' \
  -H 'x-auth-scope-nonce: {nonce}' \
  -H 'x-auth-scope-signature: {signature}' \
  -H 'content-type: application/json' \
  -d '{
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "action": {"type": "tool_call", "resource": {"type": "drive_folder", "id": "board"}, "operation": "read"},
    "context": {"finance.close.status": "open", "risk": "low", "reversible": true}
  }'
```

Risky out-of-scope actions return `require_approval` and create a pending mission expansion request:

```sh
curl -s http://localhost:8080/v1/expansion-requests/{expansion_id}/approve \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{"approval_evidence": {"method": "demo"}}'
```

Use `/approve` for a single direct approval. Use `/approvals` when approval rules require multiple independent administrators; each request records the authenticated bearer principal as one approver.

Negotiate a requested authority change before creating an expansion. Fully safe requests return `accepted`; partially safe requests return `counteroffered` with `proposed_authority` and `denied_authority`; risky out-of-scope requests can return `requires_human_approval`:

```sh
curl -s http://localhost:8080/v1/missions/{mission_ref}/authority/negotiations \
  -H 'x-auth-scope-agent-id: {agent_id}' \
  -H 'x-auth-scope-nonce: {nonce}' \
  -H 'x-auth-scope-signature: {signature}' \
  -H 'content-type: application/json' \
  -d '{
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "requested_authority": {
      "resources": [{"type": "drive_folder", "id": "board", "actions": ["read", "delete"]}]
    },
    "context": {"risk": "low", "reversible": true}
  }'
```

Verify a signed decision artifact and retrieve its stored policy evidence:

```sh
curl -s http://localhost:8080/v1/decision-artifacts/verify \
  -H 'content-type: application/json' \
  -d '{"decision_artifact": "{decision_artifact}"}'
```

Register a tool contract and authorize a tool call through the mission evaluator:

```sh
curl -s http://localhost:8080/v1/tool-contracts \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tool_name": "drive.read",
    "resource_type": "drive_folder",
    "resource_id_param": "folder_id",
    "operation": "read",
    "required_context": ["finance.close.status"]
  }'

curl -s http://localhost:8080/v1/tool-calls/authorize \
  -H 'x-auth-scope-agent-id: {agent_id}' \
  -H 'x-auth-scope-nonce: {nonce}' \
  -H 'x-auth-scope-signature: {signature}' \
  -H 'content-type: application/json' \
  -d '{
    "mission_ref": "{mission_ref}",
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "tool_name": "drive.read",
    "arguments": {"folder_id": "board"},
    "context": {"finance.close.status": "open"}
  }'
```

Mint a short-lived projection grant, exchange it for a scoped brokered credential, and verify that credential against the intended tool binding:

```sh
curl -s http://localhost:8080/v1/missions/{mission_ref}/projections \
  -H 'content-type: application/json' \
  -d '{
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "type": "tool_gateway_token",
    "scopes": ["drive.read"],
    "audience": "tool-gateway",
    "tool_name": "drive.read",
    "resource": {"type": "drive_folder", "id": "board"},
    "operation": "read",
    "ttl_seconds": 300
  }'

curl -s http://localhost:8080/v1/projections/exchange \
  -H 'content-type: application/json' \
  -d '{
    "projection_token": "{projection_token}",
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "nonce": "gateway-request-001",
    "requested_scopes": ["drive.read"],
    "audience": "tool-gateway",
    "tool_name": "drive.read",
    "resource": {"type": "drive_folder", "id": "board"},
    "operation": "read",
    "ttl_seconds": 120
  }'

curl -s http://localhost:8080/v1/projections/credentials/verify \
  -H 'content-type: application/json' \
  -d '{
    "token": "{access_token}",
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "audience": "tool-gateway",
    "tool_name": "drive.read",
    "resource": {"type": "drive_folder", "id": "board"},
    "operation": "read"
  }'
```

Projection grants and exchanged credentials are signed with richer claims including `jti`, issuer, audience, token use, mission version, agent identity, authority hash, scopes, and confirmation binding. Exchanges require a nonce; replaying the same nonce returns a conflict. Revoking a projection marks its exchange records revoked, and credential verification fails once the projection, mission, or broker exchange is revoked or expired.

Create a mission lease for cached gateway decisions and refresh it before each batch:

```sh
curl -s http://localhost:8080/v1/missions/{mission_ref}/leases \
  -H 'content-type: application/json' \
  -d '{
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "ttl_seconds": 60
  }'

curl -s http://localhost:8080/v1/leases/{lease_id}/refresh \
  -H 'content-type: application/json' \
  -d '{"actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"}}'
```

Require multiple approvers for a sensitive expansion:

```sh
curl -s http://localhost:8080/v1/approval-rules \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "applies_to": "expansion",
    "resource_type": "slack_channel",
    "resource_id": "board",
    "operation": "post_update",
    "risk_level": "high",
    "required_approvals": 2,
    "allowed_issuers": ["https://idp.example.com"]
  }'

curl -s http://localhost:8080/v1/expansion-requests/{expansion_id}/approvals \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{"reason": "reviewed and approved"}'
```

Create and activate a policy-as-code bundle that blocks high-risk external sends. Active bundles are evaluated during mission decisions and the applied bundle/rule IDs are written into decision artifacts and evidence:

```sh
curl -s http://localhost:8080/v1/policy-bundles \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "version": "mission-policy/high-risk-send-v1",
    "name": "High-risk external-send guardrail",
    "rules": [{
      "rule_id": "deny-high-risk-external-send",
      "priority": 10,
      "effect": "deny",
      "match": {"operations": ["send_external"], "base_decisions": ["allow"]},
      "conditions": [{"id": "high-risk", "expression": "context.risk == \"high\""}],
      "reason_codes": ["POLICY_HIGH_RISK_EXTERNAL_SEND"],
      "human_reason": "High-risk external sends are blocked by enterprise policy."
    }]
  }'

curl -s http://localhost:8080/v1/policy-bundles/{bundle_id}/activate \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{"reason": "approved enterprise guardrail"}'

curl -s http://localhost:8080/v1/policy-bundles/{bundle_id}/simulate \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "mission_ref": "{mission_ref}",
    "base_decision": "allow",
    "evaluation": {
      "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
      "action": {"type": "tool_call", "resource": {"type": "email", "id": "board"}, "operation": "send_external"},
      "context": {"risk": "high"}
    }
  }'
```

Gateways can subscribe to a Server-Sent Events snapshot stream:

```sh
curl -N http://localhost:8080/v1/events/stream \
  -H "authorization: Bearer ${ADMIN_TOKEN}"
```

Bind a GitHub repository to a mission so a GitHub App or Action can publish mission-authority checks on pull requests:

```sh
curl -s http://localhost:8080/v1/integrations/github/repositories \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "repository": "tauliang/auth-scope",
    "mission_ref": "{mission_ref}",
    "default_branch": "main",
    "required_checks": ["Auth Scope Mission Authority"]
  }'
```

Create mission authority for repository paths with `repo_path` resources. Prefix grants support `/**`, which is useful for coding-agent PRs:

```json
{
  "authority_region": {
    "resources": [
      {
        "type": "repo_path",
        "id": "tauliang/auth-scope:frontend/**",
        "actions": ["edit"]
      }
    ],
    "forbidden_actions": ["delete", "deploy_production"]
  }
}
```

An integration worker can ask Auth Scope for the check-run payload to publish to GitHub:

```sh
curl -s http://localhost:8080/v1/integrations/github/check-runs/plan \
  -H 'content-type: application/json' \
  -d '{
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "repository": "tauliang/auth-scope",
    "head_sha": "abc123",
    "pull_request": 42,
    "branch": "agent/fix-filter",
    "changed_files": [
      {"path": "frontend/src/features/missions/MissionDetailPage.tsx", "status": "modified"}
    ],
    "context": {"risk": "low", "reversible": true}
  }'
```

GitHub webhook requests should be sent to `/v1/integrations/github/webhooks` with `X-GitHub-Event`, `X-GitHub-Delivery`, and `X-Hub-Signature-256`. The service records signed deliveries as audit events and links them to a repository binding when one exists.

Bind an Okta OIDC application and group allowlist to a mission so an identity-aware gateway can convert verified Okta claims into canonical mission-authority context:

```sh
curl -s http://localhost:8080/v1/integrations/okta/app-bindings \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "issuer": "https://acme.okta.com/oauth2/default",
    "client_id": "0oaabc123client",
    "app_id": "0oaapp123",
    "app_label": "Auth Scope Console",
    "mission_ref": "{mission_ref}",
    "required_groups": ["Mission Operators"],
    "admin_groups": ["Mission Admins"],
    "group_match_mode": "any"
  }'
```

After a gateway verifies the Okta token signature and audience, it can resolve the token claims and optionally ask for a mission decision in the same call:

```sh
curl -s http://localhost:8080/v1/integrations/okta/authority-context/resolve \
  -H 'content-type: application/json' \
  -d '{
    "claims": {
      "iss": "https://acme.okta.com/oauth2/default",
      "cid": "0oaabc123client",
      "sub": "00u1agent",
      "groups": ["Mission Operators"],
      "scp": ["openid", "groups"]
    },
    "context": {"risk": "low", "reversible": true},
    "evaluation": {
      "mission_version_seen": 1,
      "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
      "action": {
        "type": "tool_call",
        "resource": {"type": "drive_folder", "id": "board"},
        "operation": "read"
      }
    }
  }'
```

Bind a Microsoft Entra app registration and group allowlist to a mission in the same pattern:

```sh
curl -s http://localhost:8080/v1/integrations/entra/app-registrations \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "issuer": "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
    "client_id": "00000000-0000-0000-0000-000000000000",
    "app_id": "app_entra_001",
    "app_name": "Auth Scope Console",
    "mission_ref": "{mission_ref}",
    "required_groups": ["Mission Operators"],
    "admin_groups": ["Mission Admins"],
    "group_match_mode": "any"
  }'
```

After a gateway verifies the Entra token signature and audience, it can resolve `iss`, `appid`/`azp`/`aud`, `sub`, `groups`, and `roles` claims into mission context:

```sh
curl -s http://localhost:8080/v1/integrations/entra/authority-context/resolve \
  -H 'content-type: application/json' \
  -d '{
    "claims": {
      "iss": "https://login.microsoftonline.com/12345678-1234-1234-1234-123456789012/v2.0",
      "azp": "00000000-0000-0000-0000-000000000000",
      "sub": "user@example.onmicrosoft.com",
      "groups": ["Mission Operators"],
      "roles": ["Reader"]
    },
    "context": {"risk": "low", "reversible": true},
    "evaluation": {
      "mission_version_seen": 1,
      "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
      "action": {
        "type": "tool_call",
        "resource": {"type": "drive_folder", "id": "board"},
        "operation": "read"
      }
    }
  }'
```

Bind a Slack workspace to a mission so a gateway or Slack app can authorize message actions against mission authority:

```sh
curl -s http://localhost:8080/v1/integrations/slack/workspace-bindings \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "workspace_id": "T12345678",
    "workspace_name": "Acme Corp",
    "workspace_url": "https://acme-corp.slack.com",
    "mission_ref": "{mission_ref}",
    "required_roles": ["Workspace Admin"],
    "admin_roles": ["Owner"],
    "allowed_channels": ["C11111111"],
    "blocked_channels": ["C99999999"],
    "allowed_actions": ["post_message", "react_message"],
    "role_match_mode": "any"
  }'
```

After a gateway verifies the Slack user/workspace facts, it can authorize a message action and optionally ask for a mission decision in the same call. Channel allowlists fail closed, so `channel_id` must be present when `allowed_channels` is configured:

```sh
curl -s http://localhost:8080/v1/integrations/slack/message-actions/authorize \
  -H 'content-type: application/json' \
  -d '{
    "workspace_id": "T12345678",
    "user_id": "U12345678",
    "email": "user@example.com",
    "roles": ["Workspace Admin"],
    "channel_id": "C11111111",
    "action": "post_message",
    "context": {"risk": "low", "reversible": true},
    "evaluation": {
      "mission_version_seen": 1,
      "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
      "action": {
        "type": "message_event",
        "resource": {"type": "message", "id": "msg_123", "channel_id": "C11111111"},
        "operation": "post"
      }
    }
  }'
```

Bind an Atlassian Cloud site to a mission so a Jira or Confluence gateway can authorize project and space actions through the same mission authority contract:

```sh
curl -s http://localhost:8080/v1/integrations/atlassian/site-bindings \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "site_url": "https://acme.atlassian.net",
    "cloud_id": "ari:cloud:platform::site/12345678-1234-1234-1234-123456789012",
    "site_name": "Acme Atlassian",
    "mission_ref": "{mission_ref}",
    "jira_project_keys": ["FIN"],
    "confluence_space_keys": ["ENG"],
    "allowed_jira_actions": ["transition_issue", "comment_issue"],
    "allowed_page_actions": ["update_page", "comment_page"],
    "required_groups": ["Mission Operators"],
    "group_match_mode": "any"
  }'
```

After a gateway verifies Atlassian account, group, site, project, and space facts, it can authorize Jira issue or Confluence page actions and optionally ask for a mission decision in the same call:

```sh
curl -s http://localhost:8080/v1/integrations/atlassian/jira/issues/authorize \
  -H 'content-type: application/json' \
  -d '{
    "site_url": "https://acme.atlassian.net",
    "cloud_id": "ari:cloud:platform::site/12345678-1234-1234-1234-123456789012",
    "account_id": "712020:agent-account",
    "email": "agent@example.com",
    "groups": ["Mission Operators"],
    "issue_key": "FIN-77",
    "action": "transition_issue",
    "context": {"risk": "low", "reversible": true},
    "evaluation": {
      "mission_version_seen": 1,
      "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
      "action": {
        "type": "jira_issue_transition",
        "resource": {"type": "jira_issue", "id": "FIN-77"},
        "operation": "transition"
      }
    }
  }'

curl -s http://localhost:8080/v1/integrations/atlassian/confluence/pages/authorize \
  -H 'content-type: application/json' \
  -d '{
    "site_url": "https://acme.atlassian.net",
    "cloud_id": "ari:cloud:platform::site/12345678-1234-1234-1234-123456789012",
    "account_id": "712020:agent-account",
    "email": "agent@example.com",
    "groups": ["Mission Operators"],
    "space_key": "ENG",
    "page_id": "12345",
    "action": "update_page",
    "context": {"risk": "low", "reversible": true},
    "evaluation": {
      "mission_version_seen": 1,
      "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
      "action": {
        "type": "confluence_page_update",
        "resource": {"type": "confluence_page", "id": "ENG:12345"},
        "operation": "update"
      }
    }
  }'
```

Bind a Salesforce org to a mission so a gateway or connected app can authorize CRM record actions against mission authority:

```sh
curl -s http://localhost:8080/v1/integrations/salesforce/org-bindings \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "instance_url": "https://acme.my.salesforce.com",
    "org_id": "00Dxx0000001ABC",
    "org_name": "Acme Salesforce",
    "mission_ref": "{mission_ref}",
    "allowed_object_api_names": ["Account"],
    "allowed_record_type_names": ["Customer"],
    "allowed_actions": ["read_record", "update_record"],
    "required_profiles": ["Standard User"],
    "required_permission_sets": ["CRM Agent"],
    "admin_permission_sets": ["Mission Admin"],
    "permission_set_match_mode": "any"
  }'
```

After a gateway verifies Salesforce org, user, profile, permission-set, object, and record facts, it can authorize record actions and optionally ask for a mission decision in the same call:

```sh
curl -s http://localhost:8080/v1/integrations/salesforce/records/authorize \
  -H 'content-type: application/json' \
  -d '{
    "instance_url": "https://acme.my.salesforce.com",
    "org_id": "00Dxx0000001ABC",
    "object_api_name": "Account",
    "record_id": "001xx000003DGbY",
    "record_type_name": "Customer",
    "user_id": "005xx000001",
    "username": "agent@example.com",
    "email": "agent@example.com",
    "profile": "Standard User",
    "permission_sets": ["CRM Agent", "Mission Admin"],
    "action": "update_record",
    "context": {"risk": "low", "reversible": true},
    "evaluation": {
      "mission_version_seen": 1,
      "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
      "action": {
        "type": "salesforce_record_update",
        "resource": {"type": "salesforce_record", "id": "Account:001xx000003DGbY"},
        "operation": "update"
      }
    }
  }'
```

Create a containment rule during an incident. Active containment blocks evaluation, manual expansion, delegation, projection issuance/verification, lease creation/refresh, and resume when the mission, tenant, agent, principal, tool, or resource matches:

```sh
curl -s http://localhost:8080/v1/containment-rules \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{
    "tenant_id": "demo",
    "target_type": "agent",
    "target_id": "inst_123",
    "reason": "runtime attestation failed"
  }'

curl -s http://localhost:8080/v1/containment-rules/{rule_id}/blast-radius \
  -H "authorization: Bearer ${ADMIN_TOKEN}"

curl -s http://localhost:8080/v1/containment-rules/{rule_id}/lift \
  -H "authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'content-type: application/json' \
  -d '{"reason": "runtime re-attested"}'
```

Inspect accountability lineage for a mission or agent:

```sh
curl -s http://localhost:8080/v1/missions/{mission_ref}/lineage \
  -H "authorization: Bearer ${ADMIN_TOKEN}"
curl -s http://localhost:8080/v1/agents/inst_123/lineage \
  -H "authorization: Bearer ${ADMIN_TOKEN}"
```

AuthZEN-style evaluation:

```sh
curl -s http://localhost:8080/access/v1/evaluation \
  -H 'content-type: application/json' \
  -d '{
    "subject": {"type": "agent", "id": "inst_123", "properties": {"client_id": "research-agent"}},
    "action": {"type": "tool_call", "id": "read"},
    "resource": {"type": "drive_folder", "id": "board"},
    "context": {"mission_ref": "{mission_ref}", "mission_version_seen": 1, "finance.close.status": "open"}
  }'
```

## MVP Boundary

The MVP currently includes the first PostgreSQL persistence slice plus the execution-governance enrichment slice: embedded schema migrations, opaque text identifiers, lossless mission/proposal/event/governance JSON round-trips, delegation traversal indexes, a transactional outbox, token-bound governance administrators, agent identity registration, signed runtime requests, AuthZEN-compatible evaluation, signed decision artifacts, atomic versioned expansion approvals, versioned policy-as-code bundles with simulation, policy evidence storage, MCP-style tool gateway enforcement contracts, signed external projections, brokered scoped credential exchange, mission leases, SSE event streaming, multi-approver expansion policies, centralized containment enforcement, tenant-scoped blast-radius reads, authority negotiation, mission/agent lineage graphs, GitHub repository/check-run integration hooks, Okta app/group authority-context integration hooks, Microsoft Entra app/group/role authority-context integration hooks, Slack workspace/message-action integration hooks, Atlassian Jira/Confluence site/action integration hooks, and Salesforce org/record-action integration hooks. The remaining production work is hardening deployment operations, adding live JWKS/introspection verification where the gateway does not already verify identity-provider tokens or provider-specific user/workspace/site facts, and wiring CI to run the `DATABASE_URL`-gated PostgreSQL conformance test.
