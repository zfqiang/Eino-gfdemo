package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

var ctx context.Context
var model *openai.ChatModel
var err error

func main() {
	demo1()
}

func demo1() {
	// ChatModelAgent
	// 流程  创建model -> 创建agent -> 创建Runner -> 运行Runner -> 循环事件单元数据

	// 创建 ChatModelAgent
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "AI助手",
		Description: "你是一个智能的AI助手",
		Instruction: "你是一个智能的AI助手，请根据用户输入进行回答，先给结论再解释。",
		Model:       model,
	})
	if err != nil {
		log.Fatal("创建 ChatModelAgent 失败")
	}

	// 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	history := make([]adk.Message, 0, 16)
	//多轮对话
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("开始对话（输入 'exit' 退出）：")

	for {
		fmt.Print("[user] ：")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())
		if userInput == "" {
			fmt.Println("请输入内容")
			continue
		}
		if userInput == "exit" {
			fmt.Print("再见！！！")
			break
		}

		// 创建消息 -- 添加到历史消息中
		history = append(history, schema.UserMessage(userInput))

		// 运行返回的事件单元
		events := runner.Run(ctx, history)

		for {
			// 获取下一个事件，阻塞直到有事件或结束
			event, ok := events.Next()
			if !ok {
				break // 迭代器关闭，全部事件已消费
			}
			if event.Err != nil {
				log.Fatal("运行 Runner 错误")
			}

			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
				resp := msg.Content
				fmt.Printf("[assistant] : %s\n", resp)

				// 会话内容加入到上下文
				history = append(history, schema.AssistantMessage(resp, nil))
			}

			// if event.Output != nil && event.Output.MessageOutput != nil {
			// 处理消息输出（可能是流式）
			// log.Println(event.Output.MessageOutput)
			// }
		}
	}

	// 【关键步骤】循环结束后必须检查错误
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "读取标准输入时发生错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("输入结束。")

}

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
