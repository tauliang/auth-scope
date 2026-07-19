package mission

import "testing"

func TestSalesforceHTTPIntegrationFlow(t *testing.T) {
	service := testService()
	router := NewHandler(service).Routes()
	mission := approveSalesforceMission(t, service)

	binding := postJSON[SalesforceOrgBinding](t, router, "/v1/integrations/salesforce/org-bindings", CreateSalesforceOrgBindingRequest{
		TenantID:               "demo",
		InstanceURL:            "https://acme.my.salesforce.com/",
		OrgID:                  "00Dxx0000001ABC",
		OrgName:                "Acme",
		MissionRef:             mission.MissionRef,
		AllowedObjectAPINames:  []string{"Account"},
		AllowedRecordTypeNames: []string{"Customer"},
		AllowedActions:         []string{SalesforceActionUpdateRecord},
		RequiredPermissionSets: []string{"CRM Agent"},
	})
	if binding.InstanceURL != "https://acme.my.salesforce.com" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected Salesforce binding: %#v", binding)
	}

	listed := getJSON[struct {
		OrgBindings []SalesforceOrgBinding `json:"org_bindings"`
	}](t, router, "/v1/integrations/salesforce/org-bindings")
	if len(listed.OrgBindings) != 1 || listed.OrgBindings[0].BindingID != binding.BindingID {
		t.Fatalf("listed Salesforce bindings = %#v, want binding %s", listed.OrgBindings, binding.BindingID)
	}

	record := postJSON[SalesforceRecordActionAuthorizationResponse](t, router, "/v1/integrations/salesforce/records/authorize", AuthorizeSalesforceRecordActionRequest{
		TenantID:       "demo",
		InstanceURL:    "https://acme.my.salesforce.com",
		OrgID:          "00Dxx0000001ABC",
		ObjectAPIName:  "Account",
		RecordID:       "001xx000003DGbY",
		RecordTypeName: "Customer",
		Action:         SalesforceActionUpdateRecord,
		UserID:         "005xx000001",
		Profile:        "Standard User",
		PermissionSets: []string{"CRM Agent"},
		Context:        map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &SalesforceEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Actor:              SalesforceActor{AgentInstanceID: "inst_123", ClientID: "research-agent"},
			Action: SalesforceEvaluationAction{
				Type:      "salesforce_record",
				Resource:  SalesforceEvaluationActionResource{Type: "salesforce_record", ID: "Account:001xx000003DGbY"},
				Operation: "update",
			},
		},
	})
	if !record.Accepted || record.Evaluation == nil || record.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("record authorization = %#v, want accepted allow", record)
	}

	denied := postJSON[SalesforceRecordActionAuthorizationResponse](t, router, "/v1/integrations/salesforce/records/authorize", AuthorizeSalesforceRecordActionRequest{
		TenantID:       "demo",
		InstanceURL:    "https://acme.my.salesforce.com",
		OrgID:          "00Dxx0000001ABC",
		ObjectAPIName:  "Opportunity",
		RecordID:       "006xx000004TmiE",
		Action:         SalesforceActionUpdateRecord,
		UserID:         "005xx000001",
		PermissionSets: []string{"CRM Agent"},
	})
	if denied.Accepted || !contains(denied.ReasonCodes, "salesforce_object_not_allowed") {
		t.Fatalf("denied Salesforce response = %#v, want object denial", denied)
	}
}
