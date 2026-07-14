# Auth Scope Operator Console

React 19 and TypeScript operator console for reviewing and intervening in AI
mission authority. The Go service remains the only domain backend; this SPA
uses its authenticated `/v1` API through a same-origin `/api` proxy.

## Selected stack

- React 19 with TypeScript
- Vite
- TanStack Router, Query, and Table
- React Hook Form and Zod
- CSS and CSS custom properties
- Vitest, Testing Library, and Playwright

## Local development

Start the API from the repository root:

```sh
go run ./cmd/auth-scope
```

Then start the console:

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm dev
```

Open `http://localhost:5173`. The local API default administrator token is
`auth-scope-dev-admin-token`. Set `AUTH_SCOPE_API_URL` when the API is not at
`http://127.0.0.1:8080`.

For the complete PostgreSQL-backed stack, run `docker compose up --build` from
the repository root and open `http://localhost:3000` with
`dev-compose-admin-alice`.

## Verification

```sh
pnpm typecheck
pnpm lint
pnpm test:coverage
pnpm build
pnpm e2e
```

Coverage must remain at or above 80% for statements, branches, functions, and
lines. Playwright exercises Chromium desktop and mobile projects. The E2E test
uses network interception for a deterministic UI smoke path; Go API behavior is
covered by the repository's HTTP and service tests.

## API contract

The frontend contract is [`../openapi/auth-scope-v1.yaml`](../openapi/auth-scope-v1.yaml).
Regenerate its TypeScript declarations after contract changes:

```sh
pnpm generate:api
```

Bearer credentials are held only in React memory. They are not written to a
URL, browser storage, logs, or error messages.

The original architecture and staged delivery rationale remain in
[`IMPLEMENTATION_PLAN.md`](./IMPLEMENTATION_PLAN.md).
