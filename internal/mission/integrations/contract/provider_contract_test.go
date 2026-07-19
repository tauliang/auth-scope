package contract_test

import (
	"testing"
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
	"github.com/tauliang/auth-scope/internal/mission/integrations/entra"
	"github.com/tauliang/auth-scope/internal/mission/integrations/github"
	"github.com/tauliang/auth-scope/internal/mission/integrations/okta"
	"github.com/tauliang/auth-scope/internal/mission/integrations/servicenow"
	"github.com/tauliang/auth-scope/internal/mission/integrations/slack"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type evaluator struct {
	requests []contract.EvaluationRequest
}

func (e *evaluator) Evaluate(req contract.EvaluationRequest) (contract.EvaluationResponse, error) {
	e.requests = append(e.requests, req)
	return contract.EvaluationResponse{
		Decision:       "allow",
		MissionRef:     req.MissionRef,
		MissionVersion: 1,
		ReasonCodes:    []string{"contract_test"},
	}, nil
}

type eventSink struct {
	events []contract.Event
}

func (s *eventSink) AppendEvent(event contract.Event) error {
	s.events = append(s.events, event)
	return nil
}

func TestProvidersAcceptSharedMissionAuthorityContract(t *testing.T) {
	clock := fixedClock{now: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)}
	evaluator := &evaluator{}
	events := &eventSink{}
	newID := func(prefix string) string { return prefix + "_contract" }

	_ = github.Config{Evaluator: evaluator, Events: events, Clock: clock, NewID: newID}
	_ = okta.Config{Evaluator: evaluator, Events: events, Clock: clock, NewID: newID}
	_ = entra.Config{Evaluator: evaluator, Events: events, Clock: clock, NewID: newID}
	_ = slack.Config{Evaluator: evaluator, Events: events, Clock: clock, NewID: newID}
	_ = servicenow.Config{Evaluator: evaluator, Events: events, Clock: clock, NewID: newID}
}

func TestProviderCommonTypesRoundTripThroughSharedContract(t *testing.T) {
	req := contract.EvaluationRequest{
		MissionRef:         "mref_contract",
		MissionVersionSeen: 3,
		Actor: contract.Actor{
			AgentInstanceID: "inst_contract",
			ClientID:        "agent_contract",
			KeyThumbprint:   "thumb_contract",
		},
		Action: contract.EvaluationAction{
			Type:      "provider_action",
			Name:      "integration.contract",
			Resource:  contract.EvaluationActionResource{Type: "resource", ID: "res_123", ChannelID: "C123"},
			Operation: "read",
		},
		Context: map[string]any{"risk": "low"},
	}

	providerRequests := []contract.EvaluationRequest{
		github.EvaluationRequest(req),
		okta.EvaluationRequest(req),
		entra.EvaluationRequest(req),
		slack.EvaluationRequest(req),
		servicenow.EvaluationRequest(req),
	}
	for _, providerReq := range providerRequests {
		if providerReq.MissionRef != req.MissionRef || providerReq.Action.Resource.ChannelID != "C123" {
			t.Fatalf("provider evaluation request drifted from shared contract: %#v", providerReq)
		}
	}

	event := contract.Event{
		EventID:    "evt_contract",
		MissionRef: req.MissionRef,
		TenantID:   "tenant_contract",
		Type:       "integration.contract_checked",
		Actor:      map[string]any{"agent_instance_id": req.Actor.AgentInstanceID},
		Payload:    map[string]any{"resource": req.Action.Resource.ID},
		OccurredAt: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
	}
	providerEvents := []contract.Event{
		github.Event(event),
		okta.Event(event),
		entra.Event(event),
		slack.Event(event),
		servicenow.Event(event),
	}
	for _, providerEvent := range providerEvents {
		if providerEvent.EventID != event.EventID || providerEvent.Payload["resource"] != "res_123" {
			t.Fatalf("provider event drifted from shared contract: %#v", providerEvent)
		}
	}
}

func TestContractHelpersDefaultSafely(t *testing.T) {
	now := contract.Now(nil)
	if now.IsZero() {
		t.Fatal("nil clock returned zero time")
	}
	if got := contract.NewID(nil, "evt"); got != "evt" {
		t.Fatalf("nil id generator = %q, want prefix fallback", got)
	}

	events := &eventSink{}
	contract.AppendEvent(events, contract.Event{EventID: "evt_1"})
	if len(events.events) != 1 || events.events[0].EventID != "evt_1" {
		t.Fatalf("AppendEvent did not forward event: %#v", events.events)
	}
	contract.AppendEvent(nil, contract.Event{EventID: "evt_ignored"})
}
