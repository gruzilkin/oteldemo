package redis

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps Redis client
type Client struct {
	client *redis.Client
}

// NewClient creates a new Redis client
func NewClient(redisURL string) *Client {
	// Parse Redis URL
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	// Configure ReadTimeout for long polling with Redis Streams
	// Must be longer than Block duration in XREADGROUP (60s)
	opts.ReadTimeout = 65 * time.Second

	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Printf("Connected to Redis at %s", redisURL)

	return &Client{client: client}
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.client.Close()
}

// IsHealthy checks if Redis connection is healthy
func (c *Client) IsHealthy(ctx context.Context) bool {
	return c.client.Ping(ctx).Err() == nil
}

// CreateConsumerGroup creates a consumer group if it doesn't exist
func (c *Client) CreateConsumerGroup(ctx context.Context, stream, group string) error {
	err := c.client.XGroupCreateMkStream(ctx, stream, group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// ReadPendingMessages reads pending messages (delivered but not acknowledged) from a Redis stream
func (c *Client) ReadPendingMessages(ctx context.Context, stream, group, consumer string) ([]StreamMessage, error) {
	result, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, "0"}, // "0" = pending messages for this consumer
		Count:    10,
		Block:    0, // Non-blocking - return immediately
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, nil // No pending messages
		}
		return nil, err
	}

	var messages []StreamMessage
	for _, stream := range result {
		for _, msg := range stream.Messages {
			messages = append(messages, StreamMessage{
				ID:   msg.ID,
				Data: msg.Values,
			})
		}
	}

	return messages, nil
}

// ReadFromStream reads new messages from a Redis stream
func (c *Client) ReadFromStream(ctx context.Context, stream, group, consumer string) ([]StreamMessage, error) {
	result, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"}, // ">" = only new undelivered messages
		Count:    10,
		Block:    60000, // 60 seconds - long polling for efficiency
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, nil // No messages
		}
		return nil, err
	}

	var messages []StreamMessage
	for _, stream := range result {
		for _, msg := range stream.Messages {
			messages = append(messages, StreamMessage{
				ID:   msg.ID,
				Data: msg.Values,
			})
		}
	}

	return messages, nil
}

// AckMessage acknowledges a message
func (c *Client) AckMessage(ctx context.Context, stream, group, messageID string) error {
	return c.client.XAck(ctx, stream, group, messageID).Err()
}

// PublishResult publishes a result to a Redis stream
func (c *Client) PublishResult(ctx context.Context, stream string, data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	id, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{
			"data": string(jsonData),
		},
	}).Result()

	return id, err
}

// StreamMessage represents a message from Redis stream
type StreamMessage struct {
	ID   string
	Data map[string]interface{}
}
