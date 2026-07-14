package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func ValidateWebhookSignature(secret []byte, signatureHeader string, body []byte) bool {
	if len(secret) == 0 {
		return false
	}
	signatureHeader = strings.TrimSpace(signatureHeader)
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(provided, expected)
}

func SignWebhookBody(secret []byte, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type WebhookSummary struct {
	Repository     string
	Action         string
	Ref            string
	SHA            string
	PullRequest    int
	Branch         string
	PayloadSummary map[string]any
}

func SummarizeWebhook(event string, payload map[string]any) WebhookSummary {
	summary := WebhookSummary{
		Action:         strings.TrimSpace(fmt.Sprint(payload["action"])),
		Repository:     RepositoryFromPayload(payload),
		Ref:            strings.TrimSpace(fmt.Sprint(payload["ref"])),
		SHA:            strings.TrimSpace(fmt.Sprint(payload["after"])),
		PayloadSummary: map[string]any{},
	}
	if summary.Action == "<nil>" {
		summary.Action = ""
	}
	if summary.Ref == "<nil>" {
		summary.Ref = ""
	}
	if summary.SHA == "<nil>" {
		summary.SHA = ""
	}
	if pr, ok := payload["pull_request"].(map[string]any); ok {
		summary.PullRequest = IntFromAny(pr["number"])
		if summary.PullRequest == 0 {
			summary.PullRequest = IntFromAny(payload["number"])
		}
		if head, ok := pr["head"].(map[string]any); ok {
			if sha := strings.TrimSpace(fmt.Sprint(head["sha"])); sha != "" && sha != "<nil>" {
				summary.SHA = sha
			}
			if branch := strings.TrimSpace(fmt.Sprint(head["ref"])); branch != "" && branch != "<nil>" {
				summary.Branch = branch
			}
		}
	}
	if summary.Branch == "" {
		summary.Branch = BranchFromRef(summary.Ref)
	}
	summary.PayloadSummary["event"] = event
	summary.PayloadSummary["repository"] = summary.Repository
	summary.PayloadSummary["action"] = summary.Action
	summary.PayloadSummary["ref"] = summary.Ref
	summary.PayloadSummary["sha"] = summary.SHA
	if summary.PullRequest > 0 {
		summary.PayloadSummary["pull_request"] = summary.PullRequest
	}
	return summary
}

func RepositoryFromPayload(payload map[string]any) string {
	if repo, ok := payload["repository"].(map[string]any); ok {
		if fullName := strings.TrimSpace(fmt.Sprint(repo["full_name"])); fullName != "" && fullName != "<nil>" {
			return NormalizeRepositoryName(fullName)
		}
		owner := ""
		if ownerMap, ok := repo["owner"].(map[string]any); ok {
			owner = strings.TrimSpace(fmt.Sprint(ownerMap["login"]))
		}
		name := strings.TrimSpace(fmt.Sprint(repo["name"]))
		if owner != "" && owner != "<nil>" && name != "" && name != "<nil>" {
			return NormalizeRepositoryName(owner + "/" + name)
		}
	}
	return ""
}

func WebhookResponseFor(delivery WebhookDelivery, message string) WebhookResponse {
	return WebhookResponse{
		Accepted:   delivery.Status == WebhookDeliveryStatusAccepted,
		Status:     delivery.Status,
		Event:      delivery.Event,
		DeliveryID: delivery.DeliveryID,
		Repository: delivery.Repository,
		Action:     delivery.Action,
		Ref:        delivery.Ref,
		SHA:        delivery.SHA,
		BindingID:  delivery.BindingID,
		MissionRef: delivery.MissionRef,
		Message:    message,
		ReceivedAt: delivery.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func NormalizeRepository(owner string, repo string, repository string) (string, string, string, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	repository = NormalizeRepositoryName(repository)
	if repository != "" {
		parts := strings.Split(repository, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", "", fmt.Errorf("repository must be owner/repo")
		}
		if owner == "" {
			owner = parts[0]
		}
		if repo == "" {
			repo = parts[1]
		}
	}
	if owner == "" || repo == "" {
		return "", "", "", fmt.Errorf("owner and repo are required")
	}
	owner = strings.ToLower(owner)
	repo = strings.ToLower(repo)
	return owner, repo, owner + "/" + repo, nil
}

func NormalizeRepositoryName(repository string) string {
	repository = strings.TrimSpace(strings.TrimPrefix(repository, "https://github.com/"))
	repository = strings.TrimSuffix(repository, ".git")
	repository = strings.Trim(repository, "/")
	return strings.ToLower(repository)
}

func CleanRepoPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	return value
}

func RepoPathResource(repository string, filePath string) string {
	return NormalizeRepositoryName(repository) + ":" + CleanRepoPath(filePath)
}

func FileOperation(file ChangedFile) string {
	if operation := strings.TrimSpace(file.Operation); operation != "" {
		return operation
	}
	switch strings.ToLower(strings.TrimSpace(file.Status)) {
	case "removed", "deleted":
		return "delete"
	case "added", "modified", "renamed", "changed":
		return "edit"
	default:
		return "edit"
	}
}

func MergeCheckContext(req CheckRunPlanRequest, file ChangedFile, repository string) map[string]any {
	context := map[string]any{}
	for key, value := range req.Context {
		context[key] = value
	}
	context["github.repository"] = repository
	context["github.branch"] = strings.TrimSpace(req.Branch)
	context["github.pull_request"] = req.PullRequest
	context["github.head_sha"] = strings.TrimSpace(req.HeadSHA)
	context["github.file_path"] = CleanRepoPath(file.Path)
	context["github.file_status"] = strings.TrimSpace(file.Status)
	context["github.file_additions"] = file.Additions
	context["github.file_deletions"] = file.Deletions
	return context
}

func ConclusionForDecision(decision string) string {
	switch decision {
	case "allow", "allow_with_constraints":
		return CheckConclusionSuccess
	case "deny", "suspend":
		return CheckConclusionFailure
	case "require_approval", "require_expansion":
		return CheckConclusionActionRequired
	default:
		return CheckConclusionNeutral
	}
}

func CombineConclusion(current string, next string) string {
	rank := map[string]int{
		CheckConclusionSuccess:        0,
		CheckConclusionNeutral:        1,
		CheckConclusionActionRequired: 2,
		CheckConclusionFailure:        3,
	}
	if rank[next] > rank[current] {
		return next
	}
	return current
}

func CheckText(evaluations []CheckEvaluation) string {
	counts := map[string]int{}
	for _, evaluation := range evaluations {
		counts[evaluation.Decision]++
	}
	parts := make([]string, 0, len(counts))
	for _, decision := range []string{"allow", "require_approval", "require_expansion", "require_refresh", "deny", "suspend"} {
		if count := counts[decision]; count > 0 {
			parts = append(parts, strconv.Itoa(count)+" "+decision)
		}
	}
	return strings.Join(parts, ", ")
}

func CleanStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func CloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func IntFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		i, _ := typed.Int64()
		return int(i)
	default:
		return 0
	}
}

func BranchFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return ref
}
