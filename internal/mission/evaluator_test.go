package mission

import "testing"

func TestActionInScopeHonorsWildcardsAndForbiddenActions(t *testing.T) {
	region := AuthorityRegion{
		Resources: []ResourceGrant{
			{Type: "drive_folder", ID: "*", Actions: []string{"read"}},
			{Type: "ticket", ID: "inc-123", Actions: []string{"*"}},
			{Type: "repo_path", ID: "tauliang/auth-scope:frontend/**", Actions: []string{"edit"}},
		},
		ForbiddenActions: []string{"delete", "email.send"},
	}

	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{
			name:   "allows matching resource and action",
			action: Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "read"},
			want:   true,
		},
		{
			name:   "allows wildcard action",
			action: Action{Resource: ActionResource{Type: "ticket", ID: "inc-123"}, Operation: "comment"},
			want:   true,
		},
		{
			name:   "blocks forbidden operation",
			action: Action{Resource: ActionResource{Type: "drive_folder", ID: "board"}, Operation: "delete"},
			want:   false,
		},
		{
			name:   "blocks forbidden tool name",
			action: Action{Name: "email.send", Resource: ActionResource{Type: "ticket", ID: "inc-123"}, Operation: "comment"},
			want:   false,
		},
		{
			name:   "blocks unmatched resource",
			action: Action{Resource: ActionResource{Type: "drive_folder", ID: "other"}, Operation: "write"},
			want:   false,
		},
		{
			name:   "allows repository prefix grant",
			action: Action{Resource: ActionResource{Type: "repo_path", ID: "tauliang/auth-scope:frontend/src/App.tsx"}, Operation: "edit"},
			want:   true,
		},
		{
			name:   "blocks repository path outside prefix grant",
			action: Action{Resource: ActionResource{Type: "repo_path", ID: "tauliang/auth-scope:backend/main.go"}, Operation: "edit"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := actionInScope(region, tt.action); got != tt.want {
				t.Fatalf("actionInScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceIDMatchesGlobAndTreePatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		id      string
		want    bool
	}{
		{name: "exact", pattern: "repo:path/file.go", id: "repo:path/file.go", want: true},
		{name: "tree prefix", pattern: "repo:frontend/**", id: "repo:frontend/src/App.tsx", want: true},
		{name: "tree root", pattern: "repo:frontend/**", id: "repo:frontend", want: true},
		{name: "tree outside", pattern: "repo:frontend/**", id: "repo:frontends/App.tsx", want: false},
		{name: "path match", pattern: "repo:*.md", id: "repo:README.md", want: true},
		{name: "path match does not cross slash", pattern: "repo:*.md", id: "repo:docs/README.md", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceIDMatches(tt.pattern, tt.id); got != tt.want {
				t.Fatalf("resourceIDMatches(%q, %q) = %v, want %v", tt.pattern, tt.id, got, tt.want)
			}
		})
	}
}

func TestAuthoritySubset(t *testing.T) {
	parent := AuthorityRegion{
		Resources: []ResourceGrant{
			{Type: "drive_folder", ID: "board", Actions: []string{"read", "write_draft"}},
			{Type: "ticket", ID: "*", Actions: []string{"*"}},
		},
		ForbiddenActions: []string{"send_external"},
	}

	tests := []struct {
		name  string
		child AuthorityRegion
		want  bool
	}{
		{
			name: "allows strict narrower resource and action",
			child: AuthorityRegion{
				Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
				ForbiddenActions: []string{"send_external"},
			},
			want: true,
		},
		{
			name: "allows parent wildcard resource",
			child: AuthorityRegion{
				Resources:        []ResourceGrant{{Type: "ticket", ID: "inc-123", Actions: []string{"comment"}}},
				ForbiddenActions: []string{"send_external"},
			},
			want: true,
		},
		{
			name: "rejects action not held by parent",
			child: AuthorityRegion{
				Resources:        []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"delete"}}},
				ForbiddenActions: []string{"send_external"},
			},
			want: false,
		},
		{
			name: "rejects missing inherited forbidden action",
			child: AuthorityRegion{
				Resources: []ResourceGrant{{Type: "drive_folder", ID: "board", Actions: []string{"read"}}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authoritySubset(parent, tt.child); got != tt.want {
				t.Fatalf("authoritySubset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionExpressions(t *testing.T) {
	context := map[string]any{
		"finance.close.status": "open",
		"budget":               int64(10),
		"approved":             true,
		"ratio":                1.5,
		"nested": map[string]any{
			"state": "ready",
		},
	}

	tests := []struct {
		name       string
		expression string
		context    map[string]any
		want       bool
		wantErr    bool
	}{
		{name: "empty expression is true", expression: "", context: context, want: true},
		{name: "literal true", expression: "true", context: context, want: true},
		{name: "literal false", expression: "false", context: context, want: false},
		{name: "direct string equality", expression: "finance.close.status == 'open'", context: context, want: true},
		{name: "nested lookup equality", expression: "nested.state == 'ready'", context: context, want: true},
		{name: "bool equality", expression: "approved == true", context: context, want: true},
		{name: "int equality", expression: "budget == 10", context: context, want: true},
		{name: "float equality", expression: "ratio == 1.5", context: context, want: true},
		{name: "inequality", expression: "finance.close.status != 'closed'", context: context, want: true},
		{name: "missing equality is false", expression: "missing == 'value'", context: context, want: false},
		{name: "missing inequality is true", expression: "missing != 'value'", context: context, want: true},
		{name: "nil context is false", expression: "approved == true", context: nil, want: false},
		{name: "unsupported expression errors", expression: "approved contains true", context: context, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateCondition(tt.expression, tt.context)
			if (err != nil) != tt.wantErr {
				t.Fatalf("evaluateCondition() err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("evaluateCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionsReturnsFailedConditionAndErrors(t *testing.T) {
	ok, failed, err := evaluateConditions([]Condition{
		{ID: "first", Expression: "true"},
		{ID: "second", Expression: "status == 'open'"},
	}, map[string]any{"status": "closed"})
	if err != nil {
		t.Fatalf("evaluateConditions unexpected error: %v", err)
	}
	if ok || failed != "second" {
		t.Fatalf("expected second condition failure, ok=%v failed=%q", ok, failed)
	}

	ok, failed, err = evaluateConditions([]Condition{{ID: "bad", Expression: "status ~= 'open'"}}, map[string]any{"status": "open"})
	if err == nil {
		t.Fatal("expected unsupported expression error")
	}
	if ok || failed != "bad" {
		t.Fatalf("expected bad condition error, ok=%v failed=%q", ok, failed)
	}
}
