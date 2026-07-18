package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lib/pq"
	"github.com/tauliang/auth-scope/internal/mission"
)

// PostgresStore implements the Store interface using PostgreSQL.
type PostgresStore struct {
	db      *sql.DB
	clock   mission.Clock
	logger  *slog.Logger
}

// NewPostgresStore creates a new PostgresStore instance.
func NewPostgresStore(db *sql.DB, clock mission.Clock) (*PostgresStore, error) {
	if db == nil {
		return nil, errors.New("db cannot be nil")
	}
	if clock == nil {
		clock = mission.SystemClock{}
	}

	logger := slog.Default().With("component", "postgres-store")

	return &PostgresStore{
		db:      db,
		clock:   clock,
		logger:  logger,
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
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return NewPostgresStore(db, mission.SystemClock{})
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

	regionsJSON, err := json.Marshal(proposal.AuthorityRegion)
	if err != nil {
		return fmt.Errorf("marshal authority_region: %w", err)
	}

	query := `
		INSERT INTO mission_proposals (
			id, principal_id, agent_id, purpose, regions,
			status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "SaveProposal", "duration_ms", elapsed.Milliseconds())
		}
	}()

	_, err = s.db.ExecContext(ctx, query,
		proposal.ProposalID,
		proposal.Principal.Subject,
		proposal.Agent.ClientID,
		proposal.Intent.Objective,
		regionsJSON,
		string(proposal.Status),
		proposal.CreatedAt,
	)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
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

	query := `
		SELECT id, principal_id, agent_id, purpose, regions,
		       status, created_at, approved_at, rejected_at
		FROM mission_proposals WHERE id = $1
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "GetProposal", "duration_ms", elapsed.Milliseconds())
		}
	}()

	var proposal mission.MissionProposal
	var principalID, agentID, purpose string
	var regionsJSON []byte
	var statusStr string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&proposal.ProposalID,
		&principalID,
		&agentID,
		&purpose,
		&regionsJSON,
		&statusStr,
		&proposal.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.MissionProposal{}, fmt.Errorf("query proposal: %w", err)
	}

	var regions []mission.ResourceGrant
	if err := json.Unmarshal(regionsJSON, &regions); err != nil {
		return mission.MissionProposal{}, fmt.Errorf("unmarshal authority_region: %w", err)
	}

	proposal.Principal = mission.Principal{Subject: principalID}
	proposal.Agent = mission.Agent{ClientID: agentID}
	proposal.Intent.Objective = purpose
	proposal.AuthorityRegion.Resources = regions

	return proposal, nil
}

// DeleteProposal deletes a mission proposal by ID.
func (s *PostgresStore) DeleteProposal(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `DELETE FROM mission_proposals WHERE id = $1`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "DeleteProposal", "duration_ms", elapsed.Milliseconds())
		}
	}()

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete proposal: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return mission.ErrNotFound
	}

	return nil
}

// SaveMission saves a mission to the database.
func (s *PostgresStore) SaveMission(m mission.Mission) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	regionsJSON, err := json.Marshal(m.AuthorityRegion)
	if err != nil {
		return fmt.Errorf("marshal authority_region: %w", err)
	}

	query := `
		INSERT INTO missions (
			ref, proposal_id, state, principal_id, agent_id,
			purpose, regions, current_decision, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "SaveMission", "duration_ms", elapsed.Milliseconds())
		}
	}()

	_, err = s.db.ExecContext(ctx, query,
		m.MissionRef,
		nil, // proposal_id will be set later if needed
		string(m.State),
		m.Principal.Subject,
		m.Agent.ClientID,
		m.Purpose.Objective,
		regionsJSON,
		nil, // current_decision
		m.Lifecycle.CreatedAt,
		s.clock.Now(),
	)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
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

	query := `
		SELECT ref, proposal_id, state, principal_id, agent_id,
		       purpose, regions, current_decision, created_at, updated_at
		FROM missions WHERE ref = $1
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "GetMission", "duration_ms", elapsed.Milliseconds())
		}
	}()

	var m mission.Mission
	var principalID, agentID, purpose string
	var regionsJSON []byte
	var stateStr string

	err := s.db.QueryRowContext(ctx, query, ref).Scan(
		&m.MissionRef,
		nil, // proposal_id (ignored for now)
		&stateStr,
		&principalID,
		&agentID,
		&purpose,
		&regionsJSON,
		nil, // current_decision (ignored for now)
		&m.Lifecycle.CreatedAt,
		&m.Lifecycle.ExpiresAt, // Will be overwritten below
	)
	if errors.Is(err, sql.ErrNoRows) {
		return mission.Mission{}, mission.ErrNotFound
	}
	if err != nil {
		return mission.Mission{}, fmt.Errorf("query mission: %w", err)
	}

	var regions []mission.ResourceGrant
	if err := json.Unmarshal(regionsJSON, &regions); err != nil {
		return mission.Mission{}, fmt.Errorf("unmarshal authority_region: %w", err)
	}

	m.State = mission.State(stateStr)
	m.Principal = mission.Principal{Subject: principalID}
	m.Agent = mission.Agent{ClientID: agentID}
	m.Purpose.Objective = purpose
	m.AuthorityRegion.Resources = regions

	return m, nil
}

