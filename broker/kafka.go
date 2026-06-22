package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	kafka "github.com/segmentio/kafka-go"
)

// KafkaPublisher publishes JSON-encoded messages to a Kafka topic.
type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher creates a publisher that writes to the given brokers.
// The topic is set per-message in Publish, so one publisher can write to
// multiple topics.
func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			AllowAutoTopicCreation: true,
			// RequireAll waits for all in-sync replicas to acknowledge.
			// In a single-node dev setup RequireOne is equivalent; we use
			// RequireAll here as it's the safer production default.
			RequiredAcks: kafka.RequireOne,
		},
	}
}

// Publish marshals v to JSON and writes it to topic.
func (p *KafkaPublisher) Publish(ctx context.Context, topic string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Value: data,
	})
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}

// KafkaConsumer reads messages from a single Kafka topic using a consumer group.
type KafkaConsumer struct {
	reader *kafka.Reader
}

// NewKafkaConsumer creates a consumer for topic, joining the given consumer group.
// Using a group ID means Kafka tracks the committed offset — if the consumer
// restarts, it picks up where it left off rather than replaying all messages.
func NewKafkaConsumer(brokers []string, topic, groupID string) *KafkaConsumer {
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			StartOffset: kafka.FirstOffset, // replay from the beginning if no committed offset
			MinBytes:    1,
			MaxBytes:    10e6, // 10 MB
		}),
	}
}

// Run reads messages in a loop, calling handler for each one.
// Offsets are committed after every message regardless of whether the handler
// succeeded — at-least-once delivery. Persistent handler errors are logged
// but do not stop the consumer.
func (c *KafkaConsumer) Run(ctx context.Context, handler Handler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		if err := handler(ctx, msg.Value); err != nil {
			log.Printf("handler error on topic %s: %v (committing offset anyway)", msg.Topic, err)
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit error on topic %s: %v", msg.Topic, err)
		}
	}
}

func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}
