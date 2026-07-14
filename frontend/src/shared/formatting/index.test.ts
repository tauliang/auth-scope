import { describe, expect, it, vi } from "vitest";
import { formatDate, formatRelative, shortID, titleCase } from ".";

describe("formatting", () => {
  it("formats dates and invalid values", () => {
    expect(formatDate("2026-07-18T12:30:00Z")).toContain("Jul 18, 2026");
    expect(formatDate()).toBe("Not set");
    expect(formatDate("invalid")).toBe("Invalid date");
  });

  it("formats relative dates", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-18T13:00:00Z"));
    expect(formatRelative("2026-07-18T12:00:00Z")).toContain("ago");
    expect(formatRelative()).toBe("Unknown");
    expect(formatRelative("bad")).toBe("Unknown");
    vi.useRealTimers();
  });

  it("shortens identifiers and titles enum values", () => {
    expect(shortID("short")).toBe("short");
    expect(shortID("mission-reference-that-is-long", 8)).toBe("mission-...long");
    expect(titleCase("pending_approval")).toBe("Pending Approval");
    expect(titleCase("oauth_mcp_api")).toBe("OAuth MCP API");
    expect(titleCase("mission.approved")).toBe("Mission Approved");
  });
});
