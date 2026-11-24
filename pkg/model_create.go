package pkg

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("加载 .env 文件失败: %v", err)
	}
}

func NewChatModel(ctx context.Context) *openai.ChatModel {
	ctx = context.Background()

	for _, name := range []string{"DASHSCOPE_API_KEY", "DASHSCOPE_MODE", "DASHSCOPE_BASE_URL"} {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			log.Fatalf("缺少环境变量 %s", name)
		}
	}

	// 创建模型
	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
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
	return model
}
