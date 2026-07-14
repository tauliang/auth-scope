package salesforce

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tauliang/auth-scope/internal/mission/integrations/contract"
)

type Store interface {
	SaveOrgBinding(OrgBinding) error
	GetOrgBinding(string) (OrgBinding, error)
	UpdateOrgBinding(OrgBinding) error
	ListOrgBindings() ([]OrgBinding, error)
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

func (s *Service) CreateOrgBinding(req CreateOrgBindingRequest, actor Principal) (OrgBinding, error) {
	instanceURL, err := NormalizeInstanceURL(req.InstanceURL)
	if err != nil {
		return OrgBinding{}, err
	}
	missionRef := strings.TrimSpace(req.MissionRef)
	if missionRef == "" {
		return OrgBinding{}, fmt.Errorf("mission_ref is required")
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	permissionSetMatchMode := strings.ToLower(strings.TrimSpace(req.PermissionSetMatchMode))
	if permissionSetMatchMode == "" {
		permissionSetMatchMode = PermissionMatchAny
	}
	if permissionSetMatchMode != PermissionMatchAny && permissionSetMatchMode != PermissionMatchAll {
		return OrgBinding{}, fmt.Errorf("permission_set_match_mode must be any or all")
	}
	profileClaim := strings.TrimSpace(req.ProfileClaim)
	if profileClaim == "" {
		profileClaim = "profile"
	}
	permissionSetsClaim := strings.TrimSpace(req.PermissionSetsClaim)
	if permissionSetsClaim == "" {
		permissionSetsClaim = "permission_sets"
	}
	subjectClaim := strings.TrimSpace(req.SubjectClaim)
	if subjectClaim == "" {
		subjectClaim = "sub"
	}
	usernameClaim := strings.TrimSpace(req.UsernameClaim)
	if usernameClaim == "" {
		usernameClaim = "username"
	}
	emailClaim := strings.TrimSpace(req.EmailClaim)
	if emailClaim == "" {
		emailClaim = "email"
	}

	now := s.now()
	binding := OrgBinding{
		BindingID:              s.id("sfb"),
		TenantID:               strings.TrimSpace(req.TenantID),
		InstanceURL:            instanceURL,
		OrgID:                  strings.TrimSpace(req.OrgID),
		OrgName:                strings.TrimSpace(req.OrgName),
		MissionRef:             missionRef,
		AllowedObjectAPINames:  CleanStringList(req.AllowedObjectAPINames),
		AllowedRecordTypeIDs:   CleanStringList(req.AllowedRecordTypeIDs),
		AllowedRecordTypeNames: CleanStringList(req.AllowedRecordTypeNames),
		AllowedActions:         CleanStringList(req.AllowedActions),
		RequiredProfiles:       CleanStringList(req.RequiredProfiles),
		RequiredPermissionSets: CleanStringList(req.RequiredPermissionSets),
		AdminProfiles:          CleanStringList(req.AdminProfiles),
		AdminPermissionSets:    CleanStringList(req.AdminPermissionSets),
		AllowedSubjects:        CleanStringList(req.AllowedSubjects),
		ProfileClaim:           profileClaim,
		PermissionSetsClaim:    permissionSetsClaim,
		SubjectClaim:           subjectClaim,
		UsernameClaim:          usernameClaim,
		EmailClaim:             emailClaim,
		PermissionSetMatchMode: permissionSetMatchMode,
		Status:                 OrgBindingStatusActive,
		Metadata:               CloneStringMap(req.Metadata),
		CreatedBy:              actor,
		CreatedAt:              now,
	}
	if err := s.store.SaveOrgBinding(binding); err != nil {
		return OrgBinding{}, err
	}
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		TenantID:   binding.TenantID,
		MissionRef: binding.MissionRef,
		Type:       "salesforce.org_bound",
		Actor:      map[string]any{"subject": actor.Subject, "issuer": actor.Issuer},
		Payload: map[string]any{
			"binding_id":      binding.BindingID,
			"instance_url":    binding.InstanceURL,
			"org_id":          binding.OrgID,
			"mission_ref":     binding.MissionRef,
			"allowed_objects": binding.AllowedObjectAPINames,
			"allowed_actions": binding.AllowedActions,
		},
		OccurredAt: now,
	})
	return binding, nil
}

func (s *Service) ListOrgBindings() ([]OrgBinding, error) {
	return s.store.ListOrgBindings()
}

