package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/lib/pq"
	"github.com/tauliang/auth-scope/internal/mission"
)

//go:embed migrations/*.up.sql
var migrationFS embed.FS

// PostgresStore implements the Store interface using PostgreSQL.
type PostgresStore struct {
	db     *sql.DB
	clock  mission.Clock
	logger *slog.Logger
}

// NewPostgresStore creates a new PostgresStore instance.
func NewPostgresStore(db *sql.DB, clock mission.Clock) (*PostgresStore, error) {
	if db == nil {
		return nil, errors.New("db cannot be nil")
	}
	if clock == nil {
		clock = mission.SystemClock{}
	}

	return &PostgresStore{
		db:     db,
		clock:  clock,
		logger: slog.Default().With("component", "postgres-store"),
	}, nil
}

// NewPostgresStoreFromEnv creates a PostgresStore using DATABASE_URL from environment.
func NewPostgresStoreFromEnv() (*PostgresStore, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := ApplyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return NewPostgresStore(db, mission.SystemClock{})
}

// ApplyMigrations applies embedded up migrations that have not yet run.
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("db cannot be nil")
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	names, err := MigrationNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		applied, err := migrationApplied(ctx, db, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		sqlBytes, err := migrationFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// MigrationNames returns embedded up migrations in apply order.
func MigrationNames() ([]string, error) {
	names, err := fs.Glob(migrationFS, "migrations/*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(names)
	return names, nil
}

func migrationApplied(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&exists); err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return exists, nil
}

// Close closes the database connection.
func (s *PostgresStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// SaveAgentIdentity saves a registered agent identity to the database.
func (s *PostgresStore) SaveAgentIdentity(identity mission.AgentIdentity) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	identityJSON, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("marshal agent identity: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveAgentIdentity", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agent_identities (
			id, tenant_id, provider, client_id, instance_id, key_thumbprint, public_key,
			status, identity_json, created_at, revoked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, identity.AgentID, identity.TenantID, identity.Agent.Provider, identity.Agent.ClientID, identity.Agent.InstanceID,
		identity.KeyThumbprint, identity.PublicKey, identity.Status, identityJSON, identity.CreatedAt, nullableTime(identity.RevokedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert agent identity: %w", err)
	}
	return nil
}

// GetAgentIdentity retrieves a registered agent identity by ID.
func (s *PostgresStore) GetAgentIdentity(agentID string) (mission.AgentIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetAgentIdentity", startTime)

	var identityJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT identity_json FROM agent_identities WHERE id = $1`, agentID).Scan(&identityJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.AgentIdentity{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.AgentIdentity{}, fmt.Errorf("query agent identity: %w", err)
	}

	var identity mission.AgentIdentity
	if err := json.Unmarshal(identityJSON, &identity); err != nil {
		return mission.AgentIdentity{}, fmt.Errorf("unmarshal agent identity: %w", err)
	}
	return identity, nil
}

// UpdateAgentIdentity updates a registered agent identity.
func (s *PostgresStore) UpdateAgentIdentity(identity mission.AgentIdentity) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	identityJSON, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("marshal agent identity: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("UpdateAgentIdentity", startTime)

	result, err := s.db.ExecContext(ctx, `
		UPDATE agent_identities SET
			status = $1,
			identity_json = $2,
			revoked_at = $3
		WHERE id = $4
	`, identity.Status, identityJSON, nullableTime(identity.RevokedAt), identity.AgentID)
	if err != nil {
		return fmt.Errorf("update agent identity: %w", err)
	}
	return rowsAffectedErr(result)
}

// ListAgentIdentities lists registered agent identities.
func (s *PostgresStore) ListAgentIdentities() ([]mission.AgentIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListAgentIdentities", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT identity_json
		FROM agent_identities
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query agent identities: %w", err)
	}
	defer rows.Close()

	identities := make([]mission.AgentIdentity, 0)
	for rows.Next() {
		var identityJSON []byte
		if err := rows.Scan(&identityJSON); err != nil {
			return nil, fmt.Errorf("scan agent identity: %w", err)
		}
		var identity mission.AgentIdentity
		if err := json.Unmarshal(identityJSON, &identity); err != nil {
			return nil, fmt.Errorf("unmarshal agent identity: %w", err)
		}
		identities = append(identities, identity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent identities: %w", err)
	}
	return identities, nil
}

// SaveAgentNonce records a signed request nonce for replay protection.
func (s *PostgresStore) SaveAgentNonce(nonce mission.AgentNonce) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("SaveAgentNonce", startTime)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_nonces (agent_id, nonce, request_hash, seen_at)
		VALUES ($1, $2, $3, $4)
	`, nonce.AgentID, nonce.Nonce, nonce.RequestHash, nonce.SeenAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert agent nonce: %w", err)
	}
	return nil
}

// SaveProposal saves a mission proposal to the database.
func (s *PostgresStore) SaveProposal(proposal mission.MissionProposal) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proposalJSON, err := json.Marshal(proposal)
	if err != nil {
		return fmt.Errorf("marshal proposal: %w", err)
	}
	createdAt := proposal.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	query := `
		INSERT INTO mission_proposals (
			id, tenant_id, principal_id, agent_id, status, purpose, proposal_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	startTime := time.Now()
	defer s.logSlowQuery("SaveProposal", startTime)

	_, err = s.db.ExecContext(ctx, query,
		proposal.ProposalID,
		proposal.TenantID,
		proposal.Principal.Subject,
		proposal.Agent.InstanceID,
		string(proposal.Status),
		proposal.Intent.Objective,
		proposalJSON,
		createdAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert proposal: %w", err)
	}

	return nil
}

// GetProposal retrieves a mission proposal by ID.
func (s *PostgresStore) GetProposal(id string) (mission.MissionProposal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetProposal", startTime)

	var proposalJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT proposal_json FROM mission_proposals WHERE id = $1`, id).Scan(&proposalJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.MissionProposal{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.MissionProposal{}, fmt.Errorf("query proposal: %w", err)
	}

	var proposal mission.MissionProposal
	if err := json.Unmarshal(proposalJSON, &proposal); err != nil {
		return mission.MissionProposal{}, fmt.Errorf("unmarshal proposal: %w", err)
	}
	return proposal, nil
}

// ListProposals lists mission proposals in deterministic creation order.
func (s *PostgresStore) ListProposals() ([]mission.MissionProposal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListProposals", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT proposal_json
		FROM mission_proposals
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()

	proposals := make([]mission.MissionProposal, 0)
	for rows.Next() {
		var proposalJSON []byte
		if err := rows.Scan(&proposalJSON); err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}
		var proposal mission.MissionProposal
		if err := json.Unmarshal(proposalJSON, &proposal); err != nil {
			return nil, fmt.Errorf("unmarshal proposal: %w", err)
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proposals: %w", err)
	}
	return proposals, nil
}

// DeleteProposal deletes a mission proposal by ID.
func (s *PostgresStore) DeleteProposal(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("DeleteProposal", startTime)

	result, err := s.db.ExecContext(ctx, `DELETE FROM mission_proposals WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete proposal: %w", err)
	}

	return rowsAffectedErr(result)
}

// CommitProposalApproval atomically activates a proposal and records its audit event.
func (s *PostgresStore) CommitProposalApproval(ctx context.Context, commit mission.ProposalApprovalCommit) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("CommitProposalApproval", startTime)

	missionJSON, err := json.Marshal(commit.Mission)
	if err != nil {
		return fmt.Errorf("marshal mission: %w", err)
	}
	createdAt := commit.Mission.Lifecycle.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}
	now := s.clock.Now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin proposal approval: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO missions (
			ref, mission_id, tenant_id, state, version, principal_id, agent_id,
			parent_mission_ref, purpose, mission_json, created_at, updated_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		commit.Mission.MissionRef,
		commit.Mission.MissionID,
		commit.Mission.TenantID,
		string(commit.Mission.State),
		commit.Mission.Version,
		commit.Mission.Principal.Subject,
		commit.Mission.Agent.InstanceID,
		nullableString(commit.Mission.Delegation.ParentMissionRef),
		commit.Mission.Purpose.Objective,
		missionJSON,
		createdAt,
		now,
		nullableTime(commit.Mission.Lifecycle.ExpiresAt),
	); err != nil {
		_ = tx.Rollback()
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert mission: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM mission_proposals WHERE id = $1`, commit.ProposalID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete proposal: %w", err)
	}
	if err := rowsAffectedErr(result); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := appendEventTx(ctx, tx, commit.Event); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit proposal approval: %w", err)
	}
	return nil
}

