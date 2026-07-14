DROP INDEX IF EXISTS idx_agent_nonces_seen_at;
DROP INDEX IF EXISTS idx_agent_identities_key_thumbprint;
DROP INDEX IF EXISTS idx_agent_identities_client_instance;
DROP INDEX IF EXISTS idx_agent_identities_tenant_id;
DROP TABLE IF EXISTS agent_nonces;
DROP TABLE IF EXISTS agent_identities;
