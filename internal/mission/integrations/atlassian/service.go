package atlassian

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

type Store interface {
	SaveSiteBinding(SiteBinding) error
	GetSiteBinding(string) (SiteBinding, error)
	UpdateSiteBinding(SiteBinding) error
	ListSiteBindings() ([]SiteBinding, error)
}

type Evaluator = contract.Evaluator
type EventSink = contract.EventSink
type Clock = contract.Clock

type Config struct {
	Store      Store
	Evaluator  Evaluator
	Events     EventSink
	Clock      Clock
	NewID      func(string) string
	IsConflict func(error) bool
}

type Service struct {
	store      Store
	evaluator  Evaluator
	events     EventSink
	clock      Clock
	newID      func(string) string
	isConflict func(error) bool
}

func NewService(config Config) *Service {
	isConflict := config.IsConflict
	if isConflict == nil {
		isConflict = func(error) bool { return false }
	}
	return &Service{
		store:      config.Store,
		evaluator:  config.Evaluator,
		events:     config.Events,
		clock:      config.Clock,
		newID:      config.NewID,
		isConflict: isConflict,
	}
}

func (s *Service) CreateSiteBinding(req CreateSiteBindingRequest, actor Principal) (SiteBinding, error) {
	siteURL, err := NormalizeSiteURL(req.SiteURL)
	if err != nil {
		return SiteBinding{}, err
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		return SiteBinding{}, fmt.Errorf("mission_ref is required")
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	groupMatchMode := strings.ToLower(strings.TrimSpace(req.GroupMatchMode))
	if groupMatchMode == "" {
		groupMatchMode = GroupMatchAny
	}
	if groupMatchMode != GroupMatchAny && groupMatchMode != GroupMatchAll {
		return SiteBinding{}, fmt.Errorf("group_match_mode must be any or all")
	}
	groupClaim := strings.TrimSpace(req.GroupClaim)
	if groupClaim == "" {
		groupClaim = "groups"
	}
	subjectClaim := strings.TrimSpace(req.SubjectClaim)
	if subjectClaim == "" {
		subjectClaim = "sub"
	}
	emailClaim := strings.TrimSpace(req.EmailClaim)
	if emailClaim == "" {
		emailClaim = "email"
	}

	now := s.now()
	binding := SiteBinding{
		BindingID:           s.id("atb"),
		TenantID:            strings.TrimSpace(req.TenantID),
		SiteURL:             siteURL,
		CloudID:             strings.TrimSpace(req.CloudID),
		SiteName:            strings.TrimSpace(req.SiteName),
		MissionRef:          missionRef,
		JiraProjectKeys:     CleanKeyList(req.JiraProjectKeys),
		ConfluenceSpaceKeys: CleanKeyList(req.ConfluenceSpaceKeys),
		AllowedJiraActions:  CleanStringList(req.AllowedJiraActions),
		AllowedPageActions:  CleanStringList(req.AllowedPageActions),
		RequiredGroups:      CleanStringList(req.RequiredGroups),
		AdminGroups:         CleanStringList(req.AdminGroups),
		AllowedSubjects:     CleanStringList(req.AllowedSubjects),
		GroupClaim:          groupClaim,
		SubjectClaim:        subjectClaim,
		EmailClaim:          emailClaim,
		GroupMatchMode:      groupMatchMode,
		Status:              SiteBindingStatusActive,
		Metadata:            CloneStringMap(req.Metadata),
		CreatedBy:           actor,
		CreatedAt:           now,
	}
	if err := s.store.SaveSiteBinding(binding); err != nil {
		return SiteBinding{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		TenantID:   binding.TenantID,
		MissionRef: binding.MissionRef,
		Type:       "atlassian.site_bound",
		Actor:      map[string]any{"subject": actor.Subject, "issuer": actor.Issuer},
		Payload: map[string]any{
			"binding_id":        binding.BindingID,
			"site_url":          binding.SiteURL,
			"cloud_id":          binding.CloudID,
			"mission_ref":       binding.MissionRef,
			"jira_projects":     binding.JiraProjectKeys,
			"confluence_spaces": binding.ConfluenceSpaceKeys,
		},
		OccurredAt: now,
	})
	return binding, nil
}

func (s *Service) ListSiteBindings() ([]SiteBinding, error) {
	return s.store.ListSiteBindings()
}

func (s *Service) AuthorizeJiraIssueAction(req AuthorizeJiraIssueActionRequest) (ActionAuthorizationResponse, error) {
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return ActionAuthorizationResponse{}, fmt.Errorf("action is required")
	}
	projectKey := NormalizeProjectKey(req.ProjectKey)
	if projectKey == "" {
		projectKey = ProjectKeyFromIssueKey(req.IssueKey)
	}
	if projectKey == "" {
		return ActionAuthorizationResponse{}, fmt.Errorf("project_key is required")
	}
	binding, ok, err := s.bindingFor(strings.TrimSpace(req.SiteURL), strings.TrimSpace(req.CloudID), strings.TrimSpace(req.MissionRef), strings.TrimSpace(req.TenantID))
	if err != nil {
		return ActionAuthorizationResponse{}, err
	}
	if !ok {
		return s.deniedResponse("jira", SiteBinding{}, action, []string{"atlassian_no_matching_binding"}, "No active Atlassian site binding matches the Jira request."), nil
	}

	accountID, subject, email, groups, err := ExtractJiraUserContext(req, binding)
	if err != nil {
		return ActionAuthorizationResponse{}, err
	}
	reasons := identityReasonCodes(binding, accountID, subject, email, groups)
	if len(binding.JiraProjectKeys) > 0 && !ContainsString(binding.JiraProjectKeys, projectKey) {
		reasons = append(reasons, "jira_project_not_allowed")
	}
	if len(binding.AllowedJiraActions) > 0 && !ContainsString(binding.AllowedJiraActions, action) {
		reasons = append(reasons, "jira_action_not_allowed")
	}

	context := CloneContext(req.Context)
	context["atlassian.product"] = "jira"
	context["atlassian.site_url"] = binding.SiteURL
	context["atlassian.cloud_id"] = binding.CloudID
	context["atlassian.binding_id"] = binding.BindingID
	context["atlassian.account_id"] = accountID
	context["atlassian.subject"] = subject
	context["atlassian.email"] = email
	context["atlassian.groups"] = groups
	context["jira.project_key"] = projectKey
	context["jira.issue_key"] = strings.ToUpper(strings.TrimSpace(req.IssueKey))
	context["jira.issue_type"] = strings.TrimSpace(req.IssueType)
	context["jira.action"] = action

	resp := ActionAuthorizationResponse{
		Product:    "jira",
		BindingID:  binding.BindingID,
		TenantID:   binding.TenantID,
		MissionRef: binding.MissionRef,
		SiteURL:    binding.SiteURL,
		CloudID:    binding.CloudID,
		ProjectKey: projectKey,
		IssueKey:   strings.ToUpper(strings.TrimSpace(req.IssueKey)),
		IssueType:  strings.TrimSpace(req.IssueType),
		AccountID:  accountID,
		Subject:    subject,
		Email:      email,
		Groups:     groups,
		Action:     action,
		Admin:      HasAny(groups, binding.AdminGroups),
		Context:    context,
	}
	return s.finishAuthorization(binding, resp, reasons, req.Evaluation, "atlassian.jira_issue_action_authorized")
}