// SaveMission saves a mission to the database.
func (s *PostgresStore) SaveMission(m mission.Mission) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	missionJSON, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal mission: %w", err)
	}
	createdAt := m.Lifecycle.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}
	now := s.clock.Now()

	query := `
		INSERT INTO missions (
			ref, mission_id, tenant_id, state, version, principal_id, agent_id,
			parent_mission_ref, purpose, mission_json, created_at, updated_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	startTime := time.Now()
	defer s.logSlowQuery("SaveMission", startTime)

	_, err = s.db.ExecContext(ctx, query,
		m.MissionRef,
		m.MissionID,
		m.TenantID,
		string(m.State),
		m.Version,
		m.Principal.Subject,
		m.Agent.InstanceID,
		nullableString(m.Delegation.ParentMissionRef),
		m.Purpose.Objective,
		missionJSON,
		createdAt,
		now,
		nullableTime(m.Lifecycle.ExpiresAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert mission: %w", err)
	}

	return nil
}

// GetMission retrieves a mission by reference.
func (s *PostgresStore) GetMission(ref string) (mission.Mission, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetMission", startTime)

	var missionJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT mission_json FROM missions WHERE ref = $1`, ref).Scan(&missionJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.Mission{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.Mission{}, fmt.Errorf("query mission: %w", err)
	}

	var m mission.Mission
	if err := json.Unmarshal(missionJSON, &m); err != nil {
		return mission.Mission{}, fmt.Errorf("unmarshal mission: %w", err)
	}
	return m, nil
}

// UpdateMission updates an existing mission.
func (s *PostgresStore) UpdateMission(m mission.Mission) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	missionJSON, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal mission: %w", err)
	}

	query := `
		UPDATE missions SET
			state = $1,
			version = $2,
			principal_id = $3,
			agent_id = $4,
			parent_mission_ref = $5,
			purpose = $6,
			mission_json = $7,
			updated_at = $8,
			expires_at = $9
		WHERE ref = $10
	`

	startTime := time.Now()
	defer s.logSlowQuery("UpdateMission", startTime)

	result, err := s.db.ExecContext(ctx, query,
		string(m.State),
		m.Version,
		m.Principal.Subject,
		m.Agent.InstanceID,
		nullableString(m.Delegation.ParentMissionRef),
		m.Purpose.Objective,
		missionJSON,
		s.clock.Now(),
		nullableTime(m.Lifecycle.ExpiresAt),
		m.MissionRef,
	)
	if err != nil {
		return fmt.Errorf("update mission: %w", err)
	}

	return rowsAffectedErr(result)
}

