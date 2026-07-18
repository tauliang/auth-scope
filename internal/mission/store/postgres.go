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

// AppendEvent appends an event and stages it in the outbox in one transaction.
func (s *PostgresStore) AppendEvent(event mission.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	startTime := time.Now()
	defer s.logSlowQuery("AppendEvent", startTime)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin append event: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO events (id, type, mission_ref, tenant_id, event_json, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, event.EventID, event.Type, nullableString(event.MissionRef), nullableString(event.TenantID), eventJSON, payloadJSON, event.OccurredAt)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert event: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, type, mission_ref, event_json, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, event.EventID, event.Type, nullableString(event.MissionRef), eventJSON, payloadJSON, event.OccurredAt)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit append event: %w", err)
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
