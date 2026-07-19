package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

type Clock = contract.Clock

type Store interface {
	SaveRepositoryBinding(RepositoryBinding) error
	GetRepositoryBinding(string) (RepositoryBinding, error)
	UpdateRepositoryBinding(RepositoryBinding) error
	ListRepositoryBindings() ([]RepositoryBinding, error)
	SaveWebhookDelivery(WebhookDelivery) error
	GetWebhookDelivery(string) (WebhookDelivery, error)
}

type Evaluator = contract.Evaluator

type EventSink = contract.EventSink
type EvaluationActionResource = contract.EvaluationActionResource
type EvaluationAction = contract.EvaluationAction
type EvaluationRequest = contract.EvaluationRequest
type EvaluationResponse = contract.EvaluationResponse
type Event = contract.Event

type Config struct {
	Store         Store
	Evaluator     Evaluator
	Events        EventSink
	Clock         Clock
	WebhookSecret []byte
	NewID         func(string) string
	IsConflict    func(error) bool
}

type Service struct {
	store         Store
	evaluator     Evaluator
	events        EventSink
	clock         Clock
	webhookSecret []byte
	newID         func(string) string
	isConflict    func(error) bool
}

func NewService(config Config) *Service {
	isConflict := config.IsConflict
	if isConflict == nil {
		isConflict = func(error) bool { return false }
	}
	return &Service{
		store:         config.Store,
		evaluator:     config.Evaluator,
		events:        config.Events,
		clock:         config.Clock,
		webhookSecret: config.WebhookSecret,
		newID:         config.NewID,
		isConflict:    isConflict,
	}
}

