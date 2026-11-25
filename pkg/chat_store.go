package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MySQL 全量消息表
type ChatMessage struct {
	ID        uint      `gorm:"primaryKey"`
	SessionID string    `gorm:"index;size:64;not null"`
	Role      string    `gorm:"size:32;not null"`       // user / assistant / tool
	Content   string    `gorm:"type:longtext;not null"` // JSON 序列化后的 ContentBlocks
	CreatedAt time.Time `gorm:"index"`
}

// 会话元信息表（可选，便于管理会话列表）
type ChatSession struct {
	ID        string `gorm:"primaryKey;size:64"`
	Title     string `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChatSummary 摘要持久化模型
type ChatSummary struct {
	ID        uint      `gorm:"primaryKey"`
	SessionID string    `gorm:"index;size:64;not null"`
	Summary   string    `gorm:"type:longtext;not null"`
	MsgCount  int       // 被摘要覆盖的消息数量
	CreatedAt time.Time `gorm:"index"`
}

// ChatStore 双写存储
type ChatStore struct {
	db    *gorm.DB
	redis *redis.Client
}

const (
	redisKeyPrefix = "chat:memory:"
	redisTTL       = 24 * time.Hour
	recentLimit    = 10 // Redis 中保留的最近消息数
)

func NewChatStoreDefault(ctx context.Context) *ChatStore {
	mysqlDSN := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")

	store, err := NewChatStore(ctx, mysqlDSN, redisAddr)
	if err != nil {
		log.Fatal("创建 ChatStore 失败:", err)
	}
	return store
}

func NewChatStore(ctx context.Context, mysqlDSN, redisAddr string) (*ChatStore, error) {
	db, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("mysql connect: %w", err)
	}

	// 获取底层连接池，后续需同时用于连通性检查、连接池配置和错误清理。
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get mysql connection pool: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	fmt.Println("✅ 数据库连接正常")
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(20)                   // 必须 ≤ MaxOpenConns，否则 panic
	sqlDB.SetConnMaxLifetime(240 * time.Second) // 略小于 MySQL 的 wait_timeout

	// 消息与摘要表都由本 Store 负责维护。只迁移消息表会导致摘要回调在
	// SaveSummary 时因 chat_summaries 不存在而失败。
	if err := db.WithContext(ctx).AutoMigrate(&ChatMessage{}, &ChatSummary{}); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("automigrate: %w", err)
	}

	// Redis 连接
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 测试 Redis 连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		_ = sqlDB.Close()
		return nil, fmt.Errorf("redis connect: %w", err)
	}
	fmt.Println("✅ Redis 连接正常")
	return &ChatStore{db, rdb}, nil
}

// Close 释放 MySQL 连接池和 Redis 客户端。
// ChatStore 同时持有两种长连接资源，调用方结束会话后应调用此方法，
// 避免测试、短命任务或重复启动示例时连接持续占用。
func (s *ChatStore) Close() error {
	if s == nil {
		return nil
	}

	var closeErrors []error
	if s.db != nil {
		sqlDB, err := s.db.DB()
		if err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("get mysql connection pool: %w", err))
		} else if err := sqlDB.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close mysql: %w", err))
		}
	}
	if s.redis != nil {
		if err := s.redis.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close redis: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}

// SaveMessage 双写：Redis 追加 + MySQL 持久化
func (s *ChatStore) SaveMessage(ctx context.Context, sessionID string, message *schema.Message) error {
	// 1. 序列化消息
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// 2. 写入 MySQL（全量）
	if err := s.db.WithContext(ctx).Create(&ChatMessage{
		SessionID: sessionID,
		Role:      string(message.Role),
		Content:   string(data),
	}).Error; err != nil {
		return err
	}

	// 3. 追加到 Redis List，并修剪到 recentLimit 条
	key := redisKeyPrefix + sessionID
	pipe := s.redis.Pipeline() // 创建管道
	pipe.RPush(ctx, key, data)
	pipe.LTrim(ctx, key, -recentLimit, -1) // 保留最新的 N 条
	pipe.Expire(ctx, key, redisTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// SaveSummary 持久化摘要
func (s *ChatStore) SaveSummary(ctx context.Context, sessionID, summary string, msgCount int) error {
	return s.db.WithContext(ctx).Create(&ChatSummary{
		SessionID: sessionID,
		Summary:   summary,
		MsgCount:  msgCount,
	}).Error
}

// GetLatestSummary 获取最近一条摘要，没有则返回 nil
func (s *ChatStore) GetLatestSummary(ctx context.Context, sessionID string) (*ChatSummary, error) {
	var sry ChatSummary
	err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Order("id DESC").First(&sry).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sry, nil
}

// GetRecentMessages 优先 Redis，未命中回源 MySQL，并附带最新摘要
func (s *ChatStore) GetRecentMessages(ctx context.Context, sessionID string) ([]*schema.Message, error) {
	// 摘要查询失败不能静默忽略，否则模型会在不完整上下文下继续对话。
	summary, err := s.GetLatestSummary(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load latest summary: %w", err)
	}

	key := redisKeyPrefix + sessionID

	// 1. 尝试 Redis
	vals, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err == nil && len(vals) > 0 {
		msgs, err := deserializeMessages(vals)
		if err != nil {
			return nil, err
		}
		// 无论缓存命中还是回源，都将最新摘要作为历史开头传给 Agent。
		return prependSummary(summary, msgs), nil
	}

	// 回源 MySQL（取最近 N 条，按 id 倒序后再正序）
	var records []ChatMessage
	err = s.db.WithContext(ctx).Where("session_id = ?", sessionID).Order("id desc").Limit(recentLimit).Find(&records).Error
	if err != nil {
		return nil, err
	}

	// 反转顺序为时间正序
	msgs := make([]*schema.Message, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		var m schema.Message
		if err := json.Unmarshal([]byte(records[i].Content), &m); err != nil {
			return nil, fmt.Errorf("deserialize message %d: %w", records[i].ID, err)
		}
		msgs = append(msgs, &m)
	}

	// 回填 Redis
	if len(msgs) > 0 {
		pipe := s.redis.Pipeline()
		for _, message := range msgs {
			b, err := json.Marshal(message)
			if err != nil {
				return nil, fmt.Errorf("serialize message for Redis: %w", err)
			}
			pipe.RPush(ctx, key, string(b))
		}
		pipe.LTrim(ctx, key, -recentLimit, -1)
		pipe.Expire(ctx, key, redisTTL)
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, fmt.Errorf("cache recent messages: %w", err)
		}
	}

	return prependSummary(summary, msgs), nil
}

// prependSummary 将持久化摘要放到会话历史开头。
// 摘要不写入 Redis 消息列表，避免在缓存中和全量消息混淆；每次读取时
// 根据 MySQL 中的最新摘要生成一条临时 system 消息供 Agent 使用。
func prependSummary(summary *ChatSummary, messages []*schema.Message) []*schema.Message {
	if summary == nil || strings.TrimSpace(summary.Summary) == "" {
		return messages
	}
	summaryMessage := schema.SystemMessage("[历史摘要] " + summary.Summary)
	return append([]*schema.Message{summaryMessage}, messages...)
}

// GetHistory 从 MySQL 分页查询全量历史
func (s *ChatStore) GetHistory(ctx context.Context, sessionID string, page, size int) ([]ChatMessage, error) {
	var records []ChatMessage
	err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Order("id ASC").
		Offset((page - 1) * size).Limit(size).
		Find(&records).Error
	return records, err
}

// 反序列化消息
func deserializeMessages(vals []string) ([]*schema.Message, error) {
	msgs := make([]*schema.Message, 0, len(vals))
	for _, v := range vals {
		var m schema.Message
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, err
		}
		msgs = append(msgs, &m)
	}
	return msgs, nil
}

// reverseMessages 函数用于反转消息切片中的元素顺序
// 使用双指针法实现高效的反转操作
// 参数:
//
//	messages - 指向 Message 结构体切片的指针，包含需要反转的消息列表
func reverseMessages(messages []*schema.Message) {
	// 初始化左右两个指针，分别指向切片的首尾
	// left 从前向后移动，right 从后向前移动
	// 当 left 小于 right 时，交换两个指针所指位置的元素
	// 直到两个指针在中间相遇或交叉，完成整个切片的反转
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		// 使用 Go 的并行赋值特性交换左右指针所指位置的元素
		messages[left], messages[right] = messages[right], messages[left]
	}
}
