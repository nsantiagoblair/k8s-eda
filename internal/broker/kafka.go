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

func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireOne,
		},
	}
}

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

func NewKafkaConsumer(brokers []string, topic, groupID string) *KafkaConsumer {
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			StartOffset: kafka.FirstOffset,
			MinBytes:    1,
			MaxBytes:    10e6,
		}),
	}
}

func (c *KafkaConsumer) Run(ctx context.Context, handler Handler) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
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
