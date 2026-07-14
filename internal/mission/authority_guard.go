package mission

import (
	"context"
	"time"
)

type ContainmentGuardStore interface {
	GetMission(string) (Mission, error)
	ListActiveContainmentRules(context.Context, time.Time) ([]ContainmentRule, error)
}

type AuthorityOperation struct {
	Mission    *Mission
	Projection *Projection
	Actor      Actor
	Action     Action
	At         time.Time
}

type AuthorityGuard interface {
	Check(context.Context, AuthorityOperation) (ContainmentRule, bool, error)
}

type ContainmentGuard struct {
	store ContainmentGuardStore
}

func NewContainmentGuard(store ContainmentGuardStore) *ContainmentGuard {
	return &ContainmentGuard{store: store}
}

func (g *ContainmentGuard) Check(ctx context.Context, operation AuthorityOperation) (ContainmentRule, bool, error) {
	if err := ctx.Err(); err != nil {
		return ContainmentRule{}, false, err
	}
	rules, err := g.store.ListActiveContainmentRules(ctx, operation.At)
	if err != nil {
		return ContainmentRule{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return ContainmentRule{}, false, err
	}

	mission := operation.Mission
	if mission == nil && operation.Projection != nil {
		loaded, err := g.store.GetMission(operation.Projection.MissionRef)
		if err != nil {
			return ContainmentRule{}, false, err
		}
		mission = &loaded
	}

	for _, rule := range rules {
		if mission != nil && containmentRuleMatchesEvaluation(rule, *mission, operation.Actor, operation.Action) {
			return rule, true, nil
		}
		if operation.Projection != nil && containmentRuleMatchesProjection(rule, *operation.Projection, nil) {
			return rule, true, nil
		}
	}
	return ContainmentRule{}, false, nil
}
