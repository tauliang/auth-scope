package contract

import (
	"errors"
	"testing"
)

func TestIsConflictClassifier(t *testing.T) {
	conflict := errors.New("conflict")
	if IsConflict(nil, conflict) {
		t.Fatalf("nil classifier returned true")
	}
	if !IsConflict(func(err error) bool { return errors.Is(err, conflict) }, conflict) {
		t.Fatalf("classifier returned false for configured conflict")
	}
	if IsConflict(func(err error) bool { return errors.Is(err, conflict) }, errors.New("other")) {
		t.Fatalf("classifier returned true for unrelated error")
	}
}
