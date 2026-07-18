package mission

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

func actionInScope(region AuthorityRegion, action Action) bool {
	if contains(region.ForbiddenActions, action.Operation) || contains(region.ForbiddenActions, action.Name) {
		return false
	}
	for _, grant := range region.Resources {
		if resourceMatches(grant, action.Resource) && contains(grant.Actions, action.Operation) {
			return true
		}
	}
	return false
}

func authoritySubset(parent, child AuthorityRegion) bool {
	for _, childGrant := range child.Resources {
		found := false
		for _, parentGrant := range parent.Resources {
			if grantsResource(parentGrant, childGrant) && actionsSubset(parentGrant.Actions, childGrant.Actions) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, forbidden := range parent.ForbiddenActions {
		if !contains(child.ForbiddenActions, forbidden) {
			return false
		}
	}
	return true
}

func resourceMatches(grant ResourceGrant, resource ActionResource) bool {
	typeMatches := grant.Type == "*" || grant.Type == "" || grant.Type == resource.Type
	idMatches := grant.ID == "*" || grant.ID == "" || grant.ID == resource.ID
	return typeMatches && idMatches
}

func grantsResource(parent, child ResourceGrant) bool {
	typeMatches := parent.Type == "*" || parent.Type == "" || parent.Type == child.Type
	idMatches := parent.ID == "*" || parent.ID == "" || parent.ID == child.ID
	return typeMatches && idMatches
}

func actionsSubset(parent, child []string) bool {
	for _, action := range child {
		if !contains(parent, action) {
			return false
		}
	}
	return true
}

func contains(values []string, want string) bool {
	return slices.Contains(values, "*") || slices.Contains(values, want)
}

func evaluateConditions(conditions []Condition, context map[string]any) (bool, string, error) {
	ok, failedCondition, _, err := evaluateConditionsWithEvidence(conditions, context)
	return ok, failedCondition, err
}

func evaluateConditionsWithEvidence(conditions []Condition, context map[string]any) (bool, string, []ConditionEvaluation, error) {
	results := make([]ConditionEvaluation, 0, len(conditions))
	for _, condition := range conditions {
		ok, err := evaluateCondition(condition.Expression, context)
		result := ConditionEvaluation{
			ID:         condition.ID,
			Expression: condition.Expression,
			Result:     ok,
		}
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			return false, condition.ID, results, err
		}
		results = append(results, result)
		if !ok {
			return false, condition.ID, results, nil
		}
	}
	return true, "", results, nil
}

func evaluateCondition(expression string, context map[string]any) (bool, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" || expression == "true" {
		return true, nil
	}
	if expression == "false" {
		return false, nil
	}

	if left, right, ok := strings.Cut(expression, "=="); ok {
		actual, exists := lookupValue(context, strings.TrimSpace(left))
		if !exists {
			return false, nil
		}
		expected := parseLiteral(strings.TrimSpace(right))
		return valuesEqual(actual, expected), nil
	}
	if left, right, ok := strings.Cut(expression, "!="); ok {
		actual, exists := lookupValue(context, strings.TrimSpace(left))
		if !exists {
			return true, nil
		}
		expected := parseLiteral(strings.TrimSpace(right))
		return !valuesEqual(actual, expected), nil
	}

	return false, fmt.Errorf("unsupported condition expression %q", expression)
}

func lookupValue(context map[string]any, path string) (any, bool) {
	if context == nil {
		return nil, false
	}
	if value, ok := context[path]; ok {
		return value, true
	}
	parts := strings.Split(path, ".")
	var current any = context
	for _, part := range parts {
		next, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = next[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func parseLiteral(value string) any {
	value = strings.TrimSpace(value)
	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return strings.Trim(value, "'")
}

func valuesEqual(actual, expected any) bool {
	if reflect.DeepEqual(actual, expected) {
		return true
	}
	return fmt.Sprint(actual) == fmt.Sprint(expected)
}
