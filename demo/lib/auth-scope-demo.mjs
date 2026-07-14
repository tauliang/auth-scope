import { createHash, createPrivateKey, generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const currentFile = fileURLToPath(import.meta.url);
export const demoDir = dirname(dirname(currentFile));
export const repoRoot = dirname(demoDir);
export const defaultScenarioPath = join(demoDir, "mock-data", "mission-authority-scenario.json");
export const defaultStatePath = join(demoDir, ".generated", "mission-authority-state.json");

export function apiUrlFromEnv() {
  return trimTrailingSlash(process.env.AUTH_SCOPE_API_URL ?? "http://127.0.0.1:8080");
}

export function frontendUrlFromEnv() {
  return trimTrailingSlash(
    process.env.AUTH_SCOPE_FRONTEND_URL ?? process.env.FRONTEND_URL ?? "http://127.0.0.1:3000",
  );
}

export function adminTokensFromEnv() {
  return {
    alice: process.env.AUTH_SCOPE_ADMIN_TOKEN_ALICE ?? process.env.AUTH_SCOPE_ADMIN_TOKEN ?? "dev-compose-admin-alice",
    bob: process.env.AUTH_SCOPE_ADMIN_TOKEN_BOB ?? "dev-compose-admin-bob",
  };
}

export function trimTrailingSlash(value) {
  return String(value).replace(/\/+$/, "");
}

export function generateRunId() {
  const stamp = new Date().toISOString().replace(/\D/g, "").slice(0, 14);
  return `${stamp}-${randomUUID().slice(0, 8)}`;
}

export async function readJSON(path) {
  return JSON.parse(await readFile(path, "utf8"));
}

export async function writeJSON(path, value) {
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, `${JSON.stringify(value, null, 2)}\n`, "utf8");
}

export async function loadScenario({ scenarioPath = defaultScenarioPath, runId = generateRunId() } = {}) {
  const raw = await readJSON(scenarioPath);
  return {
    runId,
    scenario: materialize(raw, { run_id: runId }),
  };
}

export async function loadState(path = process.env.AUTH_SCOPE_DEMO_STATE ?? defaultStatePath) {
  return readJSON(path);
}

export async function saveState(state, path = process.env.AUTH_SCOPE_DEMO_STATE ?? defaultStatePath) {
  await writeJSON(path, state);
  return state;
}

export async function waitForHealth(apiUrl, { timeoutMs = 60000, intervalMs = 1000 } = {}) {
  const deadline = Date.now() + timeoutMs;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${trimTrailingSlash(apiUrl)}/healthz`);
      if (response.ok) return;
      lastError = new Error(`healthz returned ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await delay(intervalMs);
  }
  throw new Error(`Auth Scope API did not become healthy at ${apiUrl}: ${lastError?.message ?? "timeout"}`);
}

export async function waitForFrontend(frontendUrl, { timeoutMs = 60000, intervalMs = 1000 } = {}) {
  const deadline = Date.now() + timeoutMs;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(trimTrailingSlash(frontendUrl));
      if (response.ok) return;
      lastError = new Error(`frontend returned ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await delay(intervalMs);
  }
  throw new Error(`Auth Scope frontend did not become reachable at ${frontendUrl}: ${lastError?.message ?? "timeout"}`);
}

export async function adminRequest({ apiUrl, token, method = "GET", path, body }) {
  return requestJSON({
    apiUrl,
    method,
    path,
    body,
    headers: {
      authorization: `Bearer ${token}`,
    },
  });
}

export async function publicRequest({ apiUrl, method = "GET", path, body }) {
  return requestJSON({ apiUrl, method, path, body });
}

export async function signedRequest({ apiUrl, agentId, privateKeyPem, method = "POST", path, body, nonce }) {
  const bodyText = JSON.stringify(body ?? {});
  const signature = signAgentRequest({
    method,
    path,
    bodyText,
    nonce,
    privateKeyPem,
  });
  return requestJSON({
    apiUrl,
    method,
    path,
    bodyText,
    headers: {
      "x-auth-scope-agent-id": agentId,
      "x-auth-scope-nonce": nonce,
      "x-auth-scope-signature": signature,
    },
  });
}

export async function requestJSON({ apiUrl, method = "GET", path, body, bodyText, headers = {} }) {
  const url = new URL(path, `${trimTrailingSlash(apiUrl)}/`);
  const init = {
    method,
    headers: {
      accept: "application/json",
      "x-request-id": `demo-${randomUUID()}`,
      ...headers,
    },
  };
  if (bodyText !== undefined) {
    init.body = bodyText;
    init.headers["content-type"] = "application/json";
  } else if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers["content-type"] = "application/json";
  }

  const response = await fetch(url, init);
  const text = await response.text();
  let parsed;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = { raw: text };
    }
  }
  if (!response.ok) {
    const message = parsed?.message ?? parsed?.error ?? response.statusText;
    throw new Error(`${method} ${path} failed with ${response.status}: ${message}`);
  }
  return parsed;
}

export function createEd25519AgentKeys() {
  const { publicKey, privateKey } = generateKeyPairSync("ed25519");
  const spki = Buffer.from(publicKey.export({ type: "spki", format: "der" }));
  const rawPublicKey = spki.subarray(spki.length - 32);
  return {
    publicKey: rawPublicKey.toString("base64url"),
    privateKeyPem: privateKey.export({ type: "pkcs8", format: "pem" }),
  };
}

export function signAgentRequest({ method, path, bodyText, nonce, privateKeyPem }) {
  const bodyHash = createHash("sha256").update(bodyText).digest("hex");
  const canonical = [
    "AUTH-SCOPE-SIGNATURE-V1",
    method.toUpperCase(),
    path,
    bodyHash,
    nonce,
  ].join("\n");
  return sign(null, Buffer.from(canonical), createPrivateKey(privateKeyPem)).toString("base64url");
}

export function nonce(runId, label) {
  return `demo-${runId}-${label}-${randomUUID()}`;
}

export function actorFromState(state) {
  return {
    agent_instance_id: state.agent.instance_id,
    client_id: state.agent.client_id,
  };
}

export function requireString(value, label) {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`${label} was not returned by the service`);
  }
  return value;
}

export function short(value, length = 12) {
  return String(value).slice(0, length);
}

function materialize(value, vars) {
  if (Array.isArray(value)) return value.map((item) => materialize(item, vars));
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, materialize(item, vars)]));
  }
  if (typeof value === "string") {
    return Object.entries(vars).reduce((next, [key, item]) => next.replaceAll(`{${key}}`, item), value);
  }
  return value;
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
