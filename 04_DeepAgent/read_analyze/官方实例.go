package readanalyze

import (
	"bufio"
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"
	"path/filepath"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/schema"
)

func GuanFangDemo() {
	fmt.Println("开始执行")
	ctx := context.Background()
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

	cm := pkg.GetBaiLianLLM(ctx)
	agent, err := deep.New(ctx, &deep.Config{
		Name:           "Ch04ToolAgent",
		Description:    "通过LocalBackend访问文件系统的ChatWithDoc代理。",
		ChatModel:      cm,
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
