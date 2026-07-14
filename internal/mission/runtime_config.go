package mission

import (
	"errors"
	"os"
	"strings"
)

func ProductionModeFromEnv() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("AUTH_SCOPE_MODE")))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(os.Getenv("AUTH_SCOPE_ENV")))
	}
	return mode == "prod" || mode == "production"
}

func ArtifactKeyFromEnv(production bool) ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("AUTH_SCOPE_DECISION_SECRET"))
	if secret == "" {
		if production {
			return nil, errors.New("AUTH_SCOPE_DECISION_SECRET is required in production")
		}
		secret = defaultDecisionArtifactSecret
	}
	if production && weakArtifactSecret(secret) {
		return nil, errors.New("AUTH_SCOPE_DECISION_SECRET must be a non-placeholder secret with at least 32 characters")
	}
	return []byte(secret), nil
}

func GitHubWebhookSecretFromEnv() []byte {
	secret := strings.TrimSpace(os.Getenv("AUTH_SCOPE_GITHUB_WEBHOOK_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("GITHUB_WEBHOOK_SECRET"))
	}
	if secret == "" {
		return nil
	}
	return []byte(secret)
}

func weakArtifactSecret(secret string) bool {
	normalized := strings.ToLower(strings.TrimSpace(secret))
	return len(secret) < 32 ||
		normalized == defaultDecisionArtifactSecret ||
		strings.Contains(normalized, "change-me") ||
		strings.Contains(normalized, "development")
}
