package main

import (
	"bufio"
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/redis/go-redis/v9"
)

// 工具输入
type DeleteFileInput struct {
	Path string `json:"path"`
}

type DeleteFileOutput struct {
	Result string `json:"result"`
}

func main() {
	ctx := context.Background()

	// 连接redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis 连接失败: %v", err)
	}
	log.Println("Redis 连接成功")

	// 2. 创建 Redis CheckPointStore（TTL 设为 24 小时）
	store := pkg.NewRedisCheckPointStore(rdb, 24*time.Hour)

	// 3. 创建可中断的工具：删除文件需要人工审批
	deleteTool, err := utils.InferTool(
		"DeleteFile",
		"删除指定路径的文件。执行前需要用户审批。",
		func(ctx context.Context, input *DeleteFileInput) (*DeleteFileOutput, error) {
			// 检查是否是从中断恢复后的调用
			isResume, hasData, approved := compose.GetResumeContext[bool](ctx)
			if isResume && hasData {
				if approved {
					// 用户批准，执行删除（演示中仅返回结果）
					return &DeleteFileOutput{
						Result: fmt.Sprintf("已删除文件: %s", input.Path),
					}, nil
				}
				return &DeleteFileOutput{
					Result: fmt.Sprintf("用户拒绝了删除操作: %s", input.Path),
				}, nil
			}
			// 首次调用：触发中断，等待审批
			return nil, compose.Interrupt(ctx, fmt.Sprintf("确认删除文件 [%s] 吗？", input.Path))
		},
	)
	if err != nil {
		log.Fatalf("创建工具失败: %v", err)
	}

	// 4. 创建 ChatModel（这里用 OpenAI 兼容接口，可替换为 DeepSeek 等）
	// 注意：需要设置环境变量 OPENAI_API_KEY 和 OPENAI_BASE_URL
	// 如果没有真实模型，可以跳过 Agent，直接用 Graph 演示中断恢复
	// 这里为了简洁，我们用一个简单的 ChatModelAgent 配置
	model := pkg.NewChatModel(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       model,
		Name:        "DeleteFile",
		Instruction: "你是一个文件管理助手。当用户要求删除文件时，必须调用 DeleteFile 工具。",
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{deleteTool},
			},
		},
	})

	if err != nil {
		log.Fatalf("创建 ChatModelAgent 失败: %v", err)
	}

	// 5. 创建 Runner，注入 Redis Store
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
		CheckPointStore: store,
	})

	// 6. 首次运行，触发中断
	fmt.Println("\n=== 首次运行，触发中断 ===")
	checkPointID := "session-redis-demo-001"
	iter := runner.Query(ctx, "请帮我删除 /tmp/test.log 这个文件", adk.WithCheckPointID(checkPointID))

	var interruptID string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Fatal(event.Err)
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			interruptCtx := event.Action.Interrupted.InterruptContexts[0]
			interruptID = interruptCtx.ID
			fmt.Printf("中断信息: %v\n", interruptCtx.Info)
			fmt.Printf("中断 ID: %s\n", interruptID)
			break
		}
	}

	if interruptID == "" {
		fmt.Println("未发生中断，流程直接结束")
		return
	}

	// 7. 模拟用户审批输入
	fmt.Print("\n是否批准删除操作？(y/n): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	approved := input == "y"

	// 8. 从 Redis 恢复并继续执行
	fmt.Println("\n=== 从 Redis 恢复执行 ===")
	resumeIter, err := runner.ResumeWithParams(ctx, checkPointID, &adk.ResumeParams{
		Targets: map[string]any{
			interruptID: approved,
		},
	})
	if err != nil {
		log.Fatalf("恢复失败: %v", err)
	}

	for {
		event, ok := resumeIter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Fatal(event.Err)
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, _ := event.Output.MessageOutput.GetMessage()
			if msg != nil && msg.Content != "" {
				fmt.Printf("Agent 响应: %s\n", msg.Content)
			}
		}
	}

	fmt.Println("\n=== 流程结束 ===")
}
