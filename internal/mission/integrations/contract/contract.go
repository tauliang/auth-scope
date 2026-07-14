package contract

import "time"

const (
	BindingStatusActive   = "active"
	BindingStatusDisabled = "disabled"

	MatchAny = "any"
	MatchAll = "all"

	ResolutionStatusAccepted = "accepted"
	ResolutionStatusDenied   = "denied"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator func(string) string

type ConflictClassifier func(error) bool

type Principal struct {
	Subject string `json:"subject"`
	Issuer  string `json:"issuer"`
}

type Actor struct {
	AgentInstanceID string `json:"agent_instance_id"`
	ClientID        string `json:"client_id"`
	KeyThumbprint   string `json:"key_thumbprint,omitempty"`
}

type EvaluationActionResource struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ChannelID string `json:"channel_id,omitempty"`
}

type EvaluationAction struct {
	Type      string                   `json:"type"`
	Name      string                   `json:"name,omitempty"`
	Resource  EvaluationActionResource `json:"resource"`
	Operation string                   `json:"operation"`
}

type EvaluationRequest struct {
	MissionRef         string           `json:"mission_ref,omitempty"`
	MissionVersionSeen int              `json:"mission_version_seen,omitempty"`
	Actor              Actor            `json:"actor"`
	Action             EvaluationAction `json:"action"`
	Context            map[string]any   `json:"context,omitempty"`
}

type EvaluationResponse struct {
	Decision         string         `json:"decision"`
	MissionRef       string         `json:"mission_ref,omitempty"`
	MissionVersion   int            `json:"mission_version,omitempty"`
	ReasonCodes      []string       `json:"reason_codes,omitempty"`
	HumanReason      string         `json:"human_reason,omitempty"`
	DecisionArtifact string         `json:"decision_artifact,omitempty"`
	Constraints      map[string]any `json:"constraints,omitempty"`
}

type Evaluator interface {
	Evaluate(EvaluationRequest) (EvaluationResponse, error)
}

type Event struct {
	EventID       string
	MissionRef    string
	TenantID      string
	Type          string
	Actor         map[string]any
	Payload       map[string]any
	VersionBefore int
	VersionAfter  int
	OccurredAt    time.Time
}

type EventSink interface {
	AppendEvent(Event) error
}

func Now(clock Clock) time.Time {
	if clock == nil {
		return time.Now().UTC()
	}
	return clock.Now()
}

func NewID(generate IDGenerator, prefix string) string {
	if generate == nil {
		return prefix
	}
	return generate(prefix)
}

func IsConflict(classifier ConflictClassifier, err error) bool {
	if classifier == nil {
		return false
	}
	return classifier(err)
}

func AppendEvent(events EventSink, event Event) {
	if events != nil {
		_ = events.AppendEvent(event)
	}
}
