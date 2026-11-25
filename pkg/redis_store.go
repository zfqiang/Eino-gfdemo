package pkg

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

// CheckPointStore 的 Redis 实现

// RedisCheckPointStore 实现 adk.CheckPointStore 接口
type RedisSessionStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisCheckPointStoreDefault() *RedisSessionStore {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	ttl := 24 * time.Hour
	return NewRedisCheckPointStore(client, ttl)
}

func NewRedisCheckPointStore(client *redis.Client, ttl time.Duration) *RedisSessionStore {
	return &RedisSessionStore{client: client, ttl: ttl}
}

func (r *RedisSessionStore) Set(ctx context.Context, key string, value []byte) error {
	return r.client.Set(ctx, "eino:checkpoint:"+key, value, r.ttl).Err()
}

func (r *RedisSessionStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	data, err := r.client.Get(ctx, "eino:checkpoint:"+key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	return data, err == nil, err
}

// Delete 可选，用于清理不再需要的检查点
func (r *RedisSessionStore) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, "eino:checkpoint:"+key).Err()
}

// Save 保存会话历史
func (s *RedisSessionStore) Save(ctx context.Context, sessionID string, msgs []*schema.Message) error {
	data, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, sessionID, data, 0).Err()
}

// Load 加载会话历史
func (s *RedisSessionStore) Load(ctx context.Context, sessionID string) ([]adk.Message, error) {
	val, err := s.client.Get(ctx, sessionID).Result()
	if err == redis.Nil {
		return []*schema.Message{}, nil // 会话不存在，返回空历史
	}
	if err != nil {
		return nil, err
	}

	var msgs []adk.Message
	if err := json.Unmarshal([]byte(val), &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}