// ChildrenOf retrieves all child missions for a given parent reference.
func (s *PostgresStore) ChildrenOf(parentRef string) ([]mission.Mission, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT mission_json
		FROM missions
		WHERE parent_mission_ref = $1
		ORDER BY created_at ASC, ref ASC
	`

	startTime := time.Now()
	defer s.logSlowQuery("ChildrenOf", startTime)

	rows, err := s.db.QueryContext(ctx, query, parentRef)
	if err != nil {
		return nil, fmt.Errorf("query children: %w", err)
	}
	defer rows.Close()

	children := make([]mission.Mission, 0)
	for rows.Next() {
		var missionJSON []byte
		if err := rows.Scan(&missionJSON); err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}
		var child mission.Mission
		if err := json.Unmarshal(missionJSON, &child); err != nil {
			return nil, fmt.Errorf("unmarshal child mission: %w", err)
		}
		children = append(children, child)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate children: %w", err)
	}
	return children, nil
}

// ListMissions lists stored missions.
func (s *PostgresStore) ListMissions() ([]mission.Mission, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListMissions", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT mission_json
		FROM missions
		ORDER BY created_at ASC, ref ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query missions: %w", err)
	}
	defer rows.Close()

	missions := make([]mission.Mission, 0)
	for rows.Next() {
		var missionJSON []byte
		if err := rows.Scan(&missionJSON); err != nil {
			return nil, fmt.Errorf("scan mission: %w", err)
		}
		var m mission.Mission
		if err := json.Unmarshal(missionJSON, &m); err != nil {
			return nil, fmt.Errorf("unmarshal mission: %w", err)
		}
		missions = append(missions, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate missions: %w", err)
	}
	return missions, nil
}

// SaveExpansionRequest saves a pending mission expansion request.
func (s *PostgresStore) SaveExpansionRequest(expansion mission.ExpansionRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	expansionJSON, err := json.Marshal(expansion)
	if err != nil {
		return fmt.Errorf("marshal expansion request: %w", err)
	}
	createdAt := expansion.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveExpansionRequest", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO expansion_requests (
			id, mission_ref, tenant_id, status, expansion_json, created_at, decided_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, expansion.ExpansionID, expansion.MissionRef, expansion.TenantID, expansion.Status, expansionJSON, createdAt, nullableTime(expansion.DecidedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert expansion request: %w", err)
	}
	return nil
}

// GetExpansionRequest retrieves a mission expansion request by ID.
func (s *PostgresStore) GetExpansionRequest(id string) (mission.ExpansionRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetExpansionRequest", startTime)

	var expansionJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT expansion_json FROM expansion_requests WHERE id = $1`, id).Scan(&expansionJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.ExpansionRequest{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.ExpansionRequest{}, fmt.Errorf("query expansion request: %w", err)
	}

	var expansion mission.ExpansionRequest
	if err := json.Unmarshal(expansionJSON, &expansion); err != nil {
		return mission.ExpansionRequest{}, fmt.Errorf("unmarshal expansion request: %w", err)
	}
	return expansion, nil
}

// UpdateExpansionRequest updates a mission expansion request.
func (s *PostgresStore) UpdateExpansionRequest(expansion mission.ExpansionRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	expansionJSON, err := json.Marshal(expansion)
	if err != nil {
		return fmt.Errorf("marshal expansion request: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("UpdateExpansionRequest", startTime)

	result, err := s.db.ExecContext(ctx, `
		UPDATE expansion_requests SET
			status = $1,
			expansion_json = $2,
			decided_at = $3
		WHERE id = $4
	`, expansion.Status, expansionJSON, nullableTime(expansion.DecidedAt), expansion.ExpansionID)
	if err != nil {
		return fmt.Errorf("update expansion request: %w", err)
	}
	return rowsAffectedErr(result)
}

// CommitExpansionDecision atomically updates an expansion, its mission authority, and its audit event.
func (s *PostgresStore) CommitExpansionDecision(ctx context.Context, commit mission.ExpansionDecisionCommit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	expansionJSON, err := json.Marshal(commit.Expansion)
	if err != nil {
		return fmt.Errorf("marshal expansion decision: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin expansion decision: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if commit.Mission != nil {
		missionJSON, err := json.Marshal(commit.Mission)
		if err != nil {
			return fmt.Errorf("marshal mission authority: %w", err)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE missions SET
				state = $1,
				version = $2,
				principal_id = $3,
				agent_id = $4,
				parent_mission_ref = $5,
				purpose = $6,
				mission_json = $7,
				updated_at = $8,
				expires_at = $9
			WHERE ref = $10 AND version = $11
		`, string(commit.Mission.State), commit.Mission.Version, commit.Mission.Principal.Subject,
			commit.Mission.Agent.InstanceID, nullableString(commit.Mission.Delegation.ParentMissionRef),
			commit.Mission.Purpose.Objective, missionJSON, s.clock.Now(), nullableTime(commit.Mission.Lifecycle.ExpiresAt),
			commit.Mission.MissionRef, commit.ExpectedMissionVersion)
		if err != nil {
			return fmt.Errorf("update mission authority: %w", err)
		}
		if err := requireOneAffected(result); err != nil {
			return err
		}
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE expansion_requests SET
			status = $1,
			expansion_json = $2,
			decided_at = $3
		WHERE id = $4 AND status = $5
	`, commit.Expansion.Status, expansionJSON, nullableTime(commit.Expansion.DecidedAt),
		commit.Expansion.ExpansionID, commit.ExpectedExpansionStatus)
	if err != nil {
		return fmt.Errorf("update expansion decision: %w", err)
	}
	if err := requireOneAffected(result); err != nil {
		return err
	}
	if err := appendEventTx(ctx, tx, commit.Event); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit expansion decision: %w", err)
	}
	return nil
}

// ListExpansionRequests lists mission expansion requests.
func (s *PostgresStore) ListExpansionRequests() ([]mission.ExpansionRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListExpansionRequests", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT expansion_json
		FROM expansion_requests
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query expansion requests: %w", err)
	}
	defer rows.Close()

	expansions := make([]mission.ExpansionRequest, 0)
	for rows.Next() {
		var expansionJSON []byte
		if err := rows.Scan(&expansionJSON); err != nil {
			return nil, fmt.Errorf("scan expansion request: %w", err)
		}
		var expansion mission.ExpansionRequest
		if err := json.Unmarshal(expansionJSON, &expansion); err != nil {
			return nil, fmt.Errorf("unmarshal expansion request: %w", err)
		}
		expansions = append(expansions, expansion)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expansion requests: %w", err)
	}
	return expansions, nil
}

// SaveEvaluationEvidence saves policy evidence for a decision artifact.
func (s *PostgresStore) SaveEvaluationEvidence(evidence mission.EvaluationEvidence) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("marshal evaluation evidence: %w", err)
	}
	createdAt := evidence.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveEvaluationEvidence", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evaluation_evidence (
			id, mission_ref, tenant_id, artifact, evidence_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, evidence.EvidenceID, evidence.MissionRef, nullableString(evidence.TenantID), evidence.Artifact, evidenceJSON, createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert evaluation evidence: %w", err)
	}
	return nil
}