func (s *Service) AuthorizeRecordAction(req AuthorizeRecordActionRequest) (RecordActionAuthorizationResponse, error) {
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return RecordActionAuthorizationResponse{}, fmt.Errorf("action is required")
	}
	objectAPIName := NormalizeObjectAPIName(req.ObjectAPIName)
	if objectAPIName == "" {
		return RecordActionAuthorizationResponse{}, fmt.Errorf("object_api_name is required")
	}
	binding, ok, err := s.bindingFor(strings.TrimSpace(req.InstanceURL), strings.TrimSpace(req.OrgID), strings.TrimSpace(req.MissionRef), strings.TrimSpace(req.TenantID))
	if err != nil {
		return RecordActionAuthorizationResponse{}, err
	}
	if !ok {
		return s.deniedResponse(OrgBinding{}, action, []string{"salesforce_no_matching_binding"}, "No active Salesforce org binding matches the record action request."), nil
	}

	userID, subject, username, email, profile, permissionSets, err := ExtractUserContext(req, binding)
	if err != nil {
		return RecordActionAuthorizationResponse{}, err
	}
	reasons := identityReasonCodes(binding, userID, subject, username, email, profile, permissionSets)
	if len(binding.AllowedObjectAPINames) > 0 && !ContainsString(binding.AllowedObjectAPINames, objectAPIName) {
		reasons = append(reasons, "salesforce_object_not_allowed")
	}
	if len(binding.AllowedRecordTypeIDs) > 0 && !ContainsString(binding.AllowedRecordTypeIDs, req.RecordTypeID) {
		reasons = append(reasons, "salesforce_record_type_id_not_allowed")
	}
	if len(binding.AllowedRecordTypeNames) > 0 && !ContainsString(binding.AllowedRecordTypeNames, req.RecordTypeName) {
		reasons = append(reasons, "salesforce_record_type_name_not_allowed")
	}
	if len(binding.AllowedActions) > 0 && !ContainsString(binding.AllowedActions, action) {
		reasons = append(reasons, "salesforce_action_not_allowed")
	}

	context := CloneContext(req.Context)
	context["salesforce.instance_url"] = binding.InstanceURL
	context["salesforce.org_id"] = binding.OrgID
	context["salesforce.binding_id"] = binding.BindingID
	context["salesforce.user_id"] = userID
	context["salesforce.subject"] = subject
	context["salesforce.username"] = username
	context["salesforce.email"] = email
	context["salesforce.profile"] = profile
	context["salesforce.permission_sets"] = permissionSets
	context["salesforce.object_api_name"] = objectAPIName
	context["salesforce.record_id"] = strings.TrimSpace(req.RecordID)
	context["salesforce.record_type_id"] = strings.TrimSpace(req.RecordTypeID)
	context["salesforce.record_type_name"] = strings.TrimSpace(req.RecordTypeName)
	context["salesforce.action"] = action

	resp := RecordActionAuthorizationResponse{
		BindingID:      binding.BindingID,
		TenantID:       binding.TenantID,
		MissionRef:     binding.MissionRef,
		InstanceURL:    binding.InstanceURL,
		OrgID:          binding.OrgID,
		ObjectAPIName:  objectAPIName,
		RecordID:       strings.TrimSpace(req.RecordID),
		RecordTypeID:   strings.TrimSpace(req.RecordTypeID),
		RecordTypeName: strings.TrimSpace(req.RecordTypeName),
		UserID:         userID,
		Subject:        subject,
		Username:       username,
		Email:          email,
		Profile:        profile,
		PermissionSets: permissionSets,
		Action:         action,
		Admin:          ContainsString(binding.AdminProfiles, profile) || HasAny(permissionSets, binding.AdminPermissionSets),
		Context:        context,
	}
	return s.finishAuthorization(binding, resp, reasons, req.Evaluation)
}

func (s *Service) bindingFor(instanceURL string, orgID string, missionRef string, tenantID string) (OrgBinding, bool, error) {
	normalizedInstanceURL := ""
	if strings.TrimSpace(instanceURL) != "" {
		var err error
		normalizedInstanceURL, err = NormalizeInstanceURL(instanceURL)
		if err != nil {
			return OrgBinding{}, false, err
		}
	}
	orgID = strings.TrimSpace(orgID)
	if normalizedInstanceURL == "" && orgID == "" {
		return OrgBinding{}, false, fmt.Errorf("instance_url or org_id is required")
	}
	bindings, err := s.store.ListOrgBindings()
	if err != nil {
		return OrgBinding{}, false, err
	}
	for _, binding := range bindings {
		if binding.Status != OrgBindingStatusActive {
			continue
		}
		if tenantID != "" && binding.TenantID != tenantID {
			continue
		}
		if missionRef != "" && binding.MissionRef != missionRef {
			continue
		}
		instanceMatches := normalizedInstanceURL != "" && binding.InstanceURL == normalizedInstanceURL
		orgMatches := orgID != "" && binding.OrgID != "" && binding.OrgID == orgID
		switch {
		case normalizedInstanceURL != "" && orgID != "":
			if !instanceMatches {
				continue
			}
			if binding.OrgID != "" && !orgMatches {
				continue
			}
		case normalizedInstanceURL != "":
			if !instanceMatches {
				continue
			}
		case orgID != "":
			if !orgMatches {
				continue
			}
		}
		return binding, true, nil
	}
	return OrgBinding{}, false, nil
}

