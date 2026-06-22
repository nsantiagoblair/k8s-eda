package broker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/broker"
)

func TestInMemoryPublisher_Publish(t *testing.T) {
	pub := broker.NewInMemoryPublisher()
	ctx := context.Background()

	type payload struct{ X int }

	if err := pub.Publish(ctx, "topic-a", payload{X: 1}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := pub.Publish(ctx, "topic-a", payload{X: 2}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := pub.Publish(ctx, "topic-b", payload{X: 3}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if got := pub.Count("topic-a"); got != 2 {
		t.Errorf("Count(topic-a) = %d; want 2", got)
	}
	if got := pub.Count("topic-b"); got != 1 {
		t.Errorf("Count(topic-b) = %d; want 1", got)
	}
	if got := pub.Count("topic-c"); got != 0 {
		t.Errorf("Count(topic-c) = %d; want 0", got)
	}
}

func TestInMemoryPublisher_MessageContents(t *testing.T) {
	pub := broker.NewInMemoryPublisher()
	ctx := context.Background()

	type msg struct{ Value string }
	if err := pub.Publish(ctx, "t", msg{Value: "hello"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msgs := pub.Messages("t")
	if len(msgs) != 1 {
		t.Fatalf("Messages = %d; want 1", len(msgs))
	}

	var got msg
	if err := json.Unmarshal(msgs[0], &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Value != "hello" {
		t.Errorf("Value = %q; want %q", got.Value, "hello")
	}
}
