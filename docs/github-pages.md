# GitHub Pages Deployment

Auth Scope can publish the static Governed Coding Agent Workbench sample on GitHub Pages for free. This hosting path is for the browser-only sample app; the Go API, PostgreSQL database, and Docker Compose stack still need a runtime host such as a VM, container platform, Fly.io, Render, Railway, or an internal environment.

## What Is Automated

The workflow at [`.github/workflows/deploy-sample-pages.yml`](../.github/workflows/deploy-sample-pages.yml) publishes [`samples/governed-coding-agent-workbench`](../samples/governed-coding-agent-workbench) as a GitHub Pages artifact.

It runs when:

- Changes to the sample or workflow are merged to `main`.
- A maintainer starts the workflow manually with `workflow_dispatch`.

The workflow:

- Checks out the repository.
- Configures GitHub Pages.
- Uploads the static sample directory as the Pages artifact.
- Deploys the artifact to the repository's Pages site.

## Manual Steps Still Needed

1. Confirm the repository is eligible for free GitHub Pages. Public repositories work on GitHub Free; private repository Pages availability depends on the account or organization plan.
2. Open the repository on GitHub and go to `Settings` -> `Pages`.
3. Under `Build and deployment`, set `Source` to `GitHub Actions`.
4. Confirm GitHub Actions are enabled for the repository or organization.
5. Merge the deployment workflow to `main`.
6. Wait for the `Deploy Auth Scope Sample to GitHub Pages` workflow to complete.
7. Open `Settings` -> `Pages` and use the published site URL. For this repository, the default URL should be similar to `https://tauliang.github.io/auth-scope/`.

GitHub may take a few minutes to publish the site after the first successful deployment.

## Local Verification

The sample remains dependency-free and can be opened directly before publishing:

```sh
open samples/governed-coding-agent-workbench/index.html
```

## Operational Notes

- Do not place API credentials, admin tokens, OAuth secrets, or provider secrets in the static sample directory.
- GitHub Pages cannot run the backend API or PostgreSQL. Use `docker compose up --build` for the local full stack, or deploy the backend to a separate service and configure any future hosted frontend to call that API.
- If the Pages site does not update, check the workflow run in the GitHub `Actions` tab and confirm `Settings` -> `Pages` is set to `GitHub Actions`.

Reference:

- [GitHub Pages documentation](https://docs.github.com/en/pages)
