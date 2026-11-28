package middlewaredemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/agentsmd"
	"github.com/cloudwego/eino/schema"
)

/*
AgentsMD（指令文件注入）  应用场景及代码实例

	AgentsMD 中间件让 Agent 能够从 Agents.md 文件中读取系统级指令和上下文，并在每次模型调用时自动注入，效果类似 Claude Code 的 CLAUDE.md。它的核心价值是让你用文件来定义 Agent 的行为规范，而无需在代码里手动拼接冗长的 system prompt。

	应用场景
	这个中间件适合以下情况：
		1、项目级行为约束：为整个项目的 Agent 定义统一的编码规范、回复格式、安全准则，所有对话共享同一份指令。
		2、团队协作：将 Agents.md 纳入版本控制，团队成员共享同一套 Agent 行为配置。
		3、动态调整指令：修改 Agents.md 文件后，下次模型调用即可生效，无需重新部署代码。

*/

// ==================== 本地文件系统 Backend 实现 ====================
// 实现 agentsmd.Backend 接口，从本地磁盘读取 Agents.md 文件

type localBackendV2 struct {
	rootDir string
}

// Read 读取文件内容
// 关键要求：文件不存在时必须返回包裹 os.ErrNotExist 的错误
// 这样 loader 会将其视为非致命警告，而不是中止加载
func (b *localBackendV2) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	filePath := req.FilePath
	// 如果是相对路径，基于 rootDir 解析
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(b.rootDir, filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 必须包裹 os.ErrNotExist，否则 loader 会把它当作致命错误
			return nil, fmt.Errorf("file not found: %w", os.ErrNotExist)
		}
		return nil, err
	}

	return &filesystem.FileContent{Content: string(data)}, nil
}

func MainAgentsMD() {
	ctx := context.Background()

	// 1. 准备 Agents.md 文件
	projectDir := "/resources/md"
	os.MkdirAll(projectDir, 0755)

	agentsMDPath := filepath.Join(projectDir, "agents.md")
	agentsMDContent := `# 项目指令

你是一个专业的 Go 后端开发助手。

请遵循以下规范：
- 所有代码必须包含中文注释
- 错误处理必须使用 fmt.Errorf 包裹，保留调用链
- 回复时先给出结论，再展开细节
`
	if err := os.WriteFile(agentsMDPath, []byte(agentsMDContent), 0644); err != nil {
		log.Fatalf("写入 agents.md 失败: %v", err)
	}

	// 2. 创建 Backend
	backend := &localBackendV2{rootDir: projectDir}

	// 3. 创建 agentsmd 中间件
	mw, err := agentsmd.New(ctx, &agentsmd.Config{
		Backend:       backend,
		AgentsMDFiles: []string{agentsMDPath}, // 至少一个文件路径
		// AllAgentsMDMaxBytes: 0,             // 0 表示无总字节限制（默认）
		// OnLoadWarning: nil,                 // 默认使用 log.Printf 打印警告
	})
	if err != nil {
		log.Fatalf("创建 agentsmd 中间件失败: %v", err)
	}

	// 4. 创建 ChatModelAgent 并挂载中间件
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model: yourChatModel, // 替换为你的 ChatModel 实例
		Handlers: []adk.ChatModelAgentMiddleware{
			mw,
		},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 5. 创建 Runner（官方推荐的生产环境执行入口）
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 6. 通过 Runner.Run 运行 Agent
	// 每次模型调用前，agentsmd 会自动将 agents.md 内容注入消息序列
	history := []*schema.Message{
		schema.UserMessage("请帮我写一个读取配置文件的函数"),
	}

	iter := runner.Run(ctx, history)

	// 7. 消费事件流
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