func (s *Service) AuthorizeConfluencePageAction(req AuthorizeConfluencePageActionRequest) (ActionAuthorizationResponse, error) {
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return ActionAuthorizationResponse{}, fmt.Errorf("action is required")
	}
	spaceKey := NormalizeSpaceKey(req.SpaceKey)
	if spaceKey == "" {
		return ActionAuthorizationResponse{}, fmt.Errorf("space_key is required")
	}
	binding, ok, err := s.bindingFor(strings.TrimSpace(req.SiteURL), strings.TrimSpace(req.CloudID), strings.TrimSpace(req.MissionRef), strings.TrimSpace(req.TenantID))
	if err != nil {
		return ActionAuthorizationResponse{}, err
	}
	if !ok {
		return s.deniedResponse("confluence", SiteBinding{}, action, []string{"atlassian_no_matching_binding"}, "No active Atlassian site binding matches the Confluence request."), nil
	}

	accountID, subject, email, groups, err := ExtractConfluenceUserContext(req, binding)
	if err != nil {
		return ActionAuthorizationResponse{}, err
	}
	reasons := identityReasonCodes(binding, accountID, subject, email, groups)
	if len(binding.ConfluenceSpaceKeys) > 0 && !ContainsString(binding.ConfluenceSpaceKeys, spaceKey) {
		reasons = append(reasons, "confluence_space_not_allowed")
	}
	if len(binding.AllowedPageActions) > 0 && !ContainsString(binding.AllowedPageActions, action) {
		reasons = append(reasons, "confluence_action_not_allowed")
	}

	context := CloneContext(req.Context)
	context["atlassian.product"] = "confluence"
	context["atlassian.site_url"] = binding.SiteURL
	context["atlassian.cloud_id"] = binding.CloudID
	context["atlassian.binding_id"] = binding.BindingID
	context["atlassian.account_id"] = accountID
	context["atlassian.subject"] = subject
	context["atlassian.email"] = email
	context["atlassian.groups"] = groups
	context["confluence.space_key"] = spaceKey
	context["confluence.page_id"] = strings.TrimSpace(req.PageID)
	context["confluence.page_title"] = strings.TrimSpace(req.PageTitle)
	context["confluence.action"] = action

	resp := ActionAuthorizationResponse{
		Product:    "confluence",
		BindingID:  binding.BindingID,
		TenantID:   binding.TenantID,
		MissionRef: binding.MissionRef,
		SiteURL:    binding.SiteURL,
		CloudID:    binding.CloudID,
		SpaceKey:   spaceKey,
		PageID:     strings.TrimSpace(req.PageID),
		PageTitle:  strings.TrimSpace(req.PageTitle),
		AccountID:  accountID,
		Subject:    subject,
		Email:      email,
		Groups:     groups,
		Action:     action,
		Admin:      HasAny(groups, binding.AdminGroups),
		Context:    context,
	}
	return s.finishAuthorization(binding, resp, reasons, req.Evaluation, "atlassian.confluence_page_action_authorized")
}

