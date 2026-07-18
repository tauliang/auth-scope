package mission

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	AgentStatusActive  = "active"
	AgentStatusRevoked = "revoked"
)

var (
	ErrInvalidSignature = errors.New("invalid agent signature")
	ErrAgentRevoked     = errors.New("agent identity revoked")
)

type AgentIdentity struct {
	AgentID       string    `json:"agent_id"`
	TenantID      string    `json:"tenant_id"`
	Agent         Agent     `json:"agent"`
	PublicKey     string    `json:"public_key"`
	KeyThumbprint string    `json:"key_thumbprint"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	RevokedAt     time.Time `json:"revoked_at,omitempty"`
}

type AgentNonce struct {
	AgentID     string    `json:"agent_id"`
	Nonce       string    `json:"nonce"`
	RequestHash string    `json:"request_hash"`
	SeenAt      time.Time `json:"seen_at"`
}

type RegisterAgentRequest struct {
	TenantID  string `json:"tenant_id"`
	Agent     Agent  `json:"agent"`
	PublicKey string `json:"public_key"`
}

type RegisterAgentResponse struct {
	AgentID       string `json:"agent_id"`
	KeyThumbprint string `json:"key_thumbprint"`
	Status        string `json:"status"`
}

func DecodeAgentPublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must be %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

func PublicKeyThumbprint(encoded string) (string, error) {
	key, err := DecodeAgentPublicKey(encoded)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(key)
	return "sha256:" + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func CanonicalAgentSigningString(method string, target string, body []byte, nonce string) string {
	bodyHash := sha256.Sum256(body)
	return strings.Join([]string{
		"AUTH-SCOPE-SIGNATURE-V1",
		strings.ToUpper(method),
		target,
		hex.EncodeToString(bodyHash[:]),
		nonce,
	}, "\n")
}

func AgentRequestHash(method string, target string, body []byte) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(method) + "\n" + target + "\n" + string(body)))
	return hex.EncodeToString(sum[:])
}

func VerifyAgentSignature(identity AgentIdentity, method string, target string, body []byte, nonce string, encodedSignature string) error {
	if identity.Status == AgentStatusRevoked {
		return ErrAgentRevoked
	}
	publicKey, err := DecodeAgentPublicKey(identity.PublicKey)
	if err != nil {
		return err
	}
	signature, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encodedSignature))
	if err != nil {
		return ErrInvalidSignature
	}
	message := []byte(CanonicalAgentSigningString(method, target, body, nonce))
	if !ed25519.Verify(publicKey, message, signature) {
		return ErrInvalidSignature
	}
	return nil
}

func EncodeAgentSignature(signature []byte) string {
	return base64.RawURLEncoding.EncodeToString(signature)
}
