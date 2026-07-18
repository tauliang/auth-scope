package mission

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeOutboxStore struct {
	cancel context.CancelFunc
	events []OutboxEvent
	err    error
	calls  int
}

func (s *fakeOutboxStore) PublishOutboxEvents() ([]OutboxEvent, error) {
	s.calls++
	s.cancel()
	return s.events, s.err
}

func TestOutboxPublisherPublishesUntilContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &fakeOutboxStore{
		cancel: cancel,
		events: []OutboxEvent{{
			ID:        "outbox-1",
			Type:      "mission.evaluated",
			CreatedAt: time.Now().UTC(),
		}},
	}

	publisher := NewOutboxPublisher(store, time.Nanosecond)
	err := publisher.Start(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Start err = %v, want context.Canceled", err)
	}
	if store.calls < 1 {
		t.Fatalf("PublishOutboxEvents calls = %d, want at least 1", store.calls)
	}
}

func TestOutboxPublisherContinuesAfterPublishError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &fakeOutboxStore{
		cancel: cancel,
		err:    errors.New("publish failed"),
	}

	publisher := NewOutboxPublisher(store, time.Nanosecond)
	err := publisher.Start(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Start err = %v, want context.Canceled", err)
	}
	if store.calls < 1 {
		t.Fatalf("PublishOutboxEvents calls = %d, want at least 1", store.calls)
	}
}
