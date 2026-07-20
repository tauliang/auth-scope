package atlassian

import (
	"errors"
	"testing"
)

func TestIsConflictClassifier(t *testing.T) {
	conflict := errors.New("conflict")
	classifier := IsConflict(conflict)
	if !classifier(conflict) {
		t.Fatalf("classifier returned false for configured conflict")
	}
	if classifier(errors.New("other")) {
		t.Fatalf("classifier returned true for unrelated error")
	}
}