// UpdateMission updates an existing mission.
func (s *PostgresStore) UpdateMission(m mission.Mission) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	regionsJSON, err := json.Marshal(m.AuthorityRegion)
	if err != nil {
		return fmt.Errorf("marshal authority_region: %w", err)
	}

	query := `
		UPDATE missions SET
			state = $1,
			principal_id = $2,
			agent_id = $3,
			purpose = $4,
			regions = $5,
			current_decision = $6,
			updated_at = $7
		WHERE ref = $8
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "UpdateMission", "duration_ms", elapsed.Milliseconds())
		}
	}()

	result, err := s.db.ExecContext(ctx, query,
		string(m.State),
		m.Principal.Subject,
		m.Agent.ClientID,
		m.Purpose.Objective,
		regionsJSON,
		nil, // current_decision
		s.clock.Now(),
		m.MissionRef,
	)
	if err != nil {
		return fmt.Errorf("update mission: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return mission.ErrNotFound
	}

	return nil
}

// ChildrenOf retrieves all child missions for a given parent reference.
func (s *PostgresStore) ChildrenOf(parentRef string) ([]mission.Mission, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT ref, state, principal_id, agent_id,
		       purpose, regions, created_at
		FROM missions WHERE delegation->>'parent_mission_ref' = $1
		ORDER BY created_at ASC
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "ChildrenOf", "duration_ms", elapsed.Milliseconds())
		}
	}()

	rows, err := s.db.QueryContext(ctx, query, parentRef)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, mission.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query children: %w", err)
	}
	defer rows.Close()

	var children []mission.Mission
	for rows.Next() {
		var m mission.Mission
		var principalID, agentID, purpose string
		var regionsJSON []byte
		var stateStr string

		err := rows.Scan(
			&m.MissionRef,
			&stateStr,
			&principalID,
			&agentID,
			&purpose,
			&regionsJSON,
			&m.Lifecycle.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}

		var regions []mission.ResourceGrant
		if err := json.Unmarshal(regionsJSON, &regions); err != nil {
			return nil, fmt.Errorf("unmarshal authority_region: %w", err)
		}

		m.State = mission.State(stateStr)
		m.Principal = mission.Principal{Subject: principalID}
		m.Agent = mission.Agent{ClientID: agentID}
		m.Purpose.Objective = purpose
		m.AuthorityRegion.Resources = regions

		children = append(children, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate children: %w", err)
	}

	return children, nil
}

// AppendEvent appends an event to the events table.
func (s *PostgresStore) AppendEvent(event mission.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	query := `
		INSERT INTO events (id, type, mission_ref, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "AppendEvent", "duration_ms", elapsed.Milliseconds())
		}
	}()

	_, err = s.db.ExecContext(ctx, query,
		event.EventID,
		event.Type,
		event.MissionRef,
		payloadJSON,
		event.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

// Events retrieves all events from the database.
func (s *PostgresStore) Events() []mission.Event {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, type, mission_ref, payload, created_at
		FROM events ORDER BY created_at ASC
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "Events", "duration_ms", elapsed.Milliseconds())
		}
	}()

	rows, err := s.db.QueryContext(ctx, query)
	if errors.Is(err, sql.ErrNoRows) {
		return []mission.Event{}
	}
	if err != nil {
		s.logger.Error("failed to query events", "error", err)
		return []mission.Event{}
	}
	defer rows.Close()

	var events []mission.Event
	for rows.Next() {
		var e mission.Event
		var payloadJSON []byte

		err := rows.Scan(
			&e.EventID,
			&e.Type,
			&e.MissionRef,
			&payloadJSON,
			&e.OccurredAt,
		)
		if err != nil {
			s.logger.Error("failed to scan event", "error", err)
			continue
		}

		if err := json.Unmarshal(payloadJSON, &e.Payload); err != nil {
			s.logger.Error("failed to unmarshal payload", "error", err)
			continue
		}

		events = append(events, e)
	}

	return events
}



// PublishOutboxEvents processes unprocessed outbox events.
func (s *PostgresStore) PublishOutboxEvents() ([]mission.OutboxEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO outbox_events_processed (event_id, processed_at)
		SELECT id, NOW()
		FROM outbox_events
		WHERE processed_at IS NULL
		ORDER BY created_at ASC
		LIMIT 100
		RETURNING id, type, mission_ref, payload, created_at
	`

	startTime := time.Now()
	defer func() {
		if elapsed := time.Since(startTime); elapsed > 100*time.Millisecond {
			s.logger.Warn("slow query", "query", "PublishOutboxEvents", "duration_ms", elapsed.Milliseconds())
		}
	}()

	rows, err := s.db.QueryContext(ctx, query)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		s.logger.Error("failed to query outbox events", "error", err)
		return nil, fmt.Errorf("query outbox: %w", err)
	}
	defer rows.Close()

	var events []mission.OutboxEvent
	for rows.Next() {
		var e mission.OutboxEvent
		var payloadJSON []byte

		err := rows.Scan(
			&e.ID,
			&e.Type,
			&e.MissionRef,
			&payloadJSON,
			&e.CreatedAt,
		)
		if err != nil {
			s.logger.Error("failed to scan outbox event", "error", err)
			continue
		}

		if err := json.Unmarshal(payloadJSON, &e.Payload); err != nil {
			s.logger.Error("failed to unmarshal outbox payload", "error", err)
			continue
		}

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("error iterating outbox events", "error", err)
		return nil, fmt.Errorf("iterate outbox: %w", err)
	}

	return events, nil
}
