import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SessionProvider, useApi, useSession } from "./SessionProvider";

function Harness() {
  const { status, session, connect, disconnect } = useSession();
  return <><span>{status}</span><span>{session?.principal.subject}</span><button onClick={() => void connect("token").catch(() => undefined)}>Connect</button><button onClick={disconnect}>Disconnect</button></>;
}

describe("SessionProvider", () => {
  it("connects and clears the in-memory session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ principal: { subject: "alice", issuer: "issuer" }, capabilities: {}, api_version: "v1" }), { status: 200 })));
    const user = userEvent.setup(); render(<SessionProvider><Harness /></SessionProvider>);
    expect(screen.getByText("disconnected")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Connect" }));
    expect(await screen.findByText("connected")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Disconnect" }));
    expect(screen.getByText("disconnected")).toBeInTheDocument();
  });

  it("returns to disconnected after authentication failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ message: "Unauthorized" }), { status: 401 })));
    const user = userEvent.setup(); render(<SessionProvider><Harness /></SessionProvider>);
    await user.click(screen.getByRole("button", { name: "Connect" }));
    expect(await screen.findByText("disconnected")).toBeInTheDocument();
  });

  it("guards both hooks outside their valid context", () => {
    function MissingProvider() { useSession(); return null; }
    function MissingClient() { useApi(); return null; }
    expect(() => render(<MissingProvider />)).toThrow("useSession must be used inside SessionProvider");
    expect(() => render(<SessionProvider><MissingClient /></SessionProvider>)).toThrow("API client unavailable without an authenticated session");
  });
});
