package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var ctx context.Context
var model *openai.ChatModel
var err error

func init() {
	ctx = context.Background()

	if err = godotenv.Load(); err != nil {
		log.Fatal("加载 .env 文件失败")
	}

	// 创建模型
	model, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
		Model:   os.Getenv("DASHSCOPE_MODE"),
		BaseURL: os.Getenv("DASHSCOPE_BASE_URL"),

		Timeout: 30 * time.Second,

		// // 生成参数
		Temperature: new(float32(0.7)), // 控制输出随机性，范围 [0.0, 2.0]，越高越随机
		TopP:        new(float32(0.7)), // 核采样参数，范围 [0.0, 1.0]，越低越聚焦
		MaxTokens:   new(int(500)),     // 限制最大生成 token 数量，范围 [1, 8192]

		// // 停止序列 - 遇到这些文本时停止生成
		// Stop: []string{"\\n\\n", "总结:"},

		// // 惩罚参数 - 控制重复度
		// PresencePenalty:  new(float32(0.6)), // 存在惩罚，范围 [-2.0, 2.0]，正值增加新话题可能性
		// FrequencyPenalty: new(float32(0.5)), // 频率惩罚，范围 [-2.0, 2.0]，正值减少重复词语
	})
	if err != nil {
		log.Fatal("阿里云百炼  创建 ChatModel 失败")
	}
	fmt.Println("阿里云百炼  创建 ChatModel 成功")
}
func main() {

	// 1. Redis 会话存储
	store := NewRedisSessionStore("127.0.0.1:6379", 86400)
	sessionID := "sess_001" // 业务侧自己生成，比如用户+会话ID

	// 2. 初始化 ChatModel -- init

	// 3. 注册工具
	// weatherTool := schema.

	// 4. 构建 ADK ChatModelAgent
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "AI助手",
		Description: "你是一个智能的AI助手",
		Instruction: "你是一个智能的AI助手，请根据用户输入进行回答，先给结论再解释。",
		Model:       model,
	})

	if err != nil {
		log.Fatal("创建 ChatModelAgent 失败")
	}

	// 5. 构建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	history, err := store.Load(ctx, sessionID)
	if err != nil {
		log.Fatal("加载会话历史失败")
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("开始对话（输入 'exit' 退出）：")

	for {
		fmt.Print("用户：")
		if !scanner.Scan() {
			break
		}
		userInput := scanner.Text()
		if userInput == "" {
			fmt.Println("请输入内容")
			continue
		}
		if userInput == "exit" {
			fmt.Print("再见！！！")
			break
		}

		history = append(history, schema.UserMessage(userInput))
		events := runner.Run(ctx, history)
		for {
			event, ok := events.Next()
			if !ok {
				break // 迭代器关闭，全部事件已消费
			}
			if event.Err != nil {
				log.Fatal("运行 Runner 错误")
			}

			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
				resp := msg.Content
				fmt.Printf("Agent: %s\n", resp)
				history = append(history, []adk.Message{schema.AssistantMessage(resp, nil)}...)
				store.Save(ctx, sessionID, history)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal("读取输入错误")
	}
	fmt.Println("对话结束。")
}

// ===================== 上下文压缩工具 =====================

// RedisSessionStore 基于 Redis 实现会话存储
type RedisSessionStore struct {
	client *redis.Client
	ttl    int // 过期时间，秒
}

func NewRedisSessionStore(addr string, ttl int) *RedisSessionStore {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	return &RedisSessionStore{
		client: rdb,
		ttl:    ttl,
	}
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

// Save 保存会话历史
func (s *RedisSessionStore) Save(ctx context.Context, sessionID string, msgs []*schema.Message) error {
	data, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, sessionID, data, 0).Err()
}

// 示例工具：查询天气
func getWeather(ctx context.Context, city string) (string, error) {
	return fmt.Sprintf("[工具返回] %s 今日：晴，26℃", city), nil
}
