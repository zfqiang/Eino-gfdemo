package chatmodelagentdemo

import (
	"bufio"
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func AgenticBase() {
	// ChatModelAgent
	// 流程  创建model -> 创建agent -> 创建Runner -> 运行Runner返回事件单元 -> 循环事件单元数据

	ctx := context.Background()

	//创建model
	model := pkg.GetBaiLianLLM(ctx)

	//创建agent
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "AI助手",
		Description: "你是一个智能的AI助手",
		Instruction: "你是一个智能的AI助手，请根据用户输入进行回答，先给结论再解释。",
		Model:       model,
	})
	if err != nil {
		log.Fatal("创建 ChatModelAgent 失败")
	}

	//创建runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	history := make([]adk.Message, 0, 10)

	// 多轮对话并记录上下文内容
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("开始对话（输入 'quit' 退出）：")
	for {
		fmt.Print("用户 => ：")
		if !scanner.Scan() {
			break
		}

		userInput := strings.TrimSpace(scanner.Text())
		if userInput == "" {
			log.Println("输入不能为空，请重新输入")
			continue
		}
		if userInput == "quit" {
			log.Println("退出,bye bye")
			break
		}

		history = append(history, schema.UserMessage(userInput))
		// 运行返回的事件单元
		events := runner.Run(ctx, history)
		for {
			event, ok := events.Next()
			if !ok {
				break // 迭代器关闭，全部事件已消费
			}
			if event.Err != nil {
				log.Fatal("运行 Runner 错误", event.Err)
			}
			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
				resp := msg.Content
				fmt.Printf("Agent: %s\n", resp)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("读取输入错误")
	}

}
