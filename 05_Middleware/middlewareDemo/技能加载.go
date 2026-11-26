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
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/schema"
)

func SkillDocsDemo() {
	ctx := context.Background()

	// 1. 准备技能目录结构
	// 按照规范，每个技能是 skills 目录下的一个一级子目录
	// 目录中必须包含 SKILL.md 文件
	workDir, _ := os.Getwd()
	skillsDir := filepath.Join(workDir, "skills")

	// 创建示例技能目录：skills/pdf/
	pdfSkillDir := filepath.Join(skillsDir, "pdf")
	if err := os.MkdirAll(pdfSkillDir, 0755); err != nil {
		log.Fatalf("创建技能目录失败: %v", err)
	}

	// 写入 SKILL.md（YAML frontmatter + Markdown 正文）
	// frontmatter 中的 name 和 description 是 Agent 发现技能的关键依据
	skillContent := `---
name: pdf-processing
description: 处理 PDF 文件，提取文本、合并拆分页面、填写表单。当用户需要操作 PDF 文件时使用此技能。
---

# PDF 处理技能

## 工作流程

1. 首先确认 PDF 文件路径
2. 使用 scripts/extract.py 提取文本内容
3. 根据用户需求进行后续处理

## 资源引用

- scripts/extract.py：PDF 文本提取脚本
`
	if err := os.WriteFile(filepath.Join(pdfSkillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		log.Fatalf("写入 SKILL.md 失败: %v", err)
	}

	// 2. 创建本地文件系统 Backend
	// 提供文件读取和 Glob 能力，供 Skill Backend 和 Filesystem 中间件使用
	fsBackend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		log.Fatalf("创建本地 Backend 失败: %v", err)
	}

	// 3. 创建 Skill Backend
	// NewBackendFromFilesystem 会扫描 BaseDir 下的一级子目录，读取每个子目录中的 SKILL.md
	skillBackend, err := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
		Backend: fsBackend,
		BaseDir: skillsDir,
	})
	if err != nil {
		log.Fatalf("创建 Skill Backend 失败: %v", err)
	}

	// 4. 创建 Skill 中间件
	// 使用 NewChatModelAgentMiddleware 创建非泛型版本的中间件
	skillMiddleware, err := skill.NewMiddleware(ctx, &skill.Config{
		Backend: skillBackend,
		// SkillToolName: nil, // 默认工具名为 "skill"，如有冲突可自定义
	})
	if err != nil {
		log.Fatalf("创建 Skill 中间件失败: %v", err)
	}

	// 5. 创建 Filesystem 中间件
	// Skill 中引用的 scripts、references 等资源，需要 Agent 通过文件系统工具读取或执行
	// 因此需要同时挂载 filesystem 中间件
	fsMiddleware, err := filesystem.New(ctx, &filesystem.MiddlewareConfig{
		Backend: fsBackend,
		
	})
	if err != nil {
		log.Fatalf("创建 Filesystem 中间件失败: %v", err)
	}

	// 6. 创建 Agent 并挂载两个中间件
	// 注意：skill 中间件本身不提供文件读取能力，必须配合 filesystem 中间件
	// 才能让 Agent 读取 SKILL.md 中引用的脚本和资源
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model: yourChatModel, // 替换为你的 ChatModel 实例
		Handlers: []adk.ChatModelAgentMiddleware{
			skillMiddleware, // 挂载 skill 中间件
			fsMiddleware,    // 挂载 filesystem 中间件
		},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 7. 运行 Agent
	// Agent 在启动时会通过 skillBackend.List() 获取技能列表（仅 name 和 description）
	// 当用户请求匹配某个技能时，Agent 调用 skill 工具加载完整内容
	history := []*schema.Message{
		schema.UserMessage("帮我从这个 PDF 文件里提取文本内容：/tmp/document.pdf"),
	}

	iter := agent.Run(ctx, &adk.AgentInput{Messages: history})
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
