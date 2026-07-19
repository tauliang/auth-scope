# Auth Scope Samples

This directory contains sample applications that show how Auth Scope can govern real agent workflows.

## Governed Coding Agent Workbench

`samples/governed-coding-agent-workbench` is a static, dependency-free sample app for demonstrating how Auth Scope can sit between a coding agent and developer tools.

Open it directly in a browser:

```bash
open samples/governed-coding-agent-workbench/index.html
```

The sample models Codex/OpenCode-style actions such as reading files, editing files, running tests, installing dependencies, opening pull requests, and attempting deployment. It shows the corresponding mission-authority decisions, approval queue, containment posture, GitHub check preview, and audit ledger.
