# auth-scope

`auth-scope` is an MVP Mission Authority Service for AI agents. It models mission-scoped delegated authority as a first-class object that can be approved, evaluated, delegated, completed, revoked, and audited independently of token validity.

The first slice is intentionally small and runnable:

- Go HTTP service with in-memory and PostgreSQL-backed stores
- React operator console for authority review, intervention, and audit workflows
- Same-origin Docker Compose console/API deployment with an nginx `/api` proxy
- Embedded PostgreSQL migrations and transactional outbox publishing
- Agent identity registry with Ed25519 request signatures and nonce replay protection
- AuthZEN-style runtime authorization endpoints for PEP/PDP integration
- Signed decision artifacts for independent evaluation evidence
- Evaluation evidence ledger with policy version and condition results
- Mission proposal and approval flow
- Mission expansion requests for risky out-of-scope actions
- Synchronous action evaluation
- Tool gateway contracts for MCP-style enforcement adapters
- Signed OAuth/MCP/tool projections bound to mission version and agent identity
- Mission leases for gateway cache invalidation and fail-closed refresh
- Multi-approver approval rules for sensitive expansions
- Authority negotiation for safe subset counteroffers before expansion
- Containment rules with blast-radius inspection and fail-closed enforcement
- Mission and agent lineage graphs for accountability tracing
- Resume checks for agent harnesses
- Strict-subset delegation for child missions
- Cascade revocation/completion semantics
- Well-known discovery document

## Run

### Docker Compose quickstart

Start PostgreSQL, the API, and the operator console with Docker Compose:

```sh
docker compose up --build
```

Open the operator console at `http://localhost:3000` and authenticate with the development token `dev-compose-admin-alice`. The API remains available at `http://localhost:8080`. The service applies embedded migrations automatically when `DATABASE_URL` is set.

Override any host port when needed:

```sh
AUTH_SCOPE_FRONTEND_PORT=3100 AUTH_SCOPE_PORT=9090 AUTH_SCOPE_POSTGRES_PORT=15432 docker compose up --build
```

### Local API process

Run the API locally without Docker:

```sh
go run ./cmd/auth-scope
```

The server listens on `:8080` by default and uses the in-memory store unless `DATABASE_URL` is set. Override the address with `AUTH_SCOPE_ADDR`. Decision artifacts and projection tokens are signed with `AUTH_SCOPE_DECISION_SECRET`; a development-only default is used when it is not set.

Set `AUTH_SCOPE_MODE=production` or `AUTH_SCOPE_ENV=production` for fail-closed startup checks. Production mode requires `DATABASE_URL`, explicit administrator credentials, and a non-placeholder `AUTH_SCOPE_DECISION_SECRET` of at least 32 characters. The production binary also requires signed agent requests on runtime authority endpoints such as mission evaluation, AuthZEN evaluation, delegation, projections, leases, and tool-call authorization.

Governance and audit endpoints require a bearer token bound to an administrator principal. Configure one administrator with `AUTH_SCOPE_ADMIN_TOKEN`, `AUTH_SCOPE_ADMIN_SUBJECT`, and `AUTH_SCOPE_ADMIN_ISSUER`, or configure multiple independent approvers with `AUTH_SCOPE_ADMIN_CREDENTIALS`:

```sh
AUTH_SCOPE_ADMIN_CREDENTIALS='[{"token":"alice-secret","subject":"alice@example.com","issuer":"https://idp.example.com"},{"token":"bob-secret","subject":"bob@example.com","issuer":"https://idp.example.com"}]' go run ./cmd/auth-scope
```

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
GET  /v1/tool-contracts/{tool_name}
POST /v1/tool-calls/authorize
POST /v1/missions/{mission_ref}/projections
GET  /v1/projections/{projection_id}/status
POST /v1/projections/{projection_id}/revoke
POST /v1/projections/verify
POST /v1/missions/{mission_ref}/leases
POST /v1/leases/{lease_id}/refresh
POST /v1/approval-rules
GET  /v1/approval-rules
POST /v1/expansion-requests/{expansion_id}/approvals
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

Mint and verify a short-lived projection for an external gateway:

```sh
curl -s http://localhost:8080/v1/missions/{mission_ref}/projections \
  -H 'content-type: application/json' \
  -d '{
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "type": "mcp_context",
    "ttl_seconds": 300
  }'

curl -s http://localhost:8080/v1/projections/verify \
  -H 'content-type: application/json' \
  -d '{"token": "{projection_token}"}'
```

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

Gateways can subscribe to a Server-Sent Events snapshot stream:

```sh
curl -N http://localhost:8080/v1/events/stream \
  -H "authorization: Bearer ${ADMIN_TOKEN}"
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

This branch now includes the first PostgreSQL persistence slice plus the execution-governance enrichment slice: embedded schema migrations, opaque text identifiers, lossless mission/proposal/event/governance JSON round-trips, delegation traversal indexes, a transactional outbox, token-bound governance administrators, agent identity registration, signed runtime requests, AuthZEN-compatible evaluation, signed decision artifacts, atomic versioned expansion approvals, policy evidence storage, MCP-style tool gateway enforcement contracts, signed external projections, mission leases, SSE event streaming, multi-approver expansion policies, centralized containment enforcement, tenant-scoped blast-radius reads, authority negotiation, and mission/agent lineage graphs. The remaining production work is hardening deployment operations, adding richer signed projections for OAuth/MCP integrations, and wiring CI to run the `DATABASE_URL`-gated PostgreSQL conformance test.
