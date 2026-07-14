import { Link, Outlet } from "@tanstack/react-router";
import {
  Activity,
  Bot,
  Boxes,
  CircleGauge,
  FlaskConical,
  LogOut,
  Menu,
  Network,
  OctagonAlert,
  ScrollText,
  ShieldCheck,
  X,
} from "lucide-react";
import { Suspense, useState } from "react";
import { useSession } from "../shared/auth/SessionProvider";
import { LoadingState } from "../shared/components/AsyncState";

const navigation = [
  { to: "/", label: "Overview", icon: CircleGauge },
  { to: "/missions", label: "Missions", icon: ShieldCheck },
  { to: "/approvals", label: "Approvals", icon: ScrollText },
  { to: "/agents", label: "Agents", icon: Bot },
  { to: "/containment", label: "Containment", icon: OctagonAlert },
  { to: "/governance", label: "Governance", icon: Boxes },
  { to: "/projections", label: "Projections", icon: Network },
  { to: "/audit", label: "Audit", icon: Activity },
  { to: "/workbench", label: "Workbench", icon: FlaskConical },
] as const;

export function AppShell() {
  const { session, disconnect } = useSession();
  const [open, setOpen] = useState(false);

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">Skip to content</a>
      <aside className={`sidebar ${open ? "sidebar-open" : ""}`}>
        <div className="brand-lockup">
          <div className="brand-mark"><ShieldCheck size={23} /></div>
          <div><strong>Auth Scope</strong><span>Authority console</span></div>
        </div>
        <nav aria-label="Primary navigation">
          {navigation.map(({ to, label, icon: Icon }) => (
            <Link key={to} to={to} onClick={() => setOpen(false)} className="nav-link" activeProps={{ className: "nav-link nav-link-active" }} activeOptions={{ exact: to === "/" }}>
              <Icon size={18} aria-hidden="true" /><span>{label}</span>
            </Link>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div className="principal-avatar">{session?.principal.subject.slice(0, 1).toUpperCase()}</div>
          <div className="principal-copy"><strong>{session?.principal.subject}</strong><span>{session?.principal.issuer || "Local authority"}</span></div>
          <button className="icon-button icon-button-inverse" onClick={disconnect} aria-label="Sign out"><LogOut size={17} /></button>
        </div>
      </aside>
      {open ? <button className="sidebar-scrim" aria-label="Close navigation" onClick={() => setOpen(false)} /> : null}
      <div className="workspace">
        <header className="topbar">
          <button className="icon-button mobile-menu" onClick={() => setOpen((value) => !value)} aria-label={open ? "Close navigation" : "Open navigation"}>{open ? <X size={20} /> : <Menu size={20} />}</button>
          <div className="environment"><span className="signal-dot" />Connected <span className="environment-version">API {session?.api_version}</span></div>
          <div className="topbar-principal">{session?.principal.subject}</div>
        </header>
        <main id="main-content" className="page-content"><Suspense fallback={<LoadingState label="Loading view" />}><Outlet /></Suspense></main>
      </div>
    </div>
  );
}