func (s *Service) bindingFor(siteURL string, cloudID string, missionRef string, tenantID string) (SiteBinding, bool, error) {
	normalizedSiteURL := ""
	if strings.TrimSpace(siteURL) != "" {
		var err error
		normalizedSiteURL, err = NormalizeSiteURL(siteURL)
		if err != nil {
			return SiteBinding{}, false, err
		}
	}
	cloudID = strings.TrimSpace(cloudID)
	if normalizedSiteURL == "" && cloudID == "" {
		return SiteBinding{}, false, fmt.Errorf("site_url or cloud_id is required")
	}
	bindings, err := s.store.ListSiteBindings()
	if err != nil {
		return SiteBinding{}, false, err
	}
	for _, binding := range bindings {
		if binding.Status != SiteBindingStatusActive {
			continue
		}
		if tenantID != "" && binding.TenantID != tenantID {
			continue
		}
		if missionRef != "" && binding.MissionRef != missionRef {
			continue
		}
		siteMatches := normalizedSiteURL != "" && binding.SiteURL == normalizedSiteURL
		cloudMatches := cloudID != "" && binding.CloudID != "" && binding.CloudID == cloudID
		switch {
		case normalizedSiteURL != "" && cloudID != "":
			if !siteMatches {
				continue
			}
			if binding.CloudID != "" && !cloudMatches {
				continue
			}
		case normalizedSiteURL != "":
			if !siteMatches {
				continue
			}
		case cloudID != "":
			if !cloudMatches {
				continue
			}
		}
		return binding, true, nil
	}
	return SiteBinding{}, false, nil
}

