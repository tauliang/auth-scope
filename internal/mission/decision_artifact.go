package mission

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultDecisionArtifactSecret = "auth-scope-development-decision-secret"

type decisionArtifactHeader struct {
	Type      string `json:"typ"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type DecisionArtifactPayload struct {
	ArtifactID         string    `json:"artifact_id"`
	MissionRef         string    `json:"mission_ref"`
	MissionVersion     int       `json:"mission_version"`
	PolicyVersion      string    `json:"policy_version,omitempty"`
	EvidenceID         string    `json:"evidence_id,omitempty"`
	ExpansionRequestID string    `json:"expansion_request_id,omitempty"`
	Decision           Decision  `json:"decision"`
	ReasonCodes        []string  `json:"reason_codes,omitempty"`
	Actor              Actor     `json:"actor"`
	Action             Action    `json:"action"`
	ContextHash        string    `json:"context_hash"`
	IssuedAt           time.Time `json:"issued_at"`
}

func decisionArtifactKeyFromEnv() []byte {
	key, err := ArtifactKeyFromEnv(false)
	if err != nil {
		return []byte(defaultDecisionArtifactSecret)
	}
	return key
}

func SignDecisionArtifact(payload DecisionArtifactPayload, key []byte) (string, error) {
	if len(key) == 0 {
		return "", errors.New("decision artifact key is required")
	}
	headerJSON, err := json.Marshal(decisionArtifactHeader{
		Type:      "mission-decision+jws",
		Algorithm: "HS256",
		KeyID:     "local",
	})
	if err != nil {
		return "", fmt.Errorf("marshal artifact header: %w", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal artifact payload: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyDecisionArtifact(token string, key []byte) (DecisionArtifactPayload, error) {
	if len(key) == 0 {
		return DecisionArtifactPayload{}, errors.New("decision artifact key is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return DecisionArtifactPayload{}, errors.New("decision artifact must have three parts")
	}
	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return DecisionArtifactPayload{}, fmt.Errorf("decode artifact signature: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(signingInput))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return DecisionArtifactPayload{}, errors.New("decision artifact signature mismatch")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return DecisionArtifactPayload{}, fmt.Errorf("decode artifact header: %w", err)
	}
	var header decisionArtifactHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return DecisionArtifactPayload{}, fmt.Errorf("unmarshal artifact header: %w", err)
	}
	if header.Algorithm != "HS256" || header.Type != "mission-decision+jws" {
		return DecisionArtifactPayload{}, errors.New("unsupported decision artifact header")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return DecisionArtifactPayload{}, fmt.Errorf("decode artifact payload: %w", err)
	}
	var payload DecisionArtifactPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return DecisionArtifactPayload{}, fmt.Errorf("unmarshal artifact payload: %w", err)
	}
	return payload, nil
}

func HashDecisionContext(context map[string]any) string {
	if context == nil {
		context = map[string]any{}
	}
	data, err := json.Marshal(context)
	if err != nil {
		data = []byte(fmt.Sprint(context))
	}
	sum := sha256.Sum256(data)
	return "sha256:" + base64.RawURLEncoding.EncodeToString(sum[:])
}
