# auth-scope

`auth-scope` is an MVP Mission Authority Service for AI agents. It models mission-scoped delegated authority as a first-class object that can be approved, evaluated, delegated, completed, revoked, and audited independently of token validity.

The first slice is intentionally small and runnable:

- Go HTTP service with no third-party dependencies
- In-memory Mission ledger and event log
- Mission proposal and approval flow
- Synchronous action evaluation
- Resume checks for agent harnesses
- Strict-subset delegation for child missions
- Cascade revocation/completion semantics
- Well-known discovery document

## Run

```sh
go run ./cmd/auth-scope
```

The server listens on `:8080` by default. Override it with `AUTH_SCOPE_ADDR`.

```sh
AUTH_SCOPE_ADDR=:9090 go run ./cmd/auth-scope
```

## Test

```sh
go test ./...
```

Coverage:

```sh
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

## API

```text
GET  /healthz
GET  /.well-known/mission-authority
POST /v1/mission-proposals
POST /v1/mission-proposals/{proposal_id}/approve
POST /v1/missions/{mission_ref}/evaluate
POST /v1/missions/{mission_ref}/resume
POST /v1/missions/{mission_ref}/delegate
POST /v1/missions/{mission_ref}/revoke
POST /v1/missions/{mission_ref}/complete
GET  /v1/missions/{mission_ref}/introspect
GET  /v1/events
```

## Example

Create a proposal:

```sh
curl -s http://localhost:8080/v1/mission-proposals \
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
  -H 'content-type: application/json' \
  -d '{"approver": {"subject": "alice@example.com", "issuer": "https://idp.example.com"}, "approval_evidence": {"method": "demo"}}'
```

Evaluate an action:

```sh
curl -s http://localhost:8080/v1/missions/{mission_ref}/evaluate \
  -H 'content-type: application/json' \
  -d '{
    "mission_version_seen": 1,
    "actor": {"agent_instance_id": "inst_123", "client_id": "research-agent"},
    "action": {"type": "tool_call", "resource": {"type": "drive_folder", "id": "board"}, "operation": "read"},
    "context": {"finance.close.status": "open", "risk": "low", "reversible": true}
  }'
```

## MVP Boundary

This branch starts with an in-memory store so the core authority semantics are easy to inspect and test. The next production step is replacing the store with Postgres tables, a transactional outbox, and signed projections for OAuth/MCP integrations.
