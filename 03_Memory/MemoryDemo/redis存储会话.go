package memorydemo

import (
	"bufio"
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func RedisSessionStoreDemo() {
	ctx := context.Background()

	// 1. Redis 会话存储
	store := pkg.NewRedisCheckPointStoreDefault()
	sessionID := "sess_001" // 业务侧自己生成，比如用户+会话ID

	// 2. 初始化 ChatModel -- init
	cm := pkg.GetBaiLianLLM(ctx)

	// 3. 注册工具
	// weatherTool := schema.

	// 4. 构建 ADK ChatModelAgent
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "AI助手",
		Description: "你是一个智能的AI助手",
		Instruction: "你是一个智能的AI助手，请根据用户输入进行回答，先给结论再解释。",
		Model:       cm,
	})

	if err != nil {
		log.Fatal("创建 ChatModelAgent 失败")
	}

	// 5. 构建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	// 6. 加载会话历史
	history, err := store.Load(ctx, sessionID)
	if err != nil {
		log.Fatal("加载会话历史失败")
	}

	// 7. 启动会话
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

		// 合并历史消息
		history = append(history, schema.UserMessage(userInput))
		// 运行返回的事件单元
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
