import { AlertTriangle, Ban, CheckCircle2, Clock3, PauseCircle, ShieldCheck } from "lucide-react";

const statusConfig: Record<string, { label: string; tone: string; icon: typeof CheckCircle2 }> = {
  active: { label: "Active", tone: "positive", icon: CheckCircle2 },
  allow: { label: "Allow", tone: "positive", icon: CheckCircle2 },
  approved: { label: "Approved", tone: "positive", icon: ShieldCheck },
  completed: { label: "Completed", tone: "neutral", icon: CheckCircle2 },
  pending: { label: "Pending", tone: "warning", icon: Clock3 },
  pending_approval: { label: "Pending approval", tone: "warning", icon: Clock3 },
  require_approval: { label: "Approval required", tone: "warning", icon: AlertTriangle },
  suspended: { label: "Suspended", tone: "warning", icon: PauseCircle },
  deny: { label: "Deny", tone: "danger", icon: Ban },
  denied: { label: "Denied", tone: "danger", icon: Ban },
  revoked: { label: "Revoked", tone: "danger", icon: Ban },
  rejected: { label: "Rejected", tone: "danger", icon: Ban },
  expired: { label: "Expired", tone: "neutral", icon: Clock3 },
  lifted: { label: "Lifted", tone: "neutral", icon: CheckCircle2 },
};

export function StatusBadge({ value }: { value: string }) {
  const config = statusConfig[value.toLowerCase()] ?? {
    label: value.replaceAll("_", " "),
    tone: "info",
    icon: ShieldCheck,
  };
  const Icon = config.icon;
  return (
    <span className={`status-badge status-${config.tone}`}>
      <Icon size={13} aria-hidden="true" />
      {config.label}
    </span>
  );
}
