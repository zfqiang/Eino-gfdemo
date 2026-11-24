package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
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
	Demo()
}

func Demo() {
	fmt.Println("开始执行")
	backend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		fmt.Println("创建后端失败")
	}

	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectRoot = cwd
		}
	}
	if abs, err := filepath.Abs(projectRoot); err == nil {
		projectRoot = abs
	}

	defaultInstruction := fmt.Sprintf(`你是一个乐于助人的助手，帮助用户学习Eino框架。

重要提示：在使用文件系统工具（如ls、read_file、glob、grep等）时，必须使用绝对路径。

项目根目录为：%s

- 当用户要求列出“当前目录”中的文件时，使用路径：%s - 当用户要求读取带有相对路径的文件时，通过在前面添加%s将其转换为绝对路径 - 示例：如果用户说“读取main.go”，则应使用文件路径“%s/main.go”调用read_file函数

在调用文件系统工具时，请始终使用绝对路径。`, projectRoot, projectRoot, projectRoot, projectRoot)

	agent, err := deep.New(ctx, &deep.Config{
		Name:           "Ch04ToolAgent",
		Description:    "通过LocalBackend访问文件系统的ChatWithDoc代理。",
		ChatModel:      model,
		Instruction:    defaultInstruction,
		Backend:        backend, // 提供文件系统操作能力
		StreamingShell: backend, // 提供命令执行能力
		MaxIteration:   10,
	})
	if err != nil {
		fmt.Println("创建代理失败", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	history := []adk.Message{}
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
				log.Fatal("运行 Runner 错误", event.Err)
			}
			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil {
				resp := msg.Content
				fmt.Printf("Agent: %s\n", resp)
				history = append(history, []adk.Message{schema.AssistantMessage(resp, nil)}...)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal("读取用户输入错误")
	}
	fmt.Println("结束执行")

}
