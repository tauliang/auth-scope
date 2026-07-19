-- Drop indexes first (PostgreSQL requires explicit order)
DROP INDEX IF EXISTS idx_events_type_created_at;
DROP INDEX IF EXISTS idx_proposals_principal_id;
DROP INDEX IF EXISTS idx_proposals_tenant_id;
DROP INDEX IF EXISTS idx_missions_parent_mission_ref;
DROP INDEX IF EXISTS idx_missions_state;
DROP INDEX IF EXISTS idx_missions_agent_id;
DROP INDEX IF EXISTS idx_missions_principal_id;
DROP INDEX IF EXISTS idx_missions_tenant_id;

DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS missions;
DROP TABLE IF EXISTS mission_proposals;
