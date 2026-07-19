#!/usr/bin/env node
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import {
  apiUrlFromEnv,
  defaultStatePath,
  frontendUrlFromEnv,
  repoRoot,
  waitForFrontend,
  waitForHealth,
} from "./lib/auth-scope-demo.mjs";
import { seedDemo } from "./seed-demo.mjs";

export async function runDemo(argv = process.argv.slice(2)) {
  const flags = new Set(argv);
  if (flags.has("--help") || flags.has("-h")) {
    printHelp();
    return;
  }

  const apiUrl = apiUrlFromEnv();
  const frontendUrl = frontendUrlFromEnv();
  const statePath = process.env.AUTH_SCOPE_DEMO_STATE ?? defaultStatePath;

  if (!flags.has("--ui-only")) {
    await seedDemo({ apiUrl, frontendUrl, statePath, log: true });
  } else {
    await waitForHealth(apiUrl);
  }

  if (flags.has("--seed-only")) {
    console.log("Seed complete. Skipping browser automation because --seed-only was provided.");
    return;
  }

  await waitForFrontend(frontendUrl);
  await runPlaywright({ frontendUrl, apiUrl, statePath, headed: flags.has("--headed"), debug: flags.has("--debug") });

  console.log("");
  console.log("Demo complete.");
  console.log(`Frontend: ${frontendUrl}`);
  console.log(`State: ${statePath}`);
}

function runPlaywright({ frontendUrl, apiUrl, statePath, headed, debug }) {
  const playwrightBin = join(repoRoot, "frontend", "node_modules", ".bin", "playwright");
  if (!existsSync(playwrightBin)) {
    throw new Error("Playwright is not installed under frontend/node_modules. Run `cd frontend && corepack pnpm install` first.");
  }

  const args = ["test", "--config", "demo/playwright.config.mjs"];
  if (headed) args.push("--headed");
  if (debug) args.push("--debug");

  return new Promise((resolve, reject) => {
    const child = spawn(playwrightBin, args, {
      cwd: repoRoot,
      stdio: "inherit",
      env: {
        ...process.env,
        AUTH_SCOPE_API_URL: apiUrl,
        AUTH_SCOPE_FRONTEND_URL: frontendUrl,
        AUTH_SCOPE_DEMO_STATE: statePath,
      },
    });
    child.on("error", reject);
    child.on("exit", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`Playwright demo failed with exit code ${code}`));
    });
  });
}

function printHelp() {
  console.log(`Mission Authority demo

Usage:
  node demo/run-demo.mjs [--headed] [--seed-only] [--ui-only] [--debug]

Environment:
  AUTH_SCOPE_API_URL             API URL, default http://127.0.0.1:8080
  AUTH_SCOPE_FRONTEND_URL        Frontend URL, default http://127.0.0.1:3000
  AUTH_SCOPE_ADMIN_TOKEN_ALICE   Alice admin token, default dev-compose-admin-alice
  AUTH_SCOPE_ADMIN_TOKEN_BOB     Bob admin token, default dev-compose-admin-bob
  AUTH_SCOPE_DEMO_STATE          State file, default demo/.generated/mission-authority-state.json
`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  runDemo().catch((error) => {
    console.error(error);
    process.exitCode = 1;
  });
}