// GetEvaluationEvidence retrieves policy evidence by ID.
func (s *PostgresStore) GetEvaluationEvidence(id string) (mission.EvaluationEvidence, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetEvaluationEvidence", startTime)

	var evidenceJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT evidence_json FROM evaluation_evidence WHERE id = $1`, id).Scan(&evidenceJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.EvaluationEvidence{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.EvaluationEvidence{}, fmt.Errorf("query evaluation evidence: %w", err)
	}

	var evidence mission.EvaluationEvidence
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil {
		return mission.EvaluationEvidence{}, fmt.Errorf("unmarshal evaluation evidence: %w", err)
	}
	return evidence, nil
}

// SaveToolContract saves a tool gateway contract.
func (s *PostgresStore) SaveToolContract(contract mission.ToolContract) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contractJSON, err := json.Marshal(contract)
	if err != nil {
		return fmt.Errorf("marshal tool contract: %w", err)
	}
	createdAt := contract.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveToolContract", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tool_contracts (
			tool_name, contract_json, created_at
		) VALUES ($1, $2, $3)
	`, contract.ToolName, contractJSON, createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert tool contract: %w", err)
	}
	return nil
}

// GetToolContract retrieves a tool gateway contract.
func (s *PostgresStore) GetToolContract(toolName string) (mission.ToolContract, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetToolContract", startTime)

	var contractJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT contract_json FROM tool_contracts WHERE tool_name = $1`, toolName).Scan(&contractJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.ToolContract{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.ToolContract{}, fmt.Errorf("query tool contract: %w", err)
	}

	var contract mission.ToolContract
	if err := json.Unmarshal(contractJSON, &contract); err != nil {
		return mission.ToolContract{}, fmt.Errorf("unmarshal tool contract: %w", err)
	}
	return contract, nil
}

// ListToolContracts lists tool gateway contracts.
func (s *PostgresStore) ListToolContracts() ([]mission.ToolContract, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListToolContracts", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT contract_json
		FROM tool_contracts
		ORDER BY created_at ASC, tool_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query tool contracts: %w", err)
	}
	defer rows.Close()

	contracts := make([]mission.ToolContract, 0)
	for rows.Next() {
		var contractJSON []byte
		if err := rows.Scan(&contractJSON); err != nil {
			return nil, fmt.Errorf("scan tool contract: %w", err)
		}
		var contract mission.ToolContract
		if err := json.Unmarshal(contractJSON, &contract); err != nil {
			return nil, fmt.Errorf("unmarshal tool contract: %w", err)
		}
		contracts = append(contracts, contract)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tool contracts: %w", err)
	}
	return contracts, nil
}

// SaveProjection saves a signed external authorization projection.
func (s *PostgresStore) SaveProjection(projection mission.Projection) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectionJSON, err := json.Marshal(projection)
	if err != nil {
		return fmt.Errorf("marshal projection: %w", err)
	}
	createdAt := projection.IssuedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveProjection", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO projections (
			id, mission_ref, tenant_id, status, projection_json, created_at, expires_at, revoked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, projection.ProjectionID, projection.MissionRef, nullableString(projection.TenantID), projection.Status, projectionJSON, createdAt, projection.ExpiresAt, nullableTime(projection.RevokedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert projection: %w", err)
	}
	return nil
}

// GetProjection retrieves a signed external authorization projection.
func (s *PostgresStore) GetProjection(id string) (mission.Projection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetProjection", startTime)

	var projectionJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT projection_json FROM projections WHERE id = $1`, id).Scan(&projectionJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.Projection{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.Projection{}, fmt.Errorf("query projection: %w", err)
	}
	var projection mission.Projection
	if err := json.Unmarshal(projectionJSON, &projection); err != nil {
		return mission.Projection{}, fmt.Errorf("unmarshal projection: %w", err)
	}
	return projection, nil
}

