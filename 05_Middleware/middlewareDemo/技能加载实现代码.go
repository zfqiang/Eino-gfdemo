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
	skill "github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/schema"
)

/*
# Skill（技能加载）应用场景
	Skill 中间件让 Agent 能够按需加载“技能文档”。它解决的场景是：有些知识或操作手册（如“如何用 Eino 写代码”、“如何调用某个内部系统 API”）篇幅较长，不适合全部塞进系统提示词里。Skill 把这些内容做成独立的 SKILL.md 文件，Agent 启动时只看到每个技能的 name 和 description，当任务匹配时，才通过 skill 工具把完整内容加载进上下文。

	应用场景
		1、框架/内部系统使用指南：把 Eino 框架的用法、公司内部 API 的调用规范写成 Skill，Agent 遇到相关问题时自动查阅。
		2、复杂操作流程：把“如何发布一个版本”、“如何处理某类工单”的多步骤流程写成 Skill，避免每次都在提示词里重复。
		3、按需加载节省上下文：技能文档可能很长，但大多数对话用不到。渐进式披露（Progressive Disclosure）让 Agent 只在需要时加载，不浪费 token。

*/

func main() {
	ctx := context.Background()

	// 1. 准备技能目录结构
	// 按照约定，每个技能是 skills 目录下的一个一级子目录，其中包含 SKILL.md
	workDir, _ := os.Getwd()
	skillsDir := filepath.Join(workDir, "skills")

	// 创建示例技能：skills/go-review/
	skillDir := filepath.Join(skillsDir, "go-review")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		log.Fatalf("创建技能目录失败: %v", err)
	}

	// 写入 SKILL.md（YAML frontmatter + Markdown 正文）
	skillContent := `---
name: go-code-review
description: 对 Go 代码进行审查，检查错误处理、并发安全、命名规范。当用户需要审查 Go 代码时使用此技能。
---

# Go 代码审查技能

## 检查清单

1. 错误处理：是否所有 error 都被检查？是否用 %w 包裹？
2. 并发安全：map 是否有锁保护？goroutine 是否泄漏？
3. 命名规范：导出的函数是否有注释？变量名是否符合 Go 惯例？
4. 资源管理：defer 是否在正确位置？文件/连接是否正确关闭？
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		log.Fatalf("写入 SKILL.md 失败: %v", err)
	}

	// 2. 创建本地文件系统 Backend
	// 它提供文件读取和 Glob 能力，供 Skill Backend 使用
	fsBackend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		log.Fatalf("创建本地 Backend 失败: %v", err)
	}

	// 3. 创建 Skill Backend
	// 扫描 BaseDir 下的一级子目录，读取每个子目录中的 SKILL.md
	skillBackend, err := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
		Backend: fsBackend,
		BaseDir: skillsDir,
	})
	if err != nil {
		log.Fatalf("创建 Skill Backend 失败: %v", err)
	}

	// 4. 创建 Skill 中间件
	// 使用 NewMiddleware
	skillMw, err := skill.NewMiddleware(ctx, &skill.Config{
		Backend: skillBackend,
	})
	if err != nil {
		log.Fatalf("创建 Skill 中间件失败: %v", err)
	}

	// 5. 创建 Agent 并挂载中间件
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model: yourChatModel, // 替换为你的 ChatModel 实例
		Handlers: []adk.ChatModelAgentMiddleware{
			skillMw,
		},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 6. 创建 Runner（官方推荐的生产环境执行入口）
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 7. 通过 Runner.Run 运行 Agent
	// Agent 启动时只看到技能的 name 和 description
	// 当用户请求匹配时，Agent 调用 skill 工具加载完整内容
	history := []*schema.Message{
		schema.UserMessage("请帮我审查一下这段 Go 代码：func main() { var m map[string]int; m[\"a\"] = 1 }"),
	}

	iter := runner.Run(ctx, history)

	// 8. 消费事件流
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
			msg := event.Output.MessageOutput.Message
			if msg != nil && msg.Content != "" {
				fmt.Printf("Agent: %s\n", msg.Content)
			}
		}
	}
}
