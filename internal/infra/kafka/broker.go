// Package kafka provides Kafka implementations of broker.Publisher and broker.Consumer.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	segmentio "github.com/segmentio/kafka-go"
)

// Publisher publishes JSON-encoded messages to Kafka topics.
type Publisher struct {
	writer *segmentio.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		writer: &segmentio.Writer{
			Addr:                   segmentio.TCP(brokers...),
			AllowAutoTopicCreation: true,
			RequiredAcks:           segmentio.RequireOne,
		},
	}
}

func (p *Publisher) Publish(ctx context.Context, topic string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return p.writer.WriteMessages(ctx, segmentio.Message{Topic: topic, Value: data})
}

func (p *Publisher) Close() error { return p.writer.Close() }

// Consumer reads messages from a single Kafka topic using a consumer group.
type Consumer struct {
	reader *segmentio.Reader
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: segmentio.NewReader(segmentio.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			StartOffset: segmentio.FirstOffset,
			MinBytes:    1,
			MaxBytes:    10e6,
		}),
	}
}

func (c *Consumer) Run(ctx context.Context, handler broker.Handler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch: %w", err)
		}
		if err := handler(ctx, msg.Value); err != nil {
			log.Printf("handler error on %s: %v (committing anyway)", msg.Topic, err)
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit error on %s: %v", msg.Topic, err)
		}
	}
}

func (c *Consumer) Close() error { return c.reader.Close() }