// UpdateProjection updates projection status.
func (s *PostgresStore) UpdateProjection(projection mission.Projection) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectionJSON, err := json.Marshal(projection)
	if err != nil {
		return fmt.Errorf("marshal projection: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("UpdateProjection", startTime)

	result, err := s.db.ExecContext(ctx, `
		UPDATE projections SET
			status = $1,
			projection_json = $2,
			expires_at = $3,
			revoked_at = $4
		WHERE id = $5
	`, projection.Status, projectionJSON, projection.ExpiresAt, nullableTime(projection.RevokedAt), projection.ProjectionID)
	if err != nil {
		return fmt.Errorf("update projection: %w", err)
	}
	return rowsAffectedErr(result)
}

// ListProjections lists signed external authorization projections.
func (s *PostgresStore) ListProjections() ([]mission.Projection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListProjections", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT projection_json
		FROM projections
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query projections: %w", err)
	}
	defer rows.Close()

	projections := make([]mission.Projection, 0)
	for rows.Next() {
		var projectionJSON []byte
		if err := rows.Scan(&projectionJSON); err != nil {
			return nil, fmt.Errorf("scan projection: %w", err)
		}
		var projection mission.Projection
		if err := json.Unmarshal(projectionJSON, &projection); err != nil {
			return nil, fmt.Errorf("unmarshal projection: %w", err)
		}
		projections = append(projections, projection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projections: %w", err)
	}
	return projections, nil
}

// SaveMissionLease saves a mission lease.
func (s *PostgresStore) SaveMissionLease(lease mission.MissionLease) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	leaseJSON, err := json.Marshal(lease)
	if err != nil {
		return fmt.Errorf("marshal mission lease: %w", err)
	}
	createdAt := lease.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveMissionLease", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mission_leases (
			id, mission_ref, tenant_id, mission_version, lease_json, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, lease.LeaseID, lease.MissionRef, nullableString(lease.TenantID), lease.MissionVersion, leaseJSON, createdAt, lease.ExpiresAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert mission lease: %w", err)
	}
	return nil
}

// GetMissionLease retrieves a mission lease by ID.
func (s *PostgresStore) GetMissionLease(id string) (mission.MissionLease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetMissionLease", startTime)

	var leaseJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT lease_json FROM mission_leases WHERE id = $1`, id).Scan(&leaseJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.MissionLease{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.MissionLease{}, fmt.Errorf("query mission lease: %w", err)
	}
	var lease mission.MissionLease
	if err := json.Unmarshal(leaseJSON, &lease); err != nil {
		return mission.MissionLease{}, fmt.Errorf("unmarshal mission lease: %w", err)
	}
	return lease, nil
}

// UpdateMissionLease updates an existing mission lease.
func (s *PostgresStore) UpdateMissionLease(lease mission.MissionLease) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	leaseJSON, err := json.Marshal(lease)
	if err != nil {
		return fmt.Errorf("marshal mission lease: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("UpdateMissionLease", startTime)

	result, err := s.db.ExecContext(ctx, `
		UPDATE mission_leases SET
			mission_version = $1,
			lease_json = $2,
			expires_at = $3
		WHERE id = $4
	`, lease.MissionVersion, leaseJSON, lease.ExpiresAt, lease.LeaseID)
	if err != nil {
		return fmt.Errorf("update mission lease: %w", err)
	}
	return rowsAffectedErr(result)
}

// ListMissionLeases lists mission leases.
func (s *PostgresStore) ListMissionLeases() ([]mission.MissionLease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListMissionLeases", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT lease_json
		FROM mission_leases
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query mission leases: %w", err)
	}
	defer rows.Close()

	leases := make([]mission.MissionLease, 0)
	for rows.Next() {
		var leaseJSON []byte
		if err := rows.Scan(&leaseJSON); err != nil {
			return nil, fmt.Errorf("scan mission lease: %w", err)
		}
		var lease mission.MissionLease
		if err := json.Unmarshal(leaseJSON, &lease); err != nil {
			return nil, fmt.Errorf("unmarshal mission lease: %w", err)
		}
		leases = append(leases, lease)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mission leases: %w", err)
	}
	return leases, nil
}

// SaveApprovalRule saves an approval policy rule.
func (s *PostgresStore) SaveApprovalRule(rule mission.ApprovalRule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal approval rule: %w", err)
	}
	createdAt := rule.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveApprovalRule", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO approval_rules (
			id, tenant_id, applies_to, rule_json, created_at
		) VALUES ($1, $2, $3, $4, $5)
	`, rule.RuleID, nullableString(rule.TenantID), rule.AppliesTo, ruleJSON, createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert approval rule: %w", err)
	}
	return nil
}

// ListApprovalRules lists approval policy rules.
func (s *PostgresStore) ListApprovalRules() ([]mission.ApprovalRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListApprovalRules", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT rule_json
		FROM approval_rules
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query approval rules: %w", err)
	}
	defer rows.Close()

	rules := make([]mission.ApprovalRule, 0)
	for rows.Next() {
		var ruleJSON []byte
		if err := rows.Scan(&ruleJSON); err != nil {
			return nil, fmt.Errorf("scan approval rule: %w", err)
		}
		var rule mission.ApprovalRule
		if err := json.Unmarshal(ruleJSON, &rule); err != nil {
			return nil, fmt.Errorf("unmarshal approval rule: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approval rules: %w", err)
	}
	return rules, nil
}

// SaveApprovalRecord saves an approval submission.
func (s *PostgresStore) SaveApprovalRecord(record mission.ApprovalRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recordJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal approval record: %w", err)
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveApprovalRecord", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO approval_records (
			id, target_type, target_id, tenant_id, approver_subject, approver_issuer, record_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, record.ApprovalID, record.TargetType, record.TargetID, nullableString(record.TenantID), record.Approver.Subject, record.Approver.Issuer, recordJSON, createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert approval record: %w", err)
	}
	return nil
}

