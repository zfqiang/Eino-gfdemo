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
	checkPointID := "session-redis-demo-002"

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\n=== 循环对话已启动（输入 exit 退出）===")
	fmt.Println("试试说：删除 /tmp/test.log")

	for {
		fmt.Print("\n you say > ")
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)
		if userInput == "" {
			fmt.Println("please input something")
			continue
		}
		if userInput == "exit" {
			break
		}

		// 每轮对话都通过同一个 CheckPointID 执行
		iter := runner.Query(ctx, userInput, adk.WithCheckPointID(checkPointID))

		var interruptID string
		var interruptInfo string
		needResume := false

		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event.Err != nil {
				log.Printf("执行错误: %v", event.Err)
				break
			}

			// 检测中断
			if event.Action != nil && event.Action.Interrupted != nil {
				interruptCtx := event.Action.Interrupted.InterruptContexts[0]
				interruptID = interruptCtx.ID
				interruptInfo = fmt.Sprintf("%v", interruptCtx.Info)
				needResume = true
				break
			}

			// 正常输出
			if event.Output != nil && event.Output.MessageOutput != nil {
				msg, _ := event.Output.MessageOutput.GetMessage()
				if msg != nil && msg.Content != "" {
					fmt.Printf("assistant > %s\n", msg.Content)
				}
			}
		}

		// 处理中断：审批后恢复
		if needResume {
			fmt.Printf("\n[需要审批] %s\n", interruptInfo)
			fmt.Print("批准? (y/n): ")

			answer, _ := reader.ReadString('\n')
			approved := strings.ToLower(strings.TrimSpace(answer)) == "y"

			resumeIter, err := runner.ResumeWithParams(ctx, checkPointID, &adk.ResumeParams{
				Targets: map[string]any{
					interruptID: approved,
				},
			})
			if err != nil {
				log.Printf("恢复失败: %v", err)
				continue
			}

			for {
				event, ok := resumeIter.Next()
				if !ok {
					break
				}
				if event.Err != nil {
					log.Printf("恢复执行错误: %v", event.Err)
					break
				}
				if event.Output != nil && event.Output.MessageOutput != nil {
					msg, _ := event.Output.MessageOutput.GetMessage()
					if msg != nil && msg.Content != "" {
						fmt.Printf("assistant> %s\n", msg.Content)
					}
				}
			}
		}

	}

	fmt.Println("\n=== 流程结束 ===")
}
