package mission

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSalesforceMissionAdapterRecordAuthorization(t *testing.T) {
	service := testService()
	mission := approveSalesforceMission(t, service)

	binding, err := service.CreateSalesforceOrgBinding(CreateSalesforceOrgBindingRequest{
		TenantID:               "demo",
		InstanceURL:            "https://acme.my.salesforce.com/",
		OrgID:                  "00Dxx0000001ABC",
		OrgName:                "Acme",
		MissionRef:             mission.MissionRef,
		AllowedObjectAPINames:  []string{"Account"},
		AllowedRecordTypeNames: []string{"Customer"},
		AllowedActions:         []string{SalesforceActionUpdateRecord},
		RequiredProfiles:       []string{"Standard User"},
		RequiredPermissionSets: []string{"CRM Agent"},
		AdminPermissionSets:    []string{"Mission Admin"},
		AllowedSubjects:        []string{"005xx000001"},
	}, Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("CreateSalesforceOrgBinding: %v", err)
	}
	if binding.InstanceURL != "https://acme.my.salesforce.com" || binding.MissionRef != mission.MissionRef {
		t.Fatalf("unexpected Salesforce binding: %#v", binding)
	}

	record, err := service.AuthorizeSalesforceRecordAction(AuthorizeSalesforceRecordActionRequest{
		TenantID:       "demo",
		MissionRef:     mission.MissionRef,
		InstanceURL:    "https://acme.my.salesforce.com",
		ObjectAPIName:  "Account",
		RecordID:       "001xx000003DGbY",
		RecordTypeName: "Customer",
		Action:         SalesforceActionUpdateRecord,
		UserID:         "005xx000001",
		Subject:        "005xx000001",
		Username:       "agent@example.com",
		Email:          "agent@example.com",
		Profile:        "Standard User",
		PermissionSets: []string{"CRM Agent", "Mission Admin"},
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
	if err != nil {
		t.Fatalf("AuthorizeSalesforceRecordAction: %v", err)
	}
	if !record.Accepted || !record.Admin || record.Evaluation == nil || record.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("record authorization = %#v, want accepted admin allow", record)
	}
	if record.Context["salesforce.binding_id"] != binding.BindingID || record.Context["salesforce.object_api_name"] != "Account" {
		t.Fatalf("record context = %#v, want binding and object context", record.Context)
	}

	denied, err := service.AuthorizeSalesforceRecordAction(AuthorizeSalesforceRecordActionRequest{
		TenantID:       "demo",
		MissionRef:     mission.MissionRef,
		InstanceURL:    "https://acme.my.salesforce.com",
		ObjectAPIName:  "Opportunity",
		RecordID:       "006xx000004TmiE",
		Action:         SalesforceActionUpdateRecord,
		UserID:         "005xx000001",
		Subject:        "005xx000001",
		Profile:        "Standard User",
		PermissionSets: []string{"CRM Agent"},
	})
	if err != nil {
		t.Fatalf("AuthorizeSalesforceRecordAction denied: %v", err)
	}
	if denied.Accepted || !contains(denied.ReasonCodes, "salesforce_object_not_allowed") {
		t.Fatalf("denied record authorization = %#v, want object denial", denied)
	}
}

func TestSalesforceMissionAdapterConversions(t *testing.T) {
	principal := salesforcePrincipal(Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"})
	if principal.Subject != "alice@example.com" || principal.Issuer != "https://idp.example.com" {
		t.Fatalf("unexpected principal conversion: %#v", principal)
	}

	actor := missionActorFromSalesforce(SalesforceActor{AgentInstanceID: "inst_123", ClientID: "agent", KeyThumbprint: "thumb"})
	if actor.AgentInstanceID != "inst_123" || actor.ClientID != "agent" || actor.KeyThumbprint != "thumb" {
		t.Fatalf("unexpected actor conversion: %#v", actor)
	}
}

func TestSignedSalesforceRecordAPIBindsAgentIdentity(t *testing.T) {
	service := testService()
	publicKey, privateKey := testAgentKeypair(t)
	registered, err := service.RegisterAgent(RegisterAgentRequest{
		TenantID:  "demo",
		Agent:     Agent{Provider: "https://agents.example.com", ClientID: "research-agent", InstanceID: "inst_123"},
		PublicKey: publicKey,
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	mission := approveSalesforceMission(t, service)
	if _, err := service.CreateSalesforceOrgBinding(CreateSalesforceOrgBindingRequest{
		TenantID:               "demo",
		InstanceURL:            "https://acme.my.salesforce.com",
		MissionRef:             mission.MissionRef,
		AllowedObjectAPINames:  []string{"Account"},
		AllowedRecordTypeNames: []string{"Customer"},
		AllowedActions:         []string{SalesforceActionUpdateRecord},
		RequiredProfiles:       []string{"Standard User"},
		RequiredPermissionSets: []string{"CRM Agent"},
		AllowedSubjects:        []string{"005xx000001"},
	}, Principal{Subject: "admin@example.com", Issuer: "https://idp.example.com"}); err != nil {
		t.Fatalf("CreateSalesforceOrgBinding: %v", err)
	}

	body := AuthorizeSalesforceRecordActionRequest{
		TenantID:       "demo",
		MissionRef:     mission.MissionRef,
		InstanceURL:    "https://acme.my.salesforce.com",
		ObjectAPIName:  "Account",
		RecordID:       "001xx000003DGbY",
		RecordTypeName: "Customer",
		Action:         SalesforceActionUpdateRecord,
		UserID:         "005xx000001",
		Subject:        "005xx000001",
		Username:       "agent@example.com",
		Email:          "agent@example.com",
		Profile:        "Standard User",
		PermissionSets: []string{"CRM Agent"},
		Context:        map[string]any{"finance.close.status": "open", "risk": "low", "reversible": true},
		Evaluation: &SalesforceEvaluationRequest{
			MissionVersionSeen: mission.MissionVersion,
			Action: SalesforceEvaluationAction{
				Type:      "salesforce_record",
				Resource:  SalesforceEvaluationActionResource{Type: "salesforce_record", ID: "Account:001xx000003DGbY"},
				Operation: "update",
			},
		},
	}
	router := NewHandler(service).Routes()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, signedJSONRequest(t, http.MethodPost, "/v1/integrations/salesforce/records/authorize", body, registered.AgentID, privateKey, "nonce-salesforce-record"))
	if rec.Code != http.StatusOK {
		t.Fatalf("signed Salesforce authorize status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp SalesforceRecordActionAuthorizationResponse
	decodeTestJSON(t, rec.Body.Bytes(), &resp)
	if !resp.Accepted || resp.Evaluation == nil || resp.Evaluation.Decision != string(DecisionAllow) {
		t.Fatalf("signed Salesforce response = %#v, want accepted allow with evaluation", resp)
	}
}

func approveSalesforceMission(t *testing.T, service *Service) ApproveProposalResponse {
	t.Helper()
	req := validProposalRequest()
	req.Intent = Purpose{Objective: "Govern Salesforce CRM record work"}
	req.AuthorityRegion = AuthorityRegion{
		Resources: []ResourceGrant{
			{Type: "salesforce_record", ID: "Account:001xx000003DGbY", Actions: []string{"update"}},
		},
		ForbiddenActions: []string{"delete"},
	}
	req.Conditions = nil
	proposal, err := service.CreateProposal(req)
	if err != nil {
		t.Fatalf("CreateProposal: %v", err)
	}
	mission, err := service.ApproveProposal(proposal.ProposalID, ApproveProposalRequest{
		Approver: Principal{Subject: "alice@example.com", Issuer: "https://idp.example.com"},
	})
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}
	return mission
}