// ListApprovalRecords lists approval submissions for a target.
func (s *PostgresStore) ListApprovalRecords(targetType string, targetID string) ([]mission.ApprovalRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListApprovalRecords", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT record_json
		FROM approval_records
		WHERE target_type = $1 AND target_id = $2
		ORDER BY created_at ASC, id ASC
	`, targetType, targetID)
	if err != nil {
		return nil, fmt.Errorf("query approval records: %w", err)
	}
	defer rows.Close()

	records := make([]mission.ApprovalRecord, 0)
	for rows.Next() {
		var recordJSON []byte
		if err := rows.Scan(&recordJSON); err != nil {
			return nil, fmt.Errorf("scan approval record: %w", err)
		}
		var record mission.ApprovalRecord
		if err := json.Unmarshal(recordJSON, &record); err != nil {
			return nil, fmt.Errorf("unmarshal approval record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approval records: %w", err)
	}
	return records, nil
}

// SaveAuthorityNegotiation saves an authority negotiation record.
func (s *PostgresStore) SaveAuthorityNegotiation(negotiation mission.AuthorityNegotiation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	negotiationJSON, err := json.Marshal(negotiation)
	if err != nil {
		return fmt.Errorf("marshal authority negotiation: %w", err)
	}
	createdAt := negotiation.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveAuthorityNegotiation", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO authority_negotiations (
			id, mission_ref, tenant_id, status, negotiation_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, negotiation.NegotiationID, negotiation.MissionRef, nullableString(negotiation.TenantID), negotiation.Status, negotiationJSON, createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert authority negotiation: %w", err)
	}
	return nil
}

// GetAuthorityNegotiation retrieves an authority negotiation record.
func (s *PostgresStore) GetAuthorityNegotiation(id string) (mission.AuthorityNegotiation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetAuthorityNegotiation", startTime)

	var negotiationJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT negotiation_json FROM authority_negotiations WHERE id = $1`, id).Scan(&negotiationJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.AuthorityNegotiation{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.AuthorityNegotiation{}, fmt.Errorf("query authority negotiation: %w", err)
	}
	var negotiation mission.AuthorityNegotiation
	if err := json.Unmarshal(negotiationJSON, &negotiation); err != nil {
		return mission.AuthorityNegotiation{}, fmt.Errorf("unmarshal authority negotiation: %w", err)
	}
	return negotiation, nil
}

// SaveContainmentRule saves an active containment rule.
func (s *PostgresStore) SaveContainmentRule(rule mission.ContainmentRule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal containment rule: %w", err)
	}
	createdAt := rule.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveContainmentRule", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO containment_rules (
			id, tenant_id, target_type, target_id, status, rule_json, created_at, expires_at, lifted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, rule.RuleID, nullableString(rule.TenantID), rule.TargetType, rule.TargetID, rule.Status, ruleJSON, createdAt, nullableTime(rule.ExpiresAt), nullableTime(rule.LiftedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert containment rule: %w", err)
	}
	return nil
}

// GetContainmentRule retrieves a containment rule.
func (s *PostgresStore) GetContainmentRule(id string) (mission.ContainmentRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetContainmentRule", startTime)

	var ruleJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT rule_json FROM containment_rules WHERE id = $1`, id).Scan(&ruleJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.ContainmentRule{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.ContainmentRule{}, fmt.Errorf("query containment rule: %w", err)
	}
	var rule mission.ContainmentRule
	if err := json.Unmarshal(ruleJSON, &rule); err != nil {
		return mission.ContainmentRule{}, fmt.Errorf("unmarshal containment rule: %w", err)
	}
	return rule, nil
}

// UpdateContainmentRule updates a containment rule.
func (s *PostgresStore) UpdateContainmentRule(rule mission.ContainmentRule) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ruleJSON, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("marshal containment rule: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("UpdateContainmentRule", startTime)

	result, err := s.db.ExecContext(ctx, `
		UPDATE containment_rules SET
			status = $1,
			rule_json = $2,
			expires_at = $3,
			lifted_at = $4
		WHERE id = $5
	`, rule.Status, ruleJSON, nullableTime(rule.ExpiresAt), nullableTime(rule.LiftedAt), rule.RuleID)
	if err != nil {
		return fmt.Errorf("update containment rule: %w", err)
	}
	return rowsAffectedErr(result)
}

// ListContainmentRules lists containment rules.
func (s *PostgresStore) ListContainmentRules() ([]mission.ContainmentRule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListContainmentRules", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT rule_json
		FROM containment_rules
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query containment rules: %w", err)
	}
	defer rows.Close()

	rules := make([]mission.ContainmentRule, 0)
	for rows.Next() {
		var ruleJSON []byte
		if err := rows.Scan(&ruleJSON); err != nil {
			return nil, fmt.Errorf("scan containment rule: %w", err)
		}
		var rule mission.ContainmentRule
		if err := json.Unmarshal(ruleJSON, &rule); err != nil {
			return nil, fmt.Errorf("unmarshal containment rule: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate containment rules: %w", err)
	}
	return rules, nil
}

// SaveGitHubRepositoryBinding saves a GitHub repository integration binding.
func (s *PostgresStore) SaveGitHubRepositoryBinding(binding mission.GitHubRepositoryBinding) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bindingJSON, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("marshal github repository binding: %w", err)
	}
	createdAt := binding.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveGitHubRepositoryBinding", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO github_repository_bindings (
			id, tenant_id, repository, status, binding_json, created_at, last_webhook_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, binding.BindingID, nullableString(binding.TenantID), binding.Repository, binding.Status, bindingJSON, createdAt, nullableTime(binding.LastWebhookAt))
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert github repository binding: %w", err)
	}
	return nil
}

// GetGitHubRepositoryBinding retrieves a GitHub repository integration binding.
func (s *PostgresStore) GetGitHubRepositoryBinding(id string) (mission.GitHubRepositoryBinding, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetGitHubRepositoryBinding", startTime)

	var bindingJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT binding_json FROM github_repository_bindings WHERE id = $1`, id).Scan(&bindingJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.GitHubRepositoryBinding{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.GitHubRepositoryBinding{}, fmt.Errorf("query github repository binding: %w", err)
	}
	var binding mission.GitHubRepositoryBinding
	if err := json.Unmarshal(bindingJSON, &binding); err != nil {
		return mission.GitHubRepositoryBinding{}, fmt.Errorf("unmarshal github repository binding: %w", err)
	}
	return binding, nil
}

