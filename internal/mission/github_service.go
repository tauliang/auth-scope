package mission

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) CreateGitHubRepositoryBinding(req CreateGitHubRepositoryBindingRequest, actor Principal) (GitHubRepositoryBinding, error) {
	owner, repo, repository, err := normalizeGitHubRepository(req.Owner, req.Repo, req.Repository)
	if err != nil {
		return GitHubRepositoryBinding{}, err
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	now := s.clock.Now()
	binding := GitHubRepositoryBinding{
		BindingID:      newID("ghr"),
		TenantID:       strings.TrimSpace(req.TenantID),
		Owner:          owner,
		Repo:           repo,
		Repository:     repository,
		DefaultBranch:  strings.TrimSpace(req.DefaultBranch),
		InstallationID: req.InstallationID,
		MissionRef:     strings.TrimSpace(req.MissionRef),
		BranchPatterns: cleanStringList(req.BranchPatterns),
		RequiredChecks: cleanStringList(req.RequiredChecks),
		Status:         GitHubRepositoryBindingStatusActive,
		Metadata:       cloneStringMap(req.Metadata),
		CreatedBy:      actor,
		CreatedAt:      now,
	}
	if binding.DefaultBranch == "" {
		binding.DefaultBranch = "main"
	}
	if len(binding.BranchPatterns) == 0 {
		binding.BranchPatterns = []string{binding.DefaultBranch, "refs/heads/" + binding.DefaultBranch}
	}
	if err := s.github.SaveGitHubRepositoryBinding(binding); err != nil {
		return GitHubRepositoryBinding{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
		TenantID:   binding.TenantID,
		Type:       "github.repository_bound",
		Actor:      map[string]any{"subject": actor.Subject, "issuer": actor.Issuer},
		Payload:    map[string]any{"binding_id": binding.BindingID, "repository": binding.Repository, "mission_ref": binding.MissionRef},
		OccurredAt: now,
	})
	return binding, nil
}

func (s *Service) ListGitHubRepositoryBindings() ([]GitHubRepositoryBinding, error) {
	return s.github.ListGitHubRepositoryBindings()
}

func (s *Service) IngestGitHubWebhook(req IngestGitHubWebhookRequest) (GitHubWebhookResponse, error) {
	if strings.TrimSpace(req.Event) == "" {
		return GitHubWebhookResponse{}, fmt.Errorf("X-GitHub-Event is required")
	}
	if strings.TrimSpace(req.DeliveryID) == "" {
		return GitHubWebhookResponse{}, fmt.Errorf("X-GitHub-Delivery is required")
	}
	if len(s.githubWebhookSecret) == 0 {
		return GitHubWebhookResponse{}, fmt.Errorf("github webhook secret is not configured")
	}
	if !ValidateGitHubWebhookSignature(s.githubWebhookSecret, req.Signature, req.Body) {
		return GitHubWebhookResponse{}, fmt.Errorf("github webhook signature is invalid")
	}

	var payload map[string]any
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return GitHubWebhookResponse{}, fmt.Errorf("decode github webhook payload: %w", err)
	}
	now := s.clock.Now()
	summary := summarizeGitHubWebhook(req.Event, payload)
	delivery := GitHubWebhookDelivery{
		DeliveryID:     strings.TrimSpace(req.DeliveryID),
		Event:          strings.TrimSpace(req.Event),
		Action:         summary.Action,
		Repository:     summary.Repository,
		Ref:            summary.Ref,
		SHA:            summary.SHA,
		PullRequest:    summary.PullRequest,
		Branch:         summary.Branch,
		Status:         GitHubWebhookDeliveryStatusIgnored,
		ReceivedAt:     now,
		PayloadSummary: summary.PayloadSummary,
	}
	if binding, ok, err := s.githubBindingForRepository(summary.Repository); err != nil {
		return GitHubWebhookResponse{}, err
	} else if ok {
		delivery.BindingID = binding.BindingID
		delivery.TenantID = binding.TenantID
		delivery.MissionRef = binding.MissionRef
		delivery.Status = GitHubWebhookDeliveryStatusAccepted
		binding.LastWebhookAt = now
		binding.LastDeliveryID = delivery.DeliveryID
		if summary.SHA != "" {
			binding.LastCheckSHA = summary.SHA
		}
		_ = s.github.UpdateGitHubRepositoryBinding(binding)
	}

	if err := s.github.SaveGitHubWebhookDelivery(delivery); err != nil {
		if errors.Is(err, ErrConflict) {
			existing, getErr := s.github.GetGitHubWebhookDelivery(delivery.DeliveryID)
			if getErr != nil {
				return GitHubWebhookResponse{}, getErr
			}
			existing.Status = GitHubWebhookDeliveryStatusDuplicate
			return githubWebhookResponse(existing, "delivery already processed"), nil
		}
		return GitHubWebhookResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
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
	return githubWebhookResponse(delivery, ""), nil
}

func (s *Service) PlanGitHubCheckRun(req GitHubCheckRunPlanRequest) (GitHubCheckRunPlanResponse, error) {
	_, _, repository, err := normalizeGitHubRepository("", "", req.Repository)
	if err != nil {
		return GitHubCheckRunPlanResponse{}, err
	}
	if strings.TrimSpace(req.HeadSHA) == "" {
		return GitHubCheckRunPlanResponse{}, fmt.Errorf("head_sha is required")
	}
	if len(req.ChangedFiles) == 0 {
		return GitHubCheckRunPlanResponse{}, fmt.Errorf("changed_files is required")
	}
	binding, ok, err := s.githubBindingForRepository(repository)
	if err != nil {
		return GitHubCheckRunPlanResponse{}, err
	}
	if !ok {
		return GitHubCheckRunPlanResponse{}, fmt.Errorf("github repository %q is not bound", repository)
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		missionRef = binding.MissionRef
	}
	if missionRef == "" {
		return GitHubCheckRunPlanResponse{}, fmt.Errorf("mission_ref is required")
	}
	if binding.MissionRef != "" && missionRef != binding.MissionRef {
		return GitHubCheckRunPlanResponse{}, fmt.Errorf("mission_ref does not match github repository binding")
	}

	evaluations := make([]GitHubCheckEvaluation, 0, len(req.ChangedFiles))
	conclusion := GitHubCheckConclusionSuccess
	var missionVersion int
	for _, file := range req.ChangedFiles {
		filePath := cleanRepoPath(file.Path)
		if filePath == "" {
			return GitHubCheckRunPlanResponse{}, fmt.Errorf("changed_files.path is required")
		}
		operation := githubFileOperation(file)
		context := mergeGitHubCheckContext(req, file, repository)
		decision, err := s.Evaluate(missionRef, EvaluateRequest{
			MissionVersionSeen: req.MissionVersionSeen,
			Actor:              req.Actor,
			Action: Action{
				Type:      "github_pull_request",
				Name:      "github.file_change",
				Resource:  ActionResource{Type: "repo_path", ID: githubRepoPathResource(repository, filePath)},
				Operation: operation,
			},
			Context: context,
		})
		if err != nil {
			return GitHubCheckRunPlanResponse{}, err
		}
		missionVersion = decision.MissionVersion
		evaluations = append(evaluations, GitHubCheckEvaluation{
			Path:             filePath,
			ResourceID:       githubRepoPathResource(repository, filePath),
			Operation:        operation,
			Decision:         decision.Decision,
			ReasonCodes:      decision.ReasonCodes,
			HumanReason:      decision.HumanReason,
			DecisionArtifact: decision.DecisionArtifact,
			Constraints:      decision.Constraints,
		})
		conclusion = combineGitHubConclusion(conclusion, conclusionForDecision(decision.Decision))
	}

	status := "completed"
	title := "Mission authority satisfied"
	summary := "All changed files are inside the active mission authority region."
	switch conclusion {
	case GitHubCheckConclusionFailure:
		title = "Mission authority blocked this change"
		summary = "One or more changed files or operations were denied by mission authority."
	case GitHubCheckConclusionActionRequired:
		title = "Mission authority requires approval"
		summary = "One or more changed files need mission expansion or human approval before merge."
	case GitHubCheckConclusionNeutral:
		title = "Mission authority needs refresh"
		summary = "The check could not produce a final allow/deny decision."
	}
	now := s.clock.Now()
	response := GitHubCheckRunPlanResponse{
		Name:           "Auth Scope Mission Authority",
		ExternalID:     missionRef + ":" + strings.TrimSpace(req.HeadSHA),
		Repository:     repository,
		HeadSHA:        strings.TrimSpace(req.HeadSHA),
		Status:         status,
		Conclusion:     conclusion,
		Title:          title,
		Summary:        summary,
		Text:           githubCheckText(evaluations),
		MissionRef:     missionRef,
		MissionVersion: missionVersion,
		BindingID:      binding.BindingID,
		Evaluations:    evaluations,
	}
	binding.LastCheckSHA = response.HeadSHA
	binding.LastCheckStatus = response.Conclusion
	_ = s.github.UpdateGitHubRepositoryBinding(binding)
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
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

func ValidateGitHubWebhookSignature(secret []byte, signatureHeader string, body []byte) bool {
	if len(secret) == 0 {
		return false
	}
	signatureHeader = strings.TrimSpace(signatureHeader)
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(provided, expected)
}

func SignGitHubWebhookBody(secret []byte, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (s *Service) githubBindingForRepository(repository string) (GitHubRepositoryBinding, bool, error) {
	repository = normalizeRepositoryName(repository)
	if repository == "" {
		return GitHubRepositoryBinding{}, false, nil
	}
	bindings, err := s.github.ListGitHubRepositoryBindings()
	if err != nil {
		return GitHubRepositoryBinding{}, false, err
	}
	for _, binding := range bindings {
		if binding.Status == GitHubRepositoryBindingStatusActive && normalizeRepositoryName(binding.Repository) == repository {
			return binding, true, nil
		}
	}
	return GitHubRepositoryBinding{}, false, nil
}

type githubWebhookSummary struct {
	Repository     string
	Action         string
	Ref            string
	SHA            string
	PullRequest    int
	Branch         string
	PayloadSummary map[string]any
}

func summarizeGitHubWebhook(event string, payload map[string]any) githubWebhookSummary {
	summary := githubWebhookSummary{
		Action:         strings.TrimSpace(fmt.Sprint(payload["action"])),
		Repository:     repositoryFromPayload(payload),
		Ref:            strings.TrimSpace(fmt.Sprint(payload["ref"])),
		SHA:            strings.TrimSpace(fmt.Sprint(payload["after"])),
		PayloadSummary: map[string]any{},
	}
	if summary.Action == "<nil>" {
		summary.Action = ""
	}
	if summary.Ref == "<nil>" {
		summary.Ref = ""
	}
	if summary.SHA == "<nil>" {
		summary.SHA = ""
	}
	if pr, ok := payload["pull_request"].(map[string]any); ok {
		summary.PullRequest = intFromAny(pr["number"])
		if summary.PullRequest == 0 {
			summary.PullRequest = intFromAny(payload["number"])
		}
		if head, ok := pr["head"].(map[string]any); ok {
			if sha := strings.TrimSpace(fmt.Sprint(head["sha"])); sha != "" && sha != "<nil>" {
				summary.SHA = sha
			}
			if branch := strings.TrimSpace(fmt.Sprint(head["ref"])); branch != "" && branch != "<nil>" {
				summary.Branch = branch
			}
		}
	}
	if summary.Branch == "" {
		summary.Branch = branchFromRef(summary.Ref)
	}
	summary.PayloadSummary["event"] = event
	summary.PayloadSummary["repository"] = summary.Repository
	summary.PayloadSummary["action"] = summary.Action
	summary.PayloadSummary["ref"] = summary.Ref
	summary.PayloadSummary["sha"] = summary.SHA
	if summary.PullRequest > 0 {
		summary.PayloadSummary["pull_request"] = summary.PullRequest
	}
	return summary
}

func repositoryFromPayload(payload map[string]any) string {
	if repo, ok := payload["repository"].(map[string]any); ok {
		if fullName := strings.TrimSpace(fmt.Sprint(repo["full_name"])); fullName != "" && fullName != "<nil>" {
			return normalizeRepositoryName(fullName)
		}
		owner := ""
		if ownerMap, ok := repo["owner"].(map[string]any); ok {
			owner = strings.TrimSpace(fmt.Sprint(ownerMap["login"]))
		}
		name := strings.TrimSpace(fmt.Sprint(repo["name"]))
		if owner != "" && owner != "<nil>" && name != "" && name != "<nil>" {
			return normalizeRepositoryName(owner + "/" + name)
		}
	}
	return ""
}

func githubWebhookResponse(delivery GitHubWebhookDelivery, message string) GitHubWebhookResponse {
	return GitHubWebhookResponse{
		Accepted:   delivery.Status == GitHubWebhookDeliveryStatusAccepted,
		Status:     delivery.Status,
		Event:      delivery.Event,
		DeliveryID: delivery.DeliveryID,
		Repository: delivery.Repository,
		Action:     delivery.Action,
		Ref:        delivery.Ref,
		SHA:        delivery.SHA,
		BindingID:  delivery.BindingID,
		MissionRef: delivery.MissionRef,
		Message:    message,
		ReceivedAt: delivery.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func normalizeGitHubRepository(owner string, repo string, repository string) (string, string, string, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	repository = normalizeRepositoryName(repository)
	if repository != "" {
		parts := strings.Split(repository, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", "", fmt.Errorf("repository must be owner/repo")
		}
		if owner == "" {
			owner = parts[0]
		}
		if repo == "" {
			repo = parts[1]
		}
	}
	if owner == "" || repo == "" {
		return "", "", "", fmt.Errorf("owner and repo are required")
	}
	owner = strings.ToLower(owner)
	repo = strings.ToLower(repo)
	return owner, repo, owner + "/" + repo, nil
}

func normalizeRepositoryName(repository string) string {
	repository = strings.TrimSpace(strings.TrimPrefix(repository, "https://github.com/"))
	repository = strings.TrimSuffix(repository, ".git")
	repository = strings.Trim(repository, "/")
	return strings.ToLower(repository)
}

func cleanRepoPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	return value
}

func githubRepoPathResource(repository string, filePath string) string {
	return normalizeRepositoryName(repository) + ":" + cleanRepoPath(filePath)
}

func githubFileOperation(file GitHubChangedFile) string {
	if operation := strings.TrimSpace(file.Operation); operation != "" {
		return operation
	}
	switch strings.ToLower(strings.TrimSpace(file.Status)) {
	case "removed", "deleted":
		return "delete"
	case "added", "modified", "renamed", "changed":
		return "edit"
	default:
		return "edit"
	}
}

func mergeGitHubCheckContext(req GitHubCheckRunPlanRequest, file GitHubChangedFile, repository string) map[string]any {
	context := map[string]any{}
	for key, value := range req.Context {
		context[key] = value
	}
	context["github.repository"] = repository
	context["github.branch"] = strings.TrimSpace(req.Branch)
	context["github.pull_request"] = req.PullRequest
	context["github.head_sha"] = strings.TrimSpace(req.HeadSHA)
	context["github.file_path"] = cleanRepoPath(file.Path)
	context["github.file_status"] = strings.TrimSpace(file.Status)
	context["github.file_additions"] = file.Additions
	context["github.file_deletions"] = file.Deletions
	return context
}

func conclusionForDecision(decision Decision) string {
	switch decision {
	case DecisionAllow, DecisionAllowWithConstraint:
		return GitHubCheckConclusionSuccess
	case DecisionDeny, DecisionSuspend:
		return GitHubCheckConclusionFailure
	case DecisionRequireApproval, DecisionRequireExpansion:
		return GitHubCheckConclusionActionRequired
	default:
		return GitHubCheckConclusionNeutral
	}
}

func combineGitHubConclusion(current string, next string) string {
	rank := map[string]int{
		GitHubCheckConclusionSuccess:        0,
		GitHubCheckConclusionNeutral:        1,
		GitHubCheckConclusionActionRequired: 2,
		GitHubCheckConclusionFailure:        3,
	}
	if rank[next] > rank[current] {
		return next
	}
	return current
}

func githubCheckText(evaluations []GitHubCheckEvaluation) string {
	counts := map[Decision]int{}
	for _, evaluation := range evaluations {
		counts[evaluation.Decision]++
	}
	parts := make([]string, 0, len(counts))
	for _, decision := range []Decision{DecisionAllow, DecisionRequireApproval, DecisionRequireExpansion, DecisionRequireRefresh, DecisionDeny, DecisionSuspend} {
		if count := counts[decision]; count > 0 {
			parts = append(parts, strconv.Itoa(count)+" "+string(decision))
		}
	}
	return strings.Join(parts, ", ")
}

func cleanStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		i, _ := typed.Int64()
		return int(i)
	default:
		return 0
	}
}

func branchFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return ref
}
