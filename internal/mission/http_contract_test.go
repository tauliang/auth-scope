package mission

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAPIDocumentsHTTPRouteInventory(t *testing.T) {
	data, err := os.ReadFile("../../openapi/auth-scope-v1.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	spec := string(data)
	requiredPaths := []string{
		"/healthz",
		"/.well-known/mission-authority",
		"/.well-known/authzen-configuration",
		"/access/v1/evaluation",
		"/access/v1/evaluations",
		"/v1/admin/session",
		"/v1/operations/summary",
		"/v1/agents",
		"/v1/agents/{agent_id}",
		"/v1/agents/{agent_id}/revoke",
		"/v1/agents/{agent_id}/lineage",
		"/v1/mission-proposals",
		"/v1/mission-proposals/{proposal_id}",
		"/v1/mission-proposals/{proposal_id}/approve",
		"/v1/missions",
		"/v1/missions/{mission_ref}/evaluate",
		"/v1/missions/{mission_ref}/authority/negotiations",
		"/v1/missions/{mission_ref}/expansion-requests",
		"/v1/missions/{mission_ref}/resume",
		"/v1/missions/{mission_ref}/delegate",
		"/v1/missions/{mission_ref}/revoke",
		"/v1/missions/{mission_ref}/complete",
		"/v1/missions/{mission_ref}/introspect",
		"/v1/missions/{mission_ref}/lineage",
		"/v1/missions/{mission_ref}/projections",
		"/v1/missions/{mission_ref}/leases",
		"/v1/expansion-requests",
		"/v1/expansion-requests/{expansion_id}",
		"/v1/expansion-requests/{expansion_id}/approve",
		"/v1/expansion-requests/{expansion_id}/deny",
		"/v1/expansion-requests/{expansion_id}/approvals",
		"/v1/authority/negotiations/{negotiation_id}",
		"/v1/decision-artifacts/verify",
		"/v1/tool-contracts",
		"/v1/tool-contracts/{tool_name}",
		"/v1/tool-calls/authorize",
		"/v1/projections",
		"/v1/projections/{projection_id}/status",
		"/v1/projections/{projection_id}/revoke",
		"/v1/projections/verify",
		"/v1/leases/{lease_id}/refresh",
		"/v1/approval-rules",
		"/v1/containment-rules",
		"/v1/containment-rules/{rule_id}",
		"/v1/containment-rules/{rule_id}/lift",
		"/v1/containment-rules/{rule_id}/blast-radius",
		"/v1/integrations/github/repositories",
		"/v1/integrations/github/webhooks",
		"/v1/integrations/github/check-runs/plan",
		"/v1/integrations/okta/app-bindings",
		"/v1/integrations/okta/authority-context/resolve",
		"/v1/integrations/slack/workspace-bindings",
		"/v1/integrations/slack/message-actions/authorize",
		"/v1/events",
		"/v1/events/stream",
	}
	for _, path := range requiredPaths {
		if !strings.Contains(spec, "  "+path+":") {
			t.Fatalf("OpenAPI contract does not document %s", path)
		}
	}
}
