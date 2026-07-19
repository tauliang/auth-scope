# Mission Authority Demo

This folder contains a runnable demo for the primary Auth Scope use cases:

1. Register a workload agent with an Ed25519 public key.
2. Create and approve a mission with bounded authority.
3. Sign runtime authorization requests as the agent.
4. Require human approval for an out-of-scope authority expansion.
5. Approve the final expansion in the operator console.
6. Authorize a tool call after authority expands.
7. Show a stale lease and activate containment.
8. Verify signed decision evidence and a containment-blocked projection token.
9. Inspect mission, agent, projection, containment, and audit views without UI dead ends.

## Prerequisites

Start the application stack:

```bash
docker-compose up --build
```

In another terminal, make sure frontend development dependencies are installed so the demo can use Playwright from `frontend/node_modules`:

```bash
cd frontend
corepack pnpm install
cd ..
```

## Run The Demo

```bash
node demo/run-demo.mjs
```

For a stage demo where people can watch the browser:

```bash
node demo/run-demo.mjs --headed
```

The runner writes per-run state to `demo/.generated/mission-authority-state.json`. That file is ignored because it contains generated IDs and a throwaway demo private key used only to sign the mock agent requests.

## Recorded Walkthrough

The repository includes a two-minute captioned recording of the primary demo flow:

- `demo/videos/auth-scope-mission-authority-demo-2min-captioned.webm` (`121.08s`)
- `demo/videos/auth-scope-mission-authority-demo-2min-captions.md`

## Manual Stakeholder Demo

Use `demo/MANUAL_DEMO_SCRIPT.md` when a presenter wants to drive the stakeholder demo by hand. It includes setup commands, seed instructions, generated values to copy, exact UI fields, a talk track, expected outcomes, and troubleshooting notes.

## Useful Options

```bash
node demo/run-demo.mjs --seed-only
node demo/run-demo.mjs --ui-only
node demo/run-demo.mjs --debug
```

Environment variables:

```bash
AUTH_SCOPE_API_URL=http://127.0.0.1:8080
AUTH_SCOPE_FRONTEND_URL=http://127.0.0.1:3000
AUTH_SCOPE_ADMIN_TOKEN_ALICE=dev-compose-admin-alice
AUTH_SCOPE_ADMIN_TOKEN_BOB=dev-compose-admin-bob
AUTH_SCOPE_DEMO_STATE=demo/.generated/mission-authority-state.json
```

## Demo Story

The mock scenario is a finance-close board packet. The agent starts with read and draft access to a Drive folder. A Slack posting action is outside the mission scope, so the service creates an expansion request. Bob supplies the first approval during seeding; Alice supplies the second approval in the UI. After approval, the script signs a tool gateway authorization, refreshes an old lease to show it has gone stale, and then activates containment for the agent so the console can show blast radius and blocked projection verification.
