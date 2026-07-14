import { createRootRoute, createRoute, createRouter, Link } from "@tanstack/react-router";
import { lazy } from "react";
import { AppShell } from "./AppShell";

const DashboardPage = lazy(() => import("../features/dashboard/DashboardPage").then((module) => ({ default: module.DashboardPage })));
const MissionsPage = lazy(() => import("../features/missions/MissionsPage").then((module) => ({ default: module.MissionsPage })));
const MissionDetailPage = lazy(() => import("../features/missions/MissionDetailPage").then((module) => ({ default: module.MissionDetailPage })));
const NewProposalPage = lazy(() => import("../features/missions/NewProposalPage").then((module) => ({ default: module.NewProposalPage })));
const ApprovalsPage = lazy(() => import("../features/approvals/ApprovalsPage").then((module) => ({ default: module.ApprovalsPage })));
const ProposalReviewPage = lazy(() => import("../features/approvals/ProposalReviewPage").then((module) => ({ default: module.ProposalReviewPage })));
const ExpansionReviewPage = lazy(() => import("../features/approvals/ExpansionReviewPage").then((module) => ({ default: module.ExpansionReviewPage })));
const AgentsPage = lazy(() => import("../features/agents/AgentsPage").then((module) => ({ default: module.AgentsPage })));
const AgentDetailPage = lazy(() => import("../features/agents/AgentDetailPage").then((module) => ({ default: module.AgentDetailPage })));
const ContainmentPage = lazy(() => import("../features/containment/ContainmentPage").then((module) => ({ default: module.ContainmentPage })));
const ContainmentDetailPage = lazy(() => import("../features/containment/ContainmentDetailPage").then((module) => ({ default: module.ContainmentDetailPage })));
const GovernancePage = lazy(() => import("../features/governance/GovernancePage").then((module) => ({ default: module.GovernancePage })));
const ProjectionsPage = lazy(() => import("../features/projections/ProjectionsPage").then((module) => ({ default: module.ProjectionsPage })));
const AuditPage = lazy(() => import("../features/audit/AuditPage").then((module) => ({ default: module.AuditPage })));
const WorkbenchPage = lazy(() => import("../features/workbench/WorkbenchPage").then((module) => ({ default: module.WorkbenchPage })));

const rootRoute = createRootRoute({
  component: AppShell,
  notFoundComponent: () => <div className="not-found"><strong>Page not found</strong><Link to="/">Return to overview</Link></div>,
});

const routes = [
  createRoute({ getParentRoute: () => rootRoute, path: "/", component: DashboardPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/missions", component: MissionsPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/missions/new", component: NewProposalPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/missions/$missionRef", component: MissionDetailPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/approvals", component: ApprovalsPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/approvals/proposals/$proposalId", component: ProposalReviewPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/approvals/expansions/$expansionId", component: ExpansionReviewPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/agents", component: AgentsPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/agents/$agentId", component: AgentDetailPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/containment", component: ContainmentPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/containment/$ruleId", component: ContainmentDetailPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/governance", component: GovernancePage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/projections", component: ProjectionsPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/audit", component: AuditPage }),
  createRoute({ getParentRoute: () => rootRoute, path: "/workbench", component: WorkbenchPage }),
];

const routeTree = rootRoute.addChildren(routes);

export const router = createRouter({ routeTree, defaultPreload: "intent", scrollRestoration: true });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