func identityReasonCodes(binding OrgBinding, userID string, subject string, username string, email string, profile string, permissionSets []string) []string {
	reasons := make([]string, 0, 3)
	if !ContainsAnyIdentity(binding.AllowedSubjects, userID, subject, username, email) {
		reasons = append(reasons, "salesforce_subject_not_allowed")
	}
	if len(binding.RequiredProfiles) > 0 && !ContainsString(binding.RequiredProfiles, profile) {
		reasons = append(reasons, "salesforce_required_profile_missing")
	}
	if !PermissionSetRequirementSatisfied(binding.RequiredPermissionSets, permissionSets, binding.PermissionSetMatchMode) {
		reasons = append(reasons, "salesforce_required_permission_set_missing")
	}
	return reasons
}

func (s *Service) finishAuthorization(binding OrgBinding, resp RecordActionAuthorizationResponse, reasons []string, evaluation *EvaluationRequest) (RecordActionAuthorizationResponse, error) {
	now := s.now()
	binding.LastResolvedAt = now
	binding.LastSubject = firstString(resp.Subject, resp.UserID, resp.Username, resp.Email)
	resp.ResolvedAt = now.Format(time.RFC3339)

	if len(reasons) > 0 {
		binding.LastResolutionStatus = ResolutionStatusDenied
		_ = s.store.UpdateOrgBinding(binding)
		resp.Accepted = false
		resp.Status = ResolutionStatusDenied
		resp.ReasonCodes = reasons
		resp.HumanReason = "Salesforce subject or record action does not satisfy the active org binding requirements."
		s.appendAuthorizationEvent(binding, resp, now)
		return resp, nil
	}

	resp.Accepted = true
	resp.Status = ResolutionStatusAccepted
	resp.ReasonCodes = []string{"salesforce_binding_satisfied"}
	resp.HumanReason = "Salesforce subject and record action satisfy the active org binding requirements."
	if evaluation != nil {
		if s.evaluator == nil {
			return RecordActionAuthorizationResponse{}, fmt.Errorf("salesforce evaluator is not configured")
		}
		evalReq := *evaluation
		evalReq.MissionRef = binding.MissionRef
		evalReq.Context = resp.Context
		decision, err := s.evaluator.Evaluate(evalReq)
		if err != nil {
			return RecordActionAuthorizationResponse{}, err
		}
		resp.Evaluation = &decision
	}

	binding.LastResolutionStatus = ResolutionStatusAccepted
	_ = s.store.UpdateOrgBinding(binding)
	s.appendAuthorizationEvent(binding, resp, now)
	return resp, nil
}

func (s *Service) deniedResponse(binding OrgBinding, action string, reasons []string, humanReason string) RecordActionAuthorizationResponse {
	return RecordActionAuthorizationResponse{
		Accepted:    false,
		Status:      ResolutionStatusDenied,
		BindingID:   binding.BindingID,
		TenantID:    binding.TenantID,
		MissionRef:  binding.MissionRef,
		InstanceURL: binding.InstanceURL,
		OrgID:       binding.OrgID,
		Action:      action,
		ReasonCodes: reasons,
		HumanReason: humanReason,
		ResolvedAt:  s.now().Format(time.RFC3339),
	}
}

func (s *Service) appendAuthorizationEvent(binding OrgBinding, resp RecordActionAuthorizationResponse, occurredAt time.Time) {
	s.appendEvent(Event{
		EventID:    s.id("mev"),
		MissionRef: binding.MissionRef,
		TenantID:   binding.TenantID,
		Type:       "salesforce.record_action_authorized",
		Actor:      map[string]any{"user_id": resp.UserID, "subject": resp.Subject, "username": resp.Username, "email": resp.Email},
		Payload: map[string]any{
			"binding_id":       resp.BindingID,
			"action":           resp.Action,
			"accepted":         resp.Accepted,
			"status":           resp.Status,
			"reason_codes":     resp.ReasonCodes,
			"admin":            resp.Admin,
			"object_api_name":  resp.ObjectAPIName,
			"record_id":        resp.RecordID,
			"record_type_id":   resp.RecordTypeID,
			"record_type_name": resp.RecordTypeName,
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
