import * as Dialog from "@radix-ui/react-dialog";
import { AlertTriangle, X } from "lucide-react";
import { useState, type ReactNode } from "react";

export function ConfirmDialog({ trigger, title, description, confirmLabel, tone = "danger", onConfirm }: {
  trigger: ReactNode;
  title: string;
  description: string;
  confirmLabel: string;
  tone?: "danger" | "primary";
  onConfirm: (reason: string) => Promise<unknown>;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  function handleOpenChange(nextOpen: boolean) {
    if (pending) return;
    setOpen(nextOpen);
    if (!nextOpen) {
      setReason("");
      setError("");
    }
  }

  async function confirm() {
    if (!reason.trim()) return;
    setPending(true);
    setError("");
    try {
      await onConfirm(reason.trim());
      setReason("");
      setOpen(false);
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "The action failed.");
    } finally {
      setPending(false);
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={handleOpenChange}>
      <Dialog.Trigger asChild>{trigger}</Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content className="dialog-content">
          <div className={`dialog-icon dialog-icon-${tone}`}><AlertTriangle size={20} /></div>
          <Dialog.Title>{title}</Dialog.Title>
          <Dialog.Description>{description}</Dialog.Description>
          <label className="field">
            <span>Reason</span>
            <textarea value={reason} onChange={(event) => setReason(event.target.value)} rows={3} autoFocus />
          </label>
          {error ? <p className="form-error" role="alert">{error}</p> : null}
          <div className="dialog-actions">
            <Dialog.Close asChild><button className="button button-secondary" disabled={pending}>Cancel</button></Dialog.Close>
            <button className={`button button-${tone}`} disabled={pending || !reason.trim()} onClick={confirm}>
              {pending ? "Working..." : confirmLabel}
            </button>
          </div>
          <Dialog.Close className="icon-button dialog-close" aria-label="Close"><X size={18} /></Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
