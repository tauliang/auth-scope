# Details

Date : 2026-07-19 09:32:05

Directory /Users/tau/Git/Hacks/auth-scope

Total : 146 files,  28394 codes, 134 comments, 3342 blanks, all 31870 lines

[Summary](results.md) / Details / [Diff Summary](diff.md) / [Diff Details](diff-details.md)

## Files
| filename | language | code | comment | blank | total |
| :--- | :--- | ---: | ---: | ---: | ---: |
| [.github/workflows/ci.yml](/.github/workflows/ci.yml) | YAML | 64 | 0 | 5 | 69 |
| [Dockerfile](/Dockerfile) | Docker | 19 | 1 | 13 | 33 |
| [README.md](/README.md) | Markdown | 420 | 0 | 95 | 515 |
| [cmd/auth-scope/main.go](/cmd/auth-scope/main.go) | Go | 91 | 0 | 14 | 105 |
| [demo/MANUAL\_DEMO\_SCRIPT.md](/demo/MANUAL_DEMO_SCRIPT.md) | Markdown | 281 | 0 | 163 | 444 |
| [demo/README.md](/demo/README.md) | Markdown | 53 | 0 | 24 | 77 |
| [demo/lib/auth-scope-demo.mjs](/demo/lib/auth-scope-demo.mjs) | JavaScript | 200 | 0 | 27 | 227 |
| [demo/mock-data/mission-authority-scenario.json](/demo/mock-data/mission-authority-scenario.json) | JSON | 171 | 0 | 1 | 172 |
| [demo/playwright.config.mjs](/demo/playwright.config.mjs) | JavaScript | 21 | 0 | 1 | 22 |
| [demo/playwright/mission-authority-demo.spec.mjs](/demo/playwright/mission-authority-demo.spec.mjs) | JavaScript | 218 | 0 | 29 | 247 |
| [demo/run-demo.mjs](/demo/run-demo.mjs) | JavaScript | 85 | 0 | 12 | 97 |
| [demo/seed-demo.mjs](/demo/seed-demo.mjs) | JavaScript | 268 | 0 | 24 | 292 |
| [demo/videos/auth-scope-mission-authority-demo-2min-captions.md](/demo/videos/auth-scope-mission-authority-demo-2min-captions.md) | Markdown | 36 | 0 | 3 | 39 |
| [docker-compose.yml](/docker-compose.yml) | YAML | 59 | 0 | 4 | 63 |
| [frontend/Dockerfile](/frontend/Dockerfile) | Docker | 12 | 0 | 3 | 15 |
| [frontend/IMPLEMENTATION\_PLAN.md](/frontend/IMPLEMENTATION_PLAN.md) | Markdown | 559 | 0 | 136 | 695 |
| [frontend/README.md](/frontend/README.md) | Markdown | 50 | 0 | 19 | 69 |
| [frontend/e2e/operator-console.spec.ts](/frontend/e2e/operator-console.spec.ts) | TypeScript | 46 | 0 | 7 | 53 |
| [frontend/eslint.config.js](/frontend/eslint.config.js) | JavaScript | 26 | 0 | 2 | 28 |
| [frontend/index.html](/frontend/index.html) | HTML | 14 | 0 | 1 | 15 |
| [frontend/nginx.conf](/frontend/nginx.conf) | Properties | 30 | 0 | 6 | 36 |
| [frontend/package.json](/frontend/package.json) | JSON | 56 | 0 | 1 | 57 |
| [frontend/playwright.config.ts](/frontend/playwright.config.ts) | TypeScript | 21 | 0 | 2 | 23 |
| [frontend/pnpm-lock.yaml](/frontend/pnpm-lock.yaml) | YAML | 3,253 | 0 | 858 | 4,111 |
| [frontend/src/app/App.test.tsx](/frontend/src/app/App.test.tsx) | TypeScript JSX | 95 | 0 | 15 | 110 |
| [frontend/src/app/App.tsx](/frontend/src/app/App.tsx) | TypeScript JSX | 33 | 0 | 4 | 37 |
| [frontend/src/app/AppShell.tsx](/frontend/src/app/AppShell.tsx) | TypeScript JSX | 65 | 0 | 4 | 69 |
| [frontend/src/app/router.tsx](/frontend/src/app/router.tsx) | TypeScript JSX | 46 | 0 | 7 | 53 |
| [frontend/src/app/styles.css](/frontend/src/app/styles.css) | PostCSS | 393 | 0 | 20 | 413 |
| [frontend/src/app/workflows.test.tsx](/frontend/src/app/workflows.test.tsx) | TypeScript JSX | 400 | 0 | 35 | 435 |
| [frontend/src/features/agents/AgentDetailPage.tsx](/frontend/src/features/agents/AgentDetailPage.tsx) | TypeScript JSX | 34 | 0 | 2 | 36 |
| [frontend/src/features/agents/AgentsPage.tsx](/frontend/src/features/agents/AgentsPage.tsx) | TypeScript JSX | 28 | 0 | 2 | 30 |
| [frontend/src/features/approvals/ApprovalsPage.tsx](/frontend/src/features/approvals/ApprovalsPage.tsx) | TypeScript JSX | 28 | 0 | 2 | 30 |
| [frontend/src/features/approvals/ExpansionReviewPage.tsx](/frontend/src/features/approvals/ExpansionReviewPage.tsx) | TypeScript JSX | 50 | 0 | 2 | 52 |
| [frontend/src/features/approvals/ProposalReviewPage.tsx](/frontend/src/features/approvals/ProposalReviewPage.tsx) | TypeScript JSX | 47 | 0 | 2 | 49 |
| [frontend/src/features/audit/AuditPage.tsx](/frontend/src/features/audit/AuditPage.tsx) | TypeScript JSX | 15 | 0 | 2 | 17 |
| [frontend/src/features/connection/ConnectionPage.tsx](/frontend/src/features/connection/ConnectionPage.tsx) | TypeScript JSX | 52 | 0 | 4 | 56 |
| [frontend/src/features/containment/ContainmentDetailPage.tsx](/frontend/src/features/containment/ContainmentDetailPage.tsx) | TypeScript JSX | 34 | 0 | 2 | 36 |
| [frontend/src/features/containment/ContainmentPage.tsx](/frontend/src/features/containment/ContainmentPage.tsx) | TypeScript JSX | 18 | 0 | 2 | 20 |
| [frontend/src/features/dashboard/DashboardPage.tsx](/frontend/src/features/dashboard/DashboardPage.tsx) | TypeScript JSX | 72 | 0 | 7 | 79 |
| [frontend/src/features/governance/GovernancePage.tsx](/frontend/src/features/governance/GovernancePage.tsx) | TypeScript JSX | 118 | 0 | 8 | 126 |
| [frontend/src/features/missions/MissionDetailPage.tsx](/frontend/src/features/missions/MissionDetailPage.tsx) | TypeScript JSX | 60 | 0 | 3 | 63 |
| [frontend/src/features/missions/MissionsPage.tsx](/frontend/src/features/missions/MissionsPage.tsx) | TypeScript JSX | 40 | 0 | 3 | 43 |
| [frontend/src/features/missions/NewProposalPage.tsx](/frontend/src/features/missions/NewProposalPage.tsx) | TypeScript JSX | 89 | 0 | 5 | 94 |
| [frontend/src/features/projections/ProjectionsPage.tsx](/frontend/src/features/projections/ProjectionsPage.tsx) | TypeScript JSX | 26 | 0 | 2 | 28 |
| [frontend/src/features/workbench/WorkbenchPage.tsx](/frontend/src/features/workbench/WorkbenchPage.tsx) | TypeScript JSX | 12 | 0 | 2 | 14 |
| [frontend/src/main.tsx](/frontend/src/main.tsx) | TypeScript JSX | 10 | 0 | 2 | 12 |
| [frontend/src/shared/api/client.test.ts](/frontend/src/shared/api/client.test.ts) | TypeScript | 63 | 0 | 8 | 71 |
| [frontend/src/shared/api/client.ts](/frontend/src/shared/api/client.ts) | TypeScript | 201 | 0 | 37 | 238 |
| [frontend/src/shared/api/generated.ts](/frontend/src/shared/api/generated.ts) | TypeScript | 2,337 | 66 | 2 | 2,405 |
| [frontend/src/shared/api/types.ts](/frontend/src/shared/api/types.ts) | TypeScript | 257 | 0 | 27 | 284 |
| [frontend/src/shared/auth/SessionProvider.test.tsx](/frontend/src/shared/auth/SessionProvider.test.tsx) | TypeScript JSX | 32 | 0 | 5 | 37 |
| [frontend/src/shared/auth/SessionProvider.tsx](/frontend/src/shared/auth/SessionProvider.tsx) | TypeScript JSX | 52 | 0 | 8 | 60 |
| [frontend/src/shared/components/AsyncState.tsx](/frontend/src/shared/components/AsyncState.tsx) | TypeScript JSX | 20 | 0 | 3 | 23 |
| [frontend/src/shared/components/AuthorityView.tsx](/frontend/src/shared/components/AuthorityView.tsx) | TypeScript JSX | 30 | 0 | 2 | 32 |
| [frontend/src/shared/components/ConfirmDialog.tsx](/frontend/src/shared/components/ConfirmDialog.tsx) | TypeScript JSX | 63 | 0 | 5 | 68 |
| [frontend/src/shared/components/DataTable.tsx](/frontend/src/shared/components/DataTable.tsx) | TypeScript JSX | 43 | 0 | 2 | 45 |
| [frontend/src/shared/components/JsonBlock.tsx](/frontend/src/shared/components/JsonBlock.tsx) | TypeScript JSX | 17 | 0 | 2 | 19 |
| [frontend/src/shared/components/LineageGraph.tsx](/frontend/src/shared/components/LineageGraph.tsx) | TypeScript JSX | 40 | 0 | 3 | 43 |
| [frontend/src/shared/components/PageHeader.tsx](/frontend/src/shared/components/PageHeader.tsx) | TypeScript JSX | 18 | 0 | 2 | 20 |
| [frontend/src/shared/components/StatusBadge.tsx](/frontend/src/shared/components/StatusBadge.tsx) | TypeScript JSX | 31 | 0 | 3 | 34 |
| [frontend/src/shared/components/components.test.tsx](/frontend/src/shared/components/components.test.tsx) | TypeScript JSX | 76 | 0 | 8 | 84 |
| [frontend/src/shared/formatting/index.test.ts](/frontend/src/shared/formatting/index.test.ts) | TypeScript | 24 | 0 | 4 | 28 |
| [frontend/src/shared/formatting/index.ts](/frontend/src/shared/formatting/index.ts) | TypeScript | 18 | 0 | 5 | 23 |
| [frontend/src/testing/setup.ts](/frontend/src/testing/setup.ts) | TypeScript | 29 | 0 | 8 | 37 |
| [frontend/tsconfig.app.json](/frontend/tsconfig.app.json) | JSON | 22 | 0 | 1 | 23 |
| [frontend/tsconfig.json](/frontend/tsconfig.json) | JSON with Comments | 7 | 0 | 1 | 8 |
| [frontend/tsconfig.node.json](/frontend/tsconfig.node.json) | JSON | 12 | 0 | 1 | 13 |
| [frontend/vite.config.ts](/frontend/vite.config.ts) | TypeScript | 19 | 0 | 2 | 21 |
| [frontend/vitest.config.ts](/frontend/vitest.config.ts) | TypeScript | 23 | 0 | 2 | 25 |
| [go.mod](/go.mod) | XML | 4 | 0 | 4 | 8 |
| [internal/mission/admin\_auth.go](/internal/mission/admin_auth.go) | Go | 158 | 0 | 21 | 179 |
| [internal/mission/admin\_auth\_test.go](/internal/mission/admin_auth_test.go) | Go | 100 | 0 | 11 | 111 |
| [internal/mission/advanced\_governance.go](/internal/mission/advanced_governance.go) | Go | 144 | 0 | 21 | 165 |
| [internal/mission/advanced\_governance\_service.go](/internal/mission/advanced_governance_service.go) | Go | 537 | 0 | 27 | 564 |
| [internal/mission/advanced\_governance\_test.go](/internal/mission/advanced_governance_test.go) | Go | 230 | 0 | 17 | 247 |
| [internal/mission/agent\_identity.go](/internal/mission/agent_identity.go) | Go | 98 | 0 | 14 | 112 |
| [internal/mission/agent\_service.go](/internal/mission/agent_service.go) | Go | 133 | 0 | 9 | 142 |
| [internal/mission/authority\_guard.go](/internal/mission/authority_guard.go) | Go | 54 | 0 | 10 | 64 |
| [internal/mission/authzen.go](/internal/mission/authzen.go) | Go | 138 | 0 | 14 | 152 |
| [internal/mission/decision\_artifact.go](/internal/mission/decision_artifact.go) | Go | 109 | 0 | 12 | 121 |
| [internal/mission/e2e\_test.go](/internal/mission/e2e_test.go) | Go | 273 | 0 | 31 | 304 |
| [internal/mission/evaluator.go](/internal/mission/evaluator.go) | Go | 193 | 0 | 20 | 213 |
| [internal/mission/evaluator\_test.go](/internal/mission/evaluator_test.go) | Go | 197 | 0 | 14 | 211 |
| [internal/mission/github.go](/internal/mission/github.go) | Go | 30 | 0 | 8 | 38 |
| [internal/mission/github\_http.go](/internal/mission/github_http.go) | Go | 99 | 0 | 8 | 107 |
| [internal/mission/github\_http\_test.go](/internal/mission/github_http_test.go) | Go | 89 | 0 | 8 | 97 |
| [internal/mission/github\_service.go](/internal/mission/github_service.go) | Go | 118 | 0 | 22 | 140 |
| [internal/mission/github\_test.go](/internal/mission/github_test.go) | Go | 166 | 0 | 11 | 177 |
| [internal/mission/governance.go](/internal/mission/governance.go) | Go | 107 | 0 | 15 | 122 |
| [internal/mission/governance\_more\_test.go](/internal/mission/governance_more_test.go) | Go | 261 | 0 | 19 | 280 |
| [internal/mission/governance\_read.go](/internal/mission/governance_read.go) | Go | 192 | 0 | 11 | 203 |
| [internal/mission/governance\_service.go](/internal/mission/governance_service.go) | Go | 386 | 0 | 25 | 411 |
| [internal/mission/governance\_test.go](/internal/mission/governance_test.go) | Go | 171 | 0 | 14 | 185 |
| [internal/mission/grand\_governance.go](/internal/mission/grand_governance.go) | Go | 73 | 0 | 12 | 85 |
| [internal/mission/grand\_governance\_service.go](/internal/mission/grand_governance_service.go) | Go | 525 | 0 | 47 | 572 |
| [internal/mission/grand\_governance\_test.go](/internal/mission/grand_governance_test.go) | Go | 408 | 0 | 33 | 441 |
| [internal/mission/http.go](/internal/mission/http.go) | Go | 974 | 0 | 63 | 1,037 |
| [internal/mission/http\_contract\_test.go](/internal/mission/http_contract_test.go) | Go | 68 | 0 | 3 | 71 |
| [internal/mission/http\_ports.go](/internal/mission/http_ports.go) | Go | 85 | 0 | 11 | 96 |
| [internal/mission/http\_test.go](/internal/mission/http_test.go) | Go | 553 | 0 | 65 | 618 |
| [internal/mission/integrations/github/helpers.go](/internal/mission/integrations/github/helpers.go) | Go | 259 | 0 | 21 | 280 |
| [internal/mission/integrations/github/service.go](/internal/mission/integrations/github/service.go) | Go | 374 | 0 | 27 | 401 |
| [internal/mission/integrations/github/types.go](/internal/mission/integrations/github/types.go) | Go | 133 | 0 | 16 | 149 |
| [internal/mission/operator.go](/internal/mission/operator.go) | Go | 35 | 0 | 6 | 41 |
| [internal/mission/operator\_http.go](/internal/mission/operator_http.go) | Go | 182 | 0 | 15 | 197 |
| [internal/mission/operator\_http\_test.go](/internal/mission/operator_http_test.go) | Go | 145 | 0 | 16 | 161 |
| [internal/mission/operator\_service.go](/internal/mission/operator_service.go) | Go | 245 | 0 | 17 | 262 |
| [internal/mission/operator\_test.go](/internal/mission/operator_test.go) | Go | 107 | 0 | 11 | 118 |
| [internal/mission/outbox\_test.go](/internal/mission/outbox_test.go) | Go | 61 | 0 | 9 | 70 |
| [internal/mission/ports.go](/internal/mission/ports.go) | Go | 78 | 0 | 14 | 92 |
| [internal/mission/runtime\_config.go](/internal/mission/runtime_config.go) | Go | 43 | 0 | 6 | 49 |
| [internal/mission/security\_test.go](/internal/mission/security_test.go) | Go | 396 | 0 | 42 | 438 |
| [internal/mission/service.go](/internal/mission/service.go) | Go | 697 | 3 | 44 | 744 |
| [internal/mission/service\_more\_test.go](/internal/mission/service_more_test.go) | Go | 270 | 0 | 30 | 300 |
| [internal/mission/service\_test.go](/internal/mission/service_test.go) | Go | 192 | 0 | 21 | 213 |
| [internal/mission/store.go](/internal/mission/store.go) | Go | 688 | 2 | 61 | 751 |
| [internal/mission/store/governance\_read.go](/internal/mission/store/governance_read.go) | Go | 204 | 0 | 13 | 217 |
| [internal/mission/store/governance\_read\_test.go](/internal/mission/store/governance_read_test.go) | Go | 136 | 0 | 19 | 155 |
| [internal/mission/store/migrations/001\_initial\_schema.down.sql](/internal/mission/store/migrations/001_initial_schema.down.sql) | MS SQL | 11 | 1 | 2 | 14 |
| [internal/mission/store/migrations/001\_initial\_schema.up.sql](/internal/mission/store/migrations/001_initial_schema.up.sql) | MS SQL | 44 | 0 | 4 | 48 |
| [internal/mission/store/migrations/002\_outbox\_table.down.sql](/internal/mission/store/migrations/002_outbox_table.down.sql) | MS SQL | 1 | 1 | 1 | 3 |
| [internal/mission/store/migrations/002\_outbox\_table.up.sql](/internal/mission/store/migrations/002_outbox_table.up.sql) | MS SQL | 10 | 0 | 2 | 12 |
| [internal/mission/store/migrations/003\_outbox\_processed\_table.down.sql](/internal/mission/store/migrations/003_outbox_processed_table.down.sql) | MS SQL | 1 | 1 | 1 | 3 |
| [internal/mission/store/migrations/003\_outbox\_processed\_table.up.sql](/internal/mission/store/migrations/003_outbox_processed_table.up.sql) | MS SQL | 4 | 0 | 1 | 5 |
| [internal/mission/store/migrations/004\_agent\_identities.down.sql](/internal/mission/store/migrations/004_agent_identities.down.sql) | MS SQL | 6 | 0 | 1 | 7 |
| [internal/mission/store/migrations/004\_agent\_identities.up.sql](/internal/mission/store/migrations/004_agent_identities.up.sql) | MS SQL | 24 | 0 | 3 | 27 |
| [internal/mission/store/migrations/005\_governance\_controls.down.sql](/internal/mission/store/migrations/005_governance_controls.down.sql) | MS SQL | 3 | 0 | 1 | 4 |
| [internal/mission/store/migrations/005\_governance\_controls.up.sql](/internal/mission/store/migrations/005_governance_controls.up.sql) | MS SQL | 26 | 0 | 5 | 31 |
| [internal/mission/store/migrations/006\_advanced\_governance.down.sql](/internal/mission/store/migrations/006_advanced_governance.down.sql) | MS SQL | 4 | 0 | 1 | 5 |
| [internal/mission/store/migrations/006\_advanced\_governance.up.sql](/internal/mission/store/migrations/006_advanced_governance.up.sql) | MS SQL | 43 | 0 | 8 | 51 |
| [internal/mission/store/migrations/007\_grand\_governance.down.sql](/internal/mission/store/migrations/007_grand_governance.down.sql) | MS SQL | 2 | 0 | 1 | 3 |
| [internal/mission/store/migrations/007\_grand\_governance.up.sql](/internal/mission/store/migrations/007_grand_governance.up.sql) | MS SQL | 24 | 0 | 4 | 28 |
| [internal/mission/store/migrations/008\_github\_integrations.down.sql](/internal/mission/store/migrations/008_github_integrations.down.sql) | MS SQL | 4 | 0 | 1 | 5 |
| [internal/mission/store/migrations/008\_github\_integrations.up.sql](/internal/mission/store/migrations/008_github_integrations.up.sql) | MS SQL | 25 | 0 | 4 | 29 |
| [internal/mission/store/postgres.go](/internal/mission/store/postgres.go) | Go | 1,615 | 59 | 235 | 1,909 |
| [internal/mission/store/postgres\_test.go](/internal/mission/store/postgres_test.go) | Go | 264 | 0 | 24 | 288 |
| [internal/mission/store/postgres\_unit\_test.go](/internal/mission/store/postgres_unit_test.go) | Go | 1,287 | 0 | 108 | 1,395 |
| [internal/mission/store\_test.go](/internal/mission/store_test.go) | Go | 458 | 0 | 30 | 488 |
| [internal/mission/types.go](/internal/mission/types.go) | Go | 217 | 0 | 34 | 251 |
| [openapi/auth-scope-v1.yaml](/openapi/auth-scope-v1.yaml) | YAML | 1,032 | 0 | 1 | 1,033 |
| [samples/README.md](/samples/README.md) | Markdown | 9 | 0 | 7 | 16 |
| [samples/governed-coding-agent-workbench/README.md](/samples/governed-coding-agent-workbench/README.md) | Markdown | 34 | 0 | 14 | 48 |
| [samples/governed-coding-agent-workbench/app.js](/samples/governed-coding-agent-workbench/app.js) | JavaScript | 467 | 0 | 41 | 508 |
| [samples/governed-coding-agent-workbench/index.html](/samples/governed-coding-agent-workbench/index.html) | HTML | 144 | 0 | 12 | 156 |
| [samples/governed-coding-agent-workbench/styles.css](/samples/governed-coding-agent-workbench/styles.css) | PostCSS | 465 | 0 | 80 | 545 |

[Summary](results.md) / Details / [Diff Summary](diff.md) / [Diff Details](diff-details.md)