package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/tauliang/auth-scope/internal/mission"
)

func (s *PostgresStore) ListActiveContainmentRules(parent context.Context, at time.Time) ([]mission.ContainmentRule, error) {
	ctx, cancel := readContext(parent)
	defer cancel()
	return queryJSONRows[mission.ContainmentRule](ctx, s.db, `
		SELECT rule_json FROM containment_rules
		WHERE status = $1 AND (expires_at IS NULL OR expires_at >= $2)
		ORDER BY created_at, id
	`, mission.ContainmentStatusActive, at)
}

func (s *PostgresStore) LoadBlastRadiusSnapshot(parent context.Context, rule mission.ContainmentRule) (mission.GovernanceSnapshot, error) {
	ctx, cancel := readContext(parent)
	defer cancel()
	tenantID := rule.TenantID
	if tenantID == "" && rule.TargetType == mission.ContainmentTargetTenant {
		tenantID = rule.TargetID
	}

	missions, err := queryTenantJSON[mission.Mission](ctx, s.db, "missions", "mission_json", "created_at, ref", tenantID)
	if err != nil {
		return mission.GovernanceSnapshot{}, err
	}
	projections, err := queryTenantJSON[mission.Projection](ctx, s.db, "projections", "projection_json", "created_at, id", tenantID)
	if err != nil {
		return mission.GovernanceSnapshot{}, err
	}
	leases, err := queryTenantJSON[mission.MissionLease](ctx, s.db, "mission_leases", "lease_json", "created_at, id", tenantID)
	if err != nil {
		return mission.GovernanceSnapshot{}, err
	}
	expansions, err := queryTenantJSON[mission.ExpansionRequest](ctx, s.db, "expansion_requests", "expansion_json", "created_at, id", tenantID)
	if err != nil {
		return mission.GovernanceSnapshot{}, err
	}
	agents, err := queryTenantJSON[mission.AgentIdentity](ctx, s.db, "agent_identities", "identity_json", "created_at, id", tenantID)
	if err != nil {
		return mission.GovernanceSnapshot{}, err
	}
	contracts, err := queryJSONRows[mission.ToolContract](ctx, s.db, `SELECT contract_json FROM tool_contracts ORDER BY created_at, tool_name`)
	if err != nil {
		return mission.GovernanceSnapshot{}, err
	}
	return mission.GovernanceSnapshot{
		Missions:          missions,
		Projections:       projections,
		Leases:            leases,
		ExpansionRequests: expansions,
		Agents:            agents,
		ToolContracts:     contracts,
	}, nil
}

func (s *PostgresStore) LoadMissionLineageSnapshot(parent context.Context, ref string) (mission.LineageSnapshot, error) {
	ctx, cancel := readContext(parent)
	defer cancel()
	missions, err := queryJSONRows[mission.Mission](ctx, s.db, `
		WITH RECURSIVE
		ancestors AS (
			SELECT ref, parent_mission_ref FROM missions WHERE ref = $1
			UNION ALL
			SELECT m.ref, m.parent_mission_ref FROM missions m
			JOIN ancestors a ON a.parent_mission_ref = m.ref
		),
		descendants AS (
			SELECT ref, parent_mission_ref FROM missions WHERE ref = $1
			UNION ALL
			SELECT m.ref, m.parent_mission_ref FROM missions m
			JOIN descendants d ON m.parent_mission_ref = d.ref
		),
		related AS (
			SELECT ref FROM ancestors
			UNION
			SELECT ref FROM descendants
		)
		SELECT m.mission_json FROM missions m
		JOIN related r ON r.ref = m.ref
		ORDER BY m.created_at, m.ref
	`, ref)
	if err != nil {
		return mission.LineageSnapshot{}, err
	}
	if len(missions) == 0 {
		return mission.LineageSnapshot{}, mission.ErrNotFound
	}
	return s.loadLineageArtifacts(ctx, missions, nil)
}

