package mission

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryStorePolicyBundleLifecycleAndGlobalFallback(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	global := PolicyBundle{
		BundleID:           "mpol_global",
		Version:            "mission-policy/global",
		Status:             PolicyBundleStatusDraft,
		CombiningAlgorithm: PolicyCombiningFirstApplicable,
		BundleHash:         "sha256:global",
		CreatedAt:          now,
		Rules:              []PolicyRule{{RuleID: "global-rule", Effect: PolicyEffectDeny}},
	}
	if err := store.SavePolicyBundle(global); err != nil {
		t.Fatalf("SavePolicyBundle global: %v", err)
	}
	global.Status = PolicyBundleStatusActive
	global.ActivatedAt = now
	if err := store.ActivatePolicyBundle(global); err != nil {
		t.Fatalf("ActivatePolicyBundle global: %v", err)
	}
	fallback, err := store.GetActivePolicyBundle("tenant-a")
	if err != nil {
		t.Fatalf("GetActivePolicyBundle fallback: %v", err)
	}
	if fallback.BundleID != global.BundleID {
		t.Fatalf("fallback bundle = %#v", fallback)
	}

	tenant := PolicyBundle{
		BundleID:           "mpol_tenant",
		TenantID:           "tenant-a",
		Version:            "mission-policy/tenant",
		Status:             PolicyBundleStatusDraft,
		CombiningAlgorithm: PolicyCombiningFirstApplicable,
		BundleHash:         "sha256:tenant",
		CreatedAt:          now.Add(time.Second),
		Rules:              []PolicyRule{{RuleID: "tenant-rule", Effect: PolicyEffectRequireApproval}},
	}
	if err := store.SavePolicyBundle(tenant); err != nil {
		t.Fatalf("SavePolicyBundle tenant: %v", err)
	}
	if err := store.SavePolicyBundle(tenant); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate SavePolicyBundle err = %v, want ErrConflict", err)
	}
	tenant.Status = PolicyBundleStatusActive
	tenant.ActivatedAt = now.Add(time.Second)
	if err := store.ActivatePolicyBundle(tenant); err != nil {
		t.Fatalf("ActivatePolicyBundle tenant: %v", err)
	}
	active, err := store.GetActivePolicyBundle("tenant-a")
	if err != nil {
		t.Fatalf("GetActivePolicyBundle tenant: %v", err)
	}
	if active.BundleID != tenant.BundleID {
		t.Fatalf("active tenant bundle = %#v", active)
	}
	if _, err := store.GetPolicyBundle("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing GetPolicyBundle err = %v, want ErrNotFound", err)
	}
}
