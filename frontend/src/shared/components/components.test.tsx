import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ColumnDef } from "@tanstack/react-table";
import { describe, expect, it, vi } from "vitest";
import { AuthorityView } from "./AuthorityView";
import { ConfirmDialog } from "./ConfirmDialog";
import { DataTable } from "./DataTable";
import { ErrorState, LoadingState } from "./AsyncState";
import { JsonBlock } from "./JsonBlock";
import { PageHeader } from "./PageHeader";
import { StatusBadge } from "./StatusBadge";
import { ApiError } from "../api/client";

describe("shared components", () => {
  it("renders semantic status and authority information", () => {
    render(<><StatusBadge value="pending_approval" /><AuthorityView authority={{ resources: [{ type: "drive", id: "board", actions: ["read"], constraints: { region: "us" } }], forbidden_actions: ["delete"] }} /></>);
    expect(screen.getByText("Pending approval")).toBeInTheDocument();
    expect(screen.getByText("board")).toBeInTheDocument();
    expect(screen.getByText("read")).toBeInTheDocument();
    expect(screen.getByText("delete")).toBeInTheDocument();
  });

  it("renders table rows and empty state", () => {
    const columns: ColumnDef<{ name: string }>[] = [{ header: "Name", accessorKey: "name" }];
    const { rerender } = render(<DataTable data={[{ name: "Alpha" }]} columns={columns} />);
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    rerender(<DataTable data={[]} columns={columns} emptyTitle="Nothing here" />);
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
  });

  it("requires a reason before confirmation", async () => {
    const user = userEvent.setup(); const confirm = vi.fn().mockResolvedValue(undefined);
    render(<ConfirmDialog trigger={<button>Revoke</button>} title="Revoke mission" description="Terminal" confirmLabel="Confirm" onConfirm={confirm} />);
    await user.click(screen.getByRole("button", { name: "Revoke" }));
    const button = screen.getByRole("button", { name: "Confirm" });
    expect(button).toBeDisabled();
    await user.type(screen.getByLabelText("Reason"), "Incident response");
    await user.click(button);
    expect(confirm).toHaveBeenCalledWith("Incident response");
  });

  it("shows normalized error details and loading state", () => {
    render(<><LoadingState label="Loading missions" /><ErrorState error={new ApiError("Conflict", 409, "conflict", "req-1")} /></>);
    expect(screen.getByText("Loading missions")).toBeInTheDocument();
    expect(screen.getByText("Conflict")).toBeInTheDocument();
    expect(screen.getByText("Request req-1")).toBeInTheDocument();
  });

  it("copies structured JSON", async () => {
    const user = userEvent.setup(); const copy = vi.spyOn(navigator.clipboard, "writeText");
    render(<JsonBlock value={{ valid: true }} label="Evidence" />);
    await user.click(screen.getByRole("button", { name: "Copy Evidence" }));
    expect(copy).toHaveBeenCalledWith(expect.stringContaining('"valid": true'));
  });

  it("covers optional and fallback presentation states", async () => {
    const retry = vi.fn();
    const user = userEvent.setup();
    render(<>
      <PageHeader title="Bare heading" />
      <StatusBadge value="custom_state" />
      <AuthorityView authority={{ resources: [] }} />
      <ErrorState error={{ reason: "offline" }} onRetry={retry} />
    </>);
    expect(screen.getByText("custom state")).toBeInTheDocument();
    expect(screen.getByText("The request could not be completed.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("keeps a failed confirmation open and reports the failure", async () => {
    const user = userEvent.setup();
    render(<ConfirmDialog trigger={<button>Complete</button>} title="Complete mission" description="Terminal" confirmLabel="Confirm" tone="primary" onConfirm={() => Promise.reject("failed")} />);
    await user.click(screen.getByRole("button", { name: "Complete" }));
    await user.type(screen.getByLabelText("Reason"), "Done");
    await user.click(screen.getByRole("button", { name: "Confirm" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("The action failed.");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(screen.getByRole("button", { name: "Complete" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Reason")).toHaveValue("");
  });
});
