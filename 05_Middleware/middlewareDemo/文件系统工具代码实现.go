package middlewaredemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"
	"path/filepath"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	fsMiddleware "github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/schema"
)

/*
===================================
# 文件系统应用场景
	filesystem 中间件让 Agent 具备读写本地或远程文件系统的能力。它通过 Backend 注入一组文件操作工具（ls、read_file、write_file、edit_file、glob、grep），并根据是否配置 Shell 额外注册 execute 工具。

	应用场景
		这个中间件在以下场景中很有用：
		1、代码助手：Agent 需要读取项目文件、搜索代码内容（grep）、按模式查找文件（glob），并在用户授权下修改代码（edit_file）。
		2、文档处理：Agent 需要读取日志文件、配置文件，或根据用户指令写入报告文件。
		3、运维助手：配合 StreamingShell，Agent 可以执行 tail -f 查看日志、运行诊断脚本，并实时获取输出。
===================================
*/

func MainFilesystem() {
	ctx := context.Background()

	// 1. 创建本地文件系统 Backend
	// local.NewBackend 同时实现了 filesystem.Backend（文件操作）和 filesystem.StreamingShell（流式命令执行）
	// 注意：local backend 目前仅支持 Unix/MacOS，Windows 需要自定义实现[citation:7]
	backend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		log.Fatalf("创建 local backend 失败: %v", err)
	}

	// 2. （可选）准备一个测试文件，方便 Agent 读取和搜索
	testDir := "/tmp/eino-fs-demo"
	os.MkdirAll(testDir, 0755)
	testFile := filepath.Join(testDir, "hello.go")
	os.WriteFile(testFile, []byte(`package main

import "fmt"

// TODO: 添加更多功能
func main() {
    fmt.Println("Hello, Eino Filesystem!")
}
`), 0644)

	// 3. 创建 filesystem 中间件
	// 关键配置：
	//   Backend：必填，驱动 ls/read_file/write_file/edit_file/glob/grep 六个工具
	//   StreamingShell：可选，设置后注册流式 execute 工具，允许 Agent 执行 shell 命令
	//   （Shell 与 StreamingShell 互斥，local backend 只实现了 StreamingShell[citation:2]）
	mw, err := fsMiddleware.New(ctx, &fsMiddleware.MiddlewareConfig{
		Backend:        backend,
		StreamingShell: backend, // 启用命令执行能力，Agent 可以运行 ls、cat、grep 等命令
		// CustomSystemPrompt: ptr("你可以操作文件。"), // 设为 nil 则不追加额外系统提示；也可自定义提示词约束 Agent 行为[citation:1]
	})
	if err != nil {
		log.Fatalf("创建 filesystem 中间件失败: %v", err)
	}

	// 4. 创建 Agent 并挂载中间件
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model: yourChatModel, // 替换为你的 ChatModel 实例，如 agenticdeepseek
		Handlers: []adk.ChatModelAgentMiddleware{
			mw,
		},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 5. 运行 Agent
	// 用户请求涉及：列出文件、读取内容、搜索关键词
	history := []*schema.Message{
		schema.UserMessage("/tmp/eino-fs-demo 目录下有哪些文件？帮我读取 hello.go 的内容，并搜索其中包含 TODO 的行。"),
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	iter := runner.Run(ctx, history)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Printf("执行出错: %v", event.Err)
			continue
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			fmt.Printf("Agent: %s\n", event.Output.MessageOutput.Message.Content)
		}
	}
}
