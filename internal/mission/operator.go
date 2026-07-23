package mission

const (
	DefaultCollectionLimit = 25
	MaxCollectionLimit     = 100
)

type ListQuery struct {
	TenantID string
	State    string
	Status   string
	Type     string
	Query    string
	Cursor   string
	Limit    int
}

type CollectionPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	Total      int    `json:"total"`
}

type OperationsSummary struct {
	MissionsTotal       int             `json:"missions_total"`
	MissionsByState     map[State]int   `json:"missions_by_state"`
	PendingProposals    int             `json:"pending_proposals"`
	PendingExpansions   int             `json:"pending_expansions"`
	ActiveContainments  int             `json:"active_containments"`
	ActiveAgents        int             `json:"active_agents"`
	ActiveProjections   int             `json:"active_projections"`
	RecentEventCount    int             `json:"recent_event_count"`
	ServiceCapabilities map[string]bool `json:"service_capabilities"`
}

type AdminSessionResponse struct {
	Principal    Principal       `json:"principal"`
	Provider     string          `json:"provider,omitempty"`
	Groups       []string        `json:"groups,omitempty"`
	Roles        []string        `json:"roles,omitempty"`
	Permissions  []string        `json:"permissions,omitempty"`
	Capabilities map[string]bool `json:"capabilities"`
	APIVersion   string          `json:"api_version"`
}