func identityReasonCodes(binding SiteBinding, accountID string, subject string, email string, groups []string) []string {
	reasons := make([]string, 0, 2)
	if !ContainsAnyIdentity(binding.AllowedSubjects, accountID, subject, email) {
		reasons = append(reasons, "atlassian_subject_not_allowed")
	}
	if !GroupRequirementSatisfied(binding.RequiredGroups, groups, binding.GroupMatchMode) {
		reasons = append(reasons, "atlassian_required_group_missing")
	}
	return reasons
}

func (s *Service) finishAuthorization(binding SiteBinding, resp ActionAuthorizationResponse, reasons []string, evaluation *EvaluationRequest, eventType string) (ActionAuthorizationResponse, error) {
	now := s.now()
	binding.LastResolvedAt = now
	binding.LastSubject = firstString(resp.Subject, resp.AccountID, resp.Email)
	resp.ResolvedAt = now.Format(time.RFC3339)

	if len(reasons) > 0 {
		binding.LastResolutionStatus = ResolutionStatusDenied
		_ = s.store.UpdateSiteBinding(binding)
		resp.Accepted = false
		resp.Status = ResolutionStatusDenied
		resp.ReasonCodes = reasons
		resp.HumanReason = "Atlassian subject or resource does not satisfy the active site binding requirements."
		s.appendAuthorizationEvent(binding, resp, eventType, now)
		return resp, nil
	}

	resp.Accepted = true
	resp.Status = ResolutionStatusAccepted
	resp.ReasonCodes = []string{"atlassian_binding_satisfied"}
	resp.HumanReason = "Atlassian subject and resource satisfy the active site binding requirements."
	if evaluation != nil {
		if s.evaluator == nil {
			return ActionAuthorizationResponse{}, fmt.Errorf("atlassian evaluator is not configured")
		}
		evalReq := *evaluation
		evalReq.MissionRef = binding.MissionRef
		evalReq.Context = resp.Context
		decision, err := s.evaluator.Evaluate(evalReq)
		if err != nil {
			return ActionAuthorizationResponse{}, err
		}
		resp.Evaluation = &decision
	}

	binding.LastResolutionStatus = ResolutionStatusAccepted
	_ = s.store.UpdateSiteBinding(binding)
	s.appendAuthorizationEvent(binding, resp, eventType, now)
	return resp, nil
}

func (s *Service) deniedResponse(product string, binding SiteBinding, action string, reasons []string, humanReason string) ActionAuthorizationResponse {
	return ActionAuthorizationResponse{
		Accepted:    false,
		Status:      ResolutionStatusDenied,
		Product:     product,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		SiteURL:     binding.SiteURL,
		CloudID:     binding.CloudID,
		Action:      action,
		ReasonCodes: reasons,
		HumanReason: humanReason,
		ResolvedAt:  s.now().Format(time.RFC3339),
	}
}

func (s *Service) appendAuthorizationEvent(binding SiteBinding, resp ActionAuthorizationResponse, eventType string, occurredAt time.Time) {
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       eventType,
		Actor:      map[string]any{"account_id": resp.AccountID, "subject": resp.Subject, "email": resp.Email},
		Payload: map[string]any{
			"binding_id":   resp.BindingID,
			"product":      resp.Product,
			"action":       resp.Action,
			"accepted":     resp.Accepted,
			"status":       resp.Status,
			"reason_codes": resp.ReasonCodes,
			"admin":        resp.Admin,
			"project_key":  resp.ProjectKey,
			"issue_key":    resp.IssueKey,
			"space_key":    resp.SpaceKey,
			"page_id":      resp.PageID,
		},
		OccurredAt: occurredAt,
	})
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