// UpdateGitHubRepositoryBinding updates a GitHub repository integration binding.
func (s *PostgresStore) UpdateGitHubRepositoryBinding(binding mission.GitHubRepositoryBinding) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bindingJSON, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("marshal github repository binding: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("UpdateGitHubRepositoryBinding", startTime)

	result, err := s.db.ExecContext(ctx, `
		UPDATE github_repository_bindings SET
			tenant_id = $1,
			repository = $2,
			status = $3,
			binding_json = $4,
			last_webhook_at = $5
		WHERE id = $6
	`, nullableString(binding.TenantID), binding.Repository, binding.Status, bindingJSON, nullableTime(binding.LastWebhookAt), binding.BindingID)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("update github repository binding: %w", err)
	}
	return rowsAffectedErr(result)
}

// ListGitHubRepositoryBindings lists GitHub repository integration bindings.
func (s *PostgresStore) ListGitHubRepositoryBindings() ([]mission.GitHubRepositoryBinding, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListGitHubRepositoryBindings", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT binding_json
		FROM github_repository_bindings
		ORDER BY repository ASC, created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query github repository bindings: %w", err)
	}
	defer rows.Close()

	bindings := make([]mission.GitHubRepositoryBinding, 0)
	for rows.Next() {
		var bindingJSON []byte
		if err := rows.Scan(&bindingJSON); err != nil {
			return nil, fmt.Errorf("scan github repository binding: %w", err)
		}
		var binding mission.GitHubRepositoryBinding
		if err := json.Unmarshal(bindingJSON, &binding); err != nil {
			return nil, fmt.Errorf("unmarshal github repository binding: %w", err)
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate github repository bindings: %w", err)
	}
	return bindings, nil
}

// SaveGitHubWebhookDelivery saves a processed GitHub webhook delivery.
func (s *PostgresStore) SaveGitHubWebhookDelivery(delivery mission.GitHubWebhookDelivery) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deliveryJSON, err := json.Marshal(delivery)
	if err != nil {
		return fmt.Errorf("marshal github webhook delivery: %w", err)
	}
	payloadSummary := delivery.PayloadSummary
	if payloadSummary == nil {
		payloadSummary = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payloadSummary)
	if err != nil {
		return fmt.Errorf("marshal github webhook summary: %w", err)
	}
	receivedAt := delivery.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveGitHubWebhookDelivery", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO github_webhook_deliveries (
			delivery_id, event, repository, binding_id, tenant_id, mission_ref, status,
			delivery_json, payload_summary, received_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, delivery.DeliveryID, delivery.Event, nullableString(delivery.Repository), nullableString(delivery.BindingID), nullableString(delivery.TenantID), nullableString(delivery.MissionRef), delivery.Status, deliveryJSON, payloadJSON, receivedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert github webhook delivery: %w", err)
	}
	return nil
}

// GetGitHubWebhookDelivery retrieves a processed GitHub webhook delivery.
func (s *PostgresStore) GetGitHubWebhookDelivery(id string) (mission.GitHubWebhookDelivery, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetGitHubWebhookDelivery", startTime)

	var deliveryJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT delivery_json FROM github_webhook_deliveries WHERE delivery_id = $1`, id).Scan(&deliveryJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.GitHubWebhookDelivery{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.GitHubWebhookDelivery{}, fmt.Errorf("query github webhook delivery: %w", err)
	}
	var delivery mission.GitHubWebhookDelivery
	if err := json.Unmarshal(deliveryJSON, &delivery); err != nil {
		return mission.GitHubWebhookDelivery{}, fmt.Errorf("unmarshal github webhook delivery: %w", err)
	}
	return delivery, nil
}

// SaveOktaAppBinding saves an Okta application and group mission binding.
func (s *PostgresStore) SaveOktaAppBinding(binding mission.OktaAppBinding) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bindingJSON, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("marshal okta app binding: %w", err)
	}
	createdAt := binding.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.clock.Now()
	}

	startTime := time.Now()
	defer s.logSlowQuery("SaveOktaAppBinding", startTime)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO okta_app_bindings (
			id, tenant_id, issuer, client_id, mission_ref, status, binding_json, created_at, last_resolved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, binding.BindingID, nullableString(binding.TenantID), binding.Issuer, binding.ClientID, binding.MissionRef, binding.Status, bindingJSON, createdAt, nullableTime(binding.LastResolvedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("insert okta app binding: %w", err)
	}
	return nil
}

// GetOktaAppBinding retrieves an Okta application and group mission binding.
func (s *PostgresStore) GetOktaAppBinding(id string) (mission.OktaAppBinding, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("GetOktaAppBinding", startTime)

	var bindingJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT binding_json FROM okta_app_bindings WHERE id = $1`, id).Scan(&bindingJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.OktaAppBinding{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.OktaAppBinding{}, fmt.Errorf("query okta app binding: %w", err)
	}
	var binding mission.OktaAppBinding
	if err := json.Unmarshal(bindingJSON, &binding); err != nil {
		return mission.OktaAppBinding{}, fmt.Errorf("unmarshal okta app binding: %w", err)
	}
	return binding, nil
}

