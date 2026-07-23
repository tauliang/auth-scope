package mission

import (
	"net/http"
	"strings"
	"time"
)

type AdminActionAudit struct {
	Identity   AdminIdentity
	Method     string
	Path       string
	Permission AdminPermission
	Allowed    bool
	StatusCode int
	Reason     string
	RequestID  string
	OccurredAt time.Time
}

type AdminAuditAPI interface {
	RecordAdminAction(AdminActionAudit)
}

func (s *Service) RecordAdminAction(action AdminActionAudit) {
	if s == nil || s.events == nil {
		return
	}
	occurredAt := action.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = s.clock.Now()
	}
	statusCode := action.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
		TenantID:   action.Identity.Principal.TenantSubject,
		Type:       "admin.action",
		Actor:      adminIdentityActor(action.Identity),
		Payload:    adminActionPayload(action, statusCode),
		OccurredAt: occurredAt,
	})
}

func adminIdentityActor(identity AdminIdentity) map[string]any {
	actor := principalActor(identity.Principal)
	if identity.Provider != "" {
		actor["provider"] = identity.Provider
	}
	if len(identity.Groups) > 0 {
		actor["groups"] = identity.Groups
	}
	if len(identity.Roles) > 0 {
		actor["roles"] = identity.Roles
	}
	if len(identity.Permissions) > 0 {
		actor["permissions"] = identity.Permissions
	}
	return actor
}

func adminActionPayload(action AdminActionAudit, statusCode int) map[string]any {
	payload := map[string]any{
		"method":      strings.ToUpper(strings.TrimSpace(action.Method)),
		"path":        strings.TrimSpace(action.Path),
		"permission":  string(action.Permission),
		"allowed":     action.Allowed,
		"status_code": statusCode,
	}
	if action.Reason != "" {
		payload["reason"] = action.Reason
	}
	if action.RequestID != "" {
		payload["request_id"] = action.RequestID
	}
	return payload
}
