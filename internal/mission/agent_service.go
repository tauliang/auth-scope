package mission

import (
	"errors"
	"fmt"
)

func (s *Service) RegisterAgent(req RegisterAgentRequest) (RegisterAgentResponse, error) {
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	if req.Agent.ClientID == "" || req.Agent.InstanceID == "" {
		return RegisterAgentResponse{}, fmt.Errorf("agent.client_id and agent.instance_id are required")
	}
	if req.PublicKey == "" {
		return RegisterAgentResponse{}, fmt.Errorf("public_key is required")
	}
	thumbprint, err := PublicKeyThumbprint(req.PublicKey)
	if err != nil {
		return RegisterAgentResponse{}, err
	}
	if req.Agent.KeyThumbprint != "" && req.Agent.KeyThumbprint != thumbprint {
		return RegisterAgentResponse{}, fmt.Errorf("agent.key_thumbprint does not match public_key")
	}
	req.Agent.KeyThumbprint = thumbprint

	now := s.clock.Now()
	identity := AgentIdentity{
		AgentID:       newID("agt"),
		TenantID:      req.TenantID,
		Agent:         req.Agent,
		PublicKey:     req.PublicKey,
		KeyThumbprint: thumbprint,
		Status:        AgentStatusActive,
		CreatedAt:     now,
	}
	if err := s.identities.SaveAgentIdentity(identity); err != nil {
		return RegisterAgentResponse{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
		TenantID:   identity.TenantID,
		Type:       "agent.registered",
		Actor:      map[string]any{"agent_id": identity.AgentID},
		Payload:    map[string]any{"client_id": identity.Agent.ClientID, "instance_id": identity.Agent.InstanceID, "key_thumbprint": identity.KeyThumbprint},
		OccurredAt: now,
	})
	return RegisterAgentResponse{
		AgentID:       identity.AgentID,
		KeyThumbprint: identity.KeyThumbprint,
		Status:        identity.Status,
	}, nil
}

func (s *Service) GetAgentIdentity(agentID string) (AgentIdentity, error) {
	return s.identities.GetAgentIdentity(agentID)
}

func (s *Service) RevokeAgent(agentID string, req StateChangeRequest) (AgentIdentity, error) {
	identity, err := s.identities.GetAgentIdentity(agentID)
	if err != nil {
		return AgentIdentity{}, err
	}
	if identity.Status == AgentStatusRevoked {
		return identity, nil
	}
	now := s.clock.Now()
	identity.Status = AgentStatusRevoked
	identity.RevokedAt = now
	if err := s.identities.UpdateAgentIdentity(identity); err != nil {
		return AgentIdentity{}, err
	}
	_ = s.events.AppendEvent(Event{
		EventID:    newID("mev"),
		TenantID:   identity.TenantID,
		Type:       "agent.revoked",
		Actor:      req.Actor,
		Payload:    map[string]any{"agent_id": agentID, "reason": req.Reason},
		OccurredAt: now,
	})
	return identity, nil
}

func (s *Service) VerifyAgentRequestSignature(method string, target string, body []byte, agentID string, nonce string, signature string) (AgentIdentity, error) {
	if agentID == "" || nonce == "" || signature == "" {
		return AgentIdentity{}, ErrInvalidSignature
	}
	identity, err := s.identities.GetAgentIdentity(agentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AgentIdentity{}, ErrInvalidSignature
		}
		return AgentIdentity{}, err
	}
	if err := VerifyAgentSignature(identity, method, target, body, nonce, signature); err != nil {
		return AgentIdentity{}, err
	}
	err = s.identities.SaveAgentNonce(AgentNonce{
		AgentID:     agentID,
		Nonce:       nonce,
		RequestHash: AgentRequestHash(method, target, body),
		SeenAt:      s.clock.Now(),
	})
	if errors.Is(err, ErrConflict) {
		return AgentIdentity{}, ErrInvalidSignature
	}
	if err != nil {
		return AgentIdentity{}, err
	}
	return identity, nil
}

func bindActorIdentity(actor *Actor, identity AgentIdentity) error {
	if actor.AgentInstanceID != "" && actor.AgentInstanceID != identity.Agent.InstanceID {
		return fmt.Errorf("signed agent instance does not match request actor")
	}
	if actor.ClientID != "" && actor.ClientID != identity.Agent.ClientID {
		return fmt.Errorf("signed agent client does not match request actor")
	}
	actor.AgentInstanceID = identity.Agent.InstanceID
	actor.ClientID = identity.Agent.ClientID
	actor.KeyThumbprint = identity.KeyThumbprint
	return nil
}

func bindAuthZENSubjectIdentity(subject *AuthZENEntity, identity AgentIdentity) error {
	if subject.ID != "" && subject.ID != identity.Agent.InstanceID {
		return fmt.Errorf("signed agent instance does not match authzen subject")
	}
	if subject.Properties == nil {
		subject.Properties = map[string]any{}
	}
	if clientID := authZENString(subject.Properties, "client_id"); clientID != "" && clientID != identity.Agent.ClientID {
		return fmt.Errorf("signed agent client does not match authzen subject")
	}
	subject.ID = identity.Agent.InstanceID
	subject.Properties["agent_instance_id"] = identity.Agent.InstanceID
	subject.Properties["client_id"] = identity.Agent.ClientID
	subject.Properties["key_thumbprint"] = identity.KeyThumbprint
	return nil
}