func (s *Service) CreateRepositoryBinding(req CreateRepositoryBindingRequest, actor Principal) (RepositoryBinding, error) {
	owner, repo, repository, err := NormalizeRepository(req.Owner, req.Repo, req.Repository)
	if err != nil {
		return RepositoryBinding{}, err
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	now := s.now()
	binding := RepositoryBinding{
		BindingID:      s.id("ghr"),
		TenantID:       strings.TrimSpace(req.TenantID),
		Owner:          owner,
		Repo:           repo,
		Repository:     repository,
		DefaultBranch:  strings.TrimSpace(req.DefaultBranch),
		InstallationID: req.InstallationID,
		MissionRef:     strings.TrimSpace(req.MissionRef),
		BranchPatterns: CleanStringList(req.BranchPatterns),
		RequiredChecks: CleanStringList(req.RequiredChecks),
		Status:         RepositoryBindingStatusActive,
		Metadata:       CloneStringMap(req.Metadata),
		CreatedBy:      actor,
		CreatedAt:      now,
	}
	if binding.DefaultBranch == "" {
		binding.DefaultBranch = "main"
	}
	if len(binding.BranchPatterns) == 0 {
		binding.BranchPatterns = []string{binding.DefaultBranch, "refs/heads/" + binding.DefaultBranch}
	}
	if err := s.store.SaveRepositoryBinding(binding); err != nil {
		return RepositoryBinding{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		TenantID:   binding.TenantID,
		Type:       "github.repository_bound",
		Actor:      map[string]any{"subject": actor.Subject, "issuer": actor.Issuer},
		Payload:    map[string]any{"binding_id": binding.BindingID, "repository": binding.Repository, "mission_ref": binding.MissionRef},
		OccurredAt: now,
	})
	return binding, nil
}

func (s *Service) ListRepositoryBindings() ([]RepositoryBinding, error) {
	return s.store.ListRepositoryBindings()
}

func (s *Service) IngestWebhook(req IngestWebhookRequest) (WebhookResponse, error) {
	if strings.TrimSpace(req.Event) == "" {
		return WebhookResponse{}, fmt.Errorf("X-GitHub-Event is required")
	}
	if strings.TrimSpace(req.DeliveryID) == "" {
		return WebhookResponse{}, fmt.Errorf("X-GitHub-Delivery is required")
	}
	if len(s.webhookSecret) == 0 {
		return WebhookResponse{}, fmt.Errorf("github webhook secret is not configured")
	}
	if !ValidateWebhookSignature(s.webhookSecret, req.Signature, req.Body) {
		return WebhookResponse{}, fmt.Errorf("github webhook signature is invalid")
	}

	var payload map[string]any
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return WebhookResponse{}, fmt.Errorf("decode github webhook payload: %w", err)
	}
	now := s.now()
	summary := SummarizeWebhook(req.Event, payload)
	delivery := WebhookDelivery{
		DeliveryID:     strings.TrimSpace(req.DeliveryID),
		Event:          strings.TrimSpace(req.Event),
		Action:         summary.Action,
		Repository:     summary.Repository,
		Ref:            summary.Ref,
		SHA:            summary.SHA,
		PullRequest:    summary.PullRequest,
		Branch:         summary.Branch,
		Status:         WebhookDeliveryStatusIgnored,
		ReceivedAt:     now,
		PayloadSummary: summary.PayloadSummary,
	}
	if binding, ok, err := s.bindingForRepository(summary.Repository); err != nil {
		return WebhookResponse{}, err
	} else if ok {
		delivery.BindingID = binding.BindingID
		delivery.TenantID = binding.TenantID
		delivery.MissionRef = binding.MissionRef
		delivery.Status = WebhookDeliveryStatusAccepted
		binding.LastWebhookAt = now
		binding.LastDeliveryID = delivery.DeliveryID
		if summary.SHA != "" {
			binding.LastCheckSHA = summary.SHA
		}
		_ = s.store.UpdateRepositoryBinding(binding)
	}

	if err := s.store.SaveWebhookDelivery(delivery); err != nil {
		if s.isConflict(err) {
			existing, getErr := s.store.GetWebhookDelivery(delivery.DeliveryID)
			if getErr != nil {
				return WebhookResponse{}, getErr
			}
			existing.Status = WebhookDeliveryStatusDuplicate
			return WebhookResponseFor(existing, "delivery already processed"), nil
		}
		return WebhookResponse{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: delivery.MissionRef,
		TenantID:   delivery.TenantID,
		Type:       "github.webhook_received",
		Payload: map[string]any{
			"delivery_id": delivery.DeliveryID,
			"event":       delivery.Event,
			"repository":  delivery.Repository,
			"action":      delivery.Action,
			"ref":         delivery.Ref,
			"sha":         delivery.SHA,
			"status":      delivery.Status,
			"binding_id":  delivery.BindingID,
		},
		OccurredAt: now,
	})
	return WebhookResponseFor(delivery, ""), nil
}