// UpdateOktaAppBinding updates an Okta application and group mission binding.
func (s *PostgresStore) UpdateOktaAppBinding(binding mission.OktaAppBinding) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bindingJSON, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("marshal okta app binding: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("UpdateOktaAppBinding", startTime)

	result, err := s.db.ExecContext(ctx, `
		UPDATE okta_app_bindings SET
			tenant_id = $1,
			issuer = $2,
			client_id = $3,
			mission_ref = $4,
			status = $5,
			binding_json = $6,
			last_resolved_at = $7
		WHERE id = $8
	`, nullableString(binding.TenantID), binding.Issuer, binding.ClientID, binding.MissionRef, binding.Status, bindingJSON, nullableTime(binding.LastResolvedAt), binding.BindingID)
	if err != nil {
		if isUniqueViolation(err) {
			return mission.ErrConflict
		}
		return fmt.Errorf("update okta app binding: %w", err)
	}
	return rowsAffectedErr(result)
}

// ListOktaAppBindings lists Okta application and group mission bindings.
func (s *PostgresStore) ListOktaAppBindings() ([]mission.OktaAppBinding, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("ListOktaAppBindings", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT binding_json
		FROM okta_app_bindings
		ORDER BY issuer ASC, client_id ASC, created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query okta app bindings: %w", err)
	}
	defer rows.Close()

	bindings := make([]mission.OktaAppBinding, 0)
	for rows.Next() {
		var bindingJSON []byte
		if err := rows.Scan(&bindingJSON); err != nil {
			return nil, fmt.Errorf("scan okta app binding: %w", err)
		}
		var binding mission.OktaAppBinding
		if err := json.Unmarshal(bindingJSON, &binding); err != nil {
			return nil, fmt.Errorf("unmarshal okta app binding: %w", err)
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate okta app bindings: %w", err)
	}
	return bindings, nil
}

// AppendEvent appends an event and stages it in the outbox in one transaction.
func (s *PostgresStore) AppendEvent(event mission.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("AppendEvent", startTime)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin append event: %w", err)
	}

	if err := appendEventTx(ctx, tx, event); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit append event: %w", err)
	}
	return nil
}

func appendEventTx(ctx context.Context, tx *sql.Tx, event mission.Event) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO events (id, type, mission_ref, tenant_id, event_json, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, event.EventID, event.Type, nullableString(event.MissionRef), nullableString(event.TenantID), eventJSON, payloadJSON, event.OccurredAt); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, type, mission_ref, event_json, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, event.EventID, event.Type, nullableString(event.MissionRef), eventJSON, payloadJSON, event.OccurredAt); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func requireOneAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if affected != 1 {
		return mission.ErrConflict
	}
	return nil
}

// Events retrieves all events from the database.
func (s *PostgresStore) Events() []mission.Event {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	defer s.logSlowQuery("Events", startTime)

	rows, err := s.db.QueryContext(ctx, `
		SELECT event_json
		FROM events
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		s.logger.Error("failed to query events", "error", err)
		return []mission.Event{}
	}
	defer rows.Close()

	events := make([]mission.Event, 0)
	for rows.Next() {
		var eventJSON []byte
		if err := rows.Scan(&eventJSON); err != nil {
			s.logger.Error("failed to scan event", "error", err)
			continue
		}
		var event mission.Event
		if err := json.Unmarshal(eventJSON, &event); err != nil {
			s.logger.Error("failed to unmarshal event", "error", err)
			continue
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		s.logger.Error("error iterating events", "error", err)
	}
	return events
}

// PublishOutboxEvents marks unprocessed outbox events processed and returns them.
func (s *PostgresStore) PublishOutboxEvents() ([]mission.OutboxEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		WITH pending AS (
			SELECT id
			FROM outbox_events
			WHERE processed_at IS NULL
			ORDER BY created_at ASC, id ASC
			LIMIT 100
			FOR UPDATE SKIP LOCKED
		),
		updated AS (
			UPDATE outbox_events o
			SET processed_at = NOW()
			FROM pending p
			WHERE o.id = p.id
			RETURNING o.id, o.type, o.mission_ref, o.payload, o.created_at
		),
		recorded AS (
			INSERT INTO outbox_events_processed (event_id, processed_at)
			SELECT id, NOW()
			FROM updated
			ON CONFLICT (event_id) DO NOTHING
			RETURNING event_id
		)
		SELECT id, type, mission_ref, payload, created_at
		FROM updated
		ORDER BY created_at ASC, id ASC
	`

	startTime := time.Now()
	defer s.logSlowQuery("PublishOutboxEvents", startTime)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query outbox: %w", err)
	}
	defer rows.Close()

	events := make([]mission.OutboxEvent, 0)
	for rows.Next() {
		var event mission.OutboxEvent
		var missionRef sql.NullString
		var payloadJSON []byte
		if err := rows.Scan(&event.ID, &event.Type, &missionRef, &payloadJSON, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		event.MissionRef = missionRef.String
		if err := json.Unmarshal(payloadJSON, &event.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal outbox payload: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox: %w", err)
	}
	return events, nil
}

func (s *PostgresStore) logSlowQuery(name string, start time.Time) {
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		s.logger.Warn("slow query", "query", name, "duration_ms", elapsed.Milliseconds())
	}
}

func rowsAffectedErr(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return mission.ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pq.Error
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: !value.IsZero()}
}
