package redis

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/go-redis/redis/v8"
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

// ReadFromStream reads messages from a Redis stream
func (c *Client) ReadFromStream(ctx context.Context, stream, group, consumer string) ([]StreamMessage, error) {
	result, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    10,
		Block:    1000, // 1 second
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