func (s *Service) PlanCheckRun(req CheckRunPlanRequest) (CheckRunPlanResponse, error) {
	_, _, repository, err := NormalizeRepository("", "", req.Repository)
	if err != nil {
		return CheckRunPlanResponse{}, err
	}
	if strings.TrimSpace(req.HeadSHA) == "" {
		return CheckRunPlanResponse{}, fmt.Errorf("head_sha is required")
	}
	if len(req.ChangedFiles) == 0 {
		return CheckRunPlanResponse{}, fmt.Errorf("changed_files is required")
	}
	binding, ok, err := s.bindingForRepository(repository)
	if err != nil {
		return CheckRunPlanResponse{}, err
	}
	if !ok {
		return CheckRunPlanResponse{}, fmt.Errorf("github repository %q is not bound", repository)
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		missionRef = binding.MissionRef
	}
	if missionRef == "" {
		return CheckRunPlanResponse{}, fmt.Errorf("mission_ref is required")
	}
	if binding.MissionRef != "" && missionRef != binding.MissionRef {
		return CheckRunPlanResponse{}, fmt.Errorf("mission_ref does not match github repository binding")
	}

	evaluations := make([]CheckEvaluation, 0, len(req.ChangedFiles))
	conclusion := CheckConclusionSuccess
	var missionVersion int
	for _, file := range req.ChangedFiles {
		filePath := CleanRepoPath(file.Path)
		if filePath == "" {
			return CheckRunPlanResponse{}, fmt.Errorf("changed_files.path is required")
		}
		operation := FileOperation(file)
		resourceID := RepoPathResource(repository, filePath)
		decision, err := s.evaluator.Evaluate(EvaluationRequest{
			MissionRef:         missionRef,
			MissionVersionSeen: req.MissionVersionSeen,
			Actor:              req.Actor,
			Action: EvaluationAction{
				Type:      "github_pull_request",
				Name:      "github.file_change",
				Resource:  EvaluationActionResource{Type: "repo_path", ID: resourceID},
				Operation: operation,
			},
			Context: MergeCheckContext(req, file, repository),
		})
		if err != nil {
			return CheckRunPlanResponse{}, err
		}
		missionVersion = decision.MissionVersion
		evaluations = append(evaluations, CheckEvaluation{
			Path:             filePath,
			ResourceID:       resourceID,
			Operation:        operation,
			Decision:         decision.Decision,
			ReasonCodes:      decision.ReasonCodes,
			HumanReason:      decision.HumanReason,
			DecisionArtifact: decision.DecisionArtifact,
			Constraints:      decision.Constraints,
		})
		conclusion = CombineConclusion(conclusion, ConclusionForDecision(decision.Decision))
	}

	status := "completed"
	title := "Mission authority satisfied"
	summary := "All changed files are inside the active mission authority region."
	switch conclusion {
	case CheckConclusionFailure:
		title = "Mission authority blocked this change"
		summary = "One or more changed files or operations were denied by mission authority."
	case CheckConclusionActionRequired:
		title = "Mission authority requires approval"
		summary = "One or more changed files need mission expansion or human approval before merge."
	case CheckConclusionNeutral:
		title = "Mission authority needs refresh"
		summary = "The check could not produce a final allow/deny decision."
	}
	now := s.now()
	response := CheckRunPlanResponse{
		Name:           "Auth Scope Mission Authority",
		ExternalID:     missionRef + ":" + strings.TrimSpace(req.HeadSHA),
		Repository:     repository,
		HeadSHA:        strings.TrimSpace(req.HeadSHA),
		Status:         status,
		Conclusion:     conclusion,
		Title:          title,
		Summary:        summary,
		Text:           CheckText(evaluations),
		MissionRef:     missionRef,
		MissionVersion: missionVersion,
		BindingID:      binding.BindingID,
		Evaluations:    evaluations,
	}
	binding.LastCheckSHA = response.HeadSHA
	binding.LastCheckStatus = response.Conclusion
	_ = s.store.UpdateRepositoryBinding(binding)
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: missionRef,
		TenantID:   binding.TenantID,
		Type:       "github.check_run_planned",
		Actor:      map[string]any{"agent_instance_id": req.Actor.AgentInstanceID, "client_id": req.Actor.ClientID, "key_thumbprint": req.Actor.KeyThumbprint},
		Payload: map[string]any{
			"binding_id":  binding.BindingID,
			"repository":  repository,
			"head_sha":    response.HeadSHA,
			"conclusion":  response.Conclusion,
			"evaluations": len(evaluations),
		},
		VersionBefore: missionVersion,
		VersionAfter:  missionVersion,
		OccurredAt:    now,
	})
	return response, nil
}

func (s *Service) bindingForRepository(repository string) (RepositoryBinding, bool, error) {
	repository = NormalizeRepositoryName(repository)
	if repository == "" {
		return RepositoryBinding{}, false, nil
	}
	bindings, err := s.store.ListRepositoryBindings()
	if err != nil {
		return RepositoryBinding{}, false, err
	}
	for _, binding := range bindings {
		if binding.Status == RepositoryBindingStatusActive && NormalizeRepositoryName(binding.Repository) == repository {
			return binding, true, nil
		}
	}
	return RepositoryBinding{}, false, nil
}

func (s *Service) appendEvent(event Event) {
	contract.AppendEvent(s.events, event)
}

func (s *Service) now() time.Time {
	return contract.Now(s.clock)
}

func (s *Service) id(prefix string) string {
	return contract.NewID(s.newID, prefix)
}

func IsConflict(conflict error) func(error) bool {
	return func(err error) bool {
		return errors.Is(err, conflict)
	}
}
