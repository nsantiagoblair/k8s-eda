package broker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
)

func TestInMemoryPublisher(t *testing.T) {
	pub := broker.NewInMemoryPublisher()
	ctx := context.Background()

	type payload struct{ X int }
	pub.Publish(ctx, "topic-a", payload{X: 1}) //nolint:errcheck
	pub.Publish(ctx, "topic-a", payload{X: 2}) //nolint:errcheck
	pub.Publish(ctx, "topic-b", payload{X: 3}) //nolint:errcheck

	if pub.Count("topic-a") != 2 { t.Errorf("Count(topic-a) = %d; want 2", pub.Count("topic-a")) }
	if pub.Count("topic-b") != 1 { t.Errorf("Count(topic-b) = %d; want 1", pub.Count("topic-b")) }
	if pub.Count("topic-c") != 0 { t.Errorf("Count(topic-c) = %d; want 0", pub.Count("topic-c")) }
}

func TestHandlerFunc(t *testing.T) {
	type msg struct{ Value string }

	called := false
	h := broker.HandlerFunc[msg](func(_ context.Context, m msg) error {
		called = true
		if m.Value != "hello" {
			t.Errorf("Value = %q; want hello", m.Value)
		}
		return nil
	})

	data, _ := json.Marshal(msg{Value: "hello"})
	if err := h(context.Background(), data); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
}

func TestHandlerFunc_InvalidJSON(t *testing.T) {
	h := broker.HandlerFunc[struct{ X int }](func(_ context.Context, _ struct{ X int }) error {
		return nil
	})
	if err := h(context.Background(), []byte("not json")); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