func (s *PostgresStore) LoadAgentLineageSnapshot(parent context.Context, agentID string) (mission.LineageSnapshot, error) {
	ctx, cancel := readContext(parent)
	defer cancel()
	var identityJSON []byte
	identityErr := s.db.QueryRowContext(ctx, `SELECT identity_json FROM agent_identities WHERE id = $1`, agentID).Scan(&identityJSON)
	var identity *mission.AgentIdentity
	if identityErr == nil {
		var decoded mission.AgentIdentity
		if err := json.Unmarshal(identityJSON, &decoded); err != nil {
			return mission.LineageSnapshot{}, fmt.Errorf("unmarshal agent identity: %w", err)
		}
		identity = &decoded
	} else if !errors.Is(identityErr, sql.ErrNoRows) {
		return mission.LineageSnapshot{}, fmt.Errorf("query agent identity: %w", identityErr)
	}

	var missions []mission.Mission
	var err error
	if identity != nil {
		missions, err = queryJSONRows[mission.Mission](ctx, s.db, `
			SELECT mission_json FROM missions
			WHERE tenant_id = $1 AND (
				agent_id = $2 OR
				mission_json->'agent'->>'client_id' = $3 OR
				mission_json->'agent'->>'instance_id' = $4
			)
			ORDER BY created_at, ref
		`, identity.TenantID, identity.Agent.InstanceID, identity.Agent.ClientID, identity.Agent.InstanceID)
	} else {
		missions, err = queryJSONRows[mission.Mission](ctx, s.db, `
			SELECT mission_json FROM missions
			WHERE agent_id = $1 OR
				mission_json->'agent'->>'client_id' = $1 OR
				mission_json->'agent'->>'instance_id' = $1
			ORDER BY created_at, ref
		`, agentID)
	}
	if err != nil {
		return mission.LineageSnapshot{}, err
	}
	return s.loadLineageArtifacts(ctx, missions, identity)
}

func (s *PostgresStore) loadLineageArtifacts(ctx context.Context, missions []mission.Mission, identity *mission.AgentIdentity) (mission.LineageSnapshot, error) {
	snapshot := mission.LineageSnapshot{Missions: missions, Identity: identity}
	if len(missions) == 0 {
		return snapshot, nil
	}
	refs := make([]string, 0, len(missions))
	for _, item := range missions {
		refs = append(refs, item.MissionRef)
	}
	var err error
	snapshot.ExpansionRequests, err = queryJSONRows[mission.ExpansionRequest](ctx, s.db, `
		SELECT expansion_json FROM expansion_requests WHERE mission_ref = ANY($1) ORDER BY created_at, id
	`, pq.Array(refs))
	if err != nil {
		return mission.LineageSnapshot{}, err
	}
	snapshot.Projections, err = queryJSONRows[mission.Projection](ctx, s.db, `
		SELECT projection_json FROM projections WHERE mission_ref = ANY($1) ORDER BY created_at, id
	`, pq.Array(refs))
	if err != nil {
		return mission.LineageSnapshot{}, err
	}
	snapshot.Leases, err = queryJSONRows[mission.MissionLease](ctx, s.db, `
		SELECT lease_json FROM mission_leases WHERE mission_ref = ANY($1) ORDER BY created_at, id
	`, pq.Array(refs))
	if err != nil {
		return mission.LineageSnapshot{}, err
	}
	return snapshot, nil
}

func queryTenantJSON[T any](ctx context.Context, db *sql.DB, table string, column string, order string, tenantID string) ([]T, error) {
	query := fmt.Sprintf("SELECT %s FROM %s", column, table)
	args := make([]any, 0, 1)
	if tenantID != "" {
		query += " WHERE tenant_id = $1"
		args = append(args, tenantID)
	}
	query += " ORDER BY " + order
	return queryJSONRows[T](ctx, db, query, args...)
}

func queryJSONRows[T any](ctx context.Context, db *sql.DB, query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query governance read model: %w", err)
	}
	defer rows.Close()
	items := make([]T, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan governance read model: %w", err)
		}
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("unmarshal governance read model: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate governance read model: %w", err)
	}
	return items, nil
}

func readContext(parent context.Context) (context.Context, context.CancelFunc) {
	if _, ok := parent.Deadline(); ok {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, 5*time.Second)
}
