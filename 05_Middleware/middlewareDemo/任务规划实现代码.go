package middlewaredemo

import (
	"context"
	"eino-dfdemo/pkg"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/plantask"
)

/*
===================================
# PlanTask（任务规划）应用场景
	plantask 中间件让 Agent 具备结构化的任务规划与追踪能力。它通过注入四个任务管理工具（TaskCreate、TaskGet、TaskUpdate、TaskList），把复杂请求拆解为可持久化的子任务，并将任务状态以 JSON 文件形式存储在文件系统中，实现跨会话、跨进程的任务管理。

	应用场景
	当用户请求涉及多步骤、有依赖关系的复杂工程时，纯靠 LLM 在上下文里“记 TODO”容易遗忘或错乱。plantask 将任务外置存储，让 Agent 随时通过工具查询和更新，适合：
		1、代码开发任务：如“实现用户登录功能”，拆解为前端页面、后端 API、数据库表设计三个子任务，并设置依赖关系（API 依赖表设计完成）。
		2、多 Agent 协作：不同子 Agent 通过共享的任务列表感知全局进度，避免重复或冲突。
		3、长周期任务：进程重启后，Agent 再次调用 TaskList 即可恢复所有任务状态，不依赖对话历史。
===================================
*/

// ==================== 内存版 plantask.Backend 实现 ====================
// 生产环境建议使用 local.NewBackend 或自定义持久化存储
// 这里用内存 map 简化示例，保证代码可直接运行

type memBackend struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func newMemBackend() *memBackend {
	return &memBackend{files: make(map[string][]byte)}
}

// LsInfo 列出目录下的文件
func (b *memBackend) LsInfo(ctx context.Context, req *plantask.LsInfoRequest) ([]plantask.FileInfo, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var infos []plantask.FileInfo
	for path := range b.files {
		infos = append(infos, plantask.FileInfo{Path: path, IsDir: false})
	}
	return infos, nil
}

// Read 读取文件内容
func (b *memBackend) Read(ctx context.Context, req *plantask.ReadRequest) (*filesystem.FileContent, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	data, ok := b.files[req.FilePath]
	if !ok {
		return nil, fmt.Errorf("file not found: %w", errNotExist)
	}
	return &filesystem.FileContent{Content: string(data)}, nil
}

// Write 写入文件内容
func (b *memBackend) Write(ctx context.Context, req *plantask.WriteRequest) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files[req.FilePath] = []byte(req.Content)
	return nil
}

// Delete 删除文件
func (b *memBackend) Delete(ctx context.Context, req *plantask.DeleteRequest) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.files, req.FilePath)
	return nil
}

// 简单的错误变量，用于模拟文件不存在
var errNotExist = fmt.Errorf("not exist")

func MainTaskPlanning() {
	ctx := context.Background()

	// 1. 创建内存 Backend（每个 session 应使用独立实例）
	backend := newMemBackend()
	baseDir := "/tasks/session-001"

	// 2. 创建 plantask 中间件
	// 它会通过 BeforeAgent 钩子向 Agent 注入四个任务管理工具
	planTaskMw, err := plantask.New(ctx, &plantask.Config{
		Backend: backend, // 任务存储后端
		BaseDir: baseDir, // 任务 JSON 文件存储目录
	})
	if err != nil {
		log.Fatalf("创建 plantask 中间件失败: %v", err)
	}

	// 3. 创建 ChatModelAgent 并挂载中间件
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       yourChatModel, // 替换为你的 ChatModel 实例
		Instruction: "你是一个任务规划助手。对于复杂请求，请使用 TaskCreate 创建子任务，用 TaskUpdate 更新状态，用 TaskList 查看进度。",
		Handlers: []adk.ChatModelAgentMiddleware{
			planTaskMw,
		},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 4. 创建 Runner（官方推荐的生产环境执行入口）
	// Runner 负责管理 Agent 生命周期，plantask 中间件在 Agent 执行时自动生效
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 5. 通过 Runner.Query 运行 Agent
	// 用户请求会触发 Agent 自主拆解任务、创建任务列表、更新进度
	iter := runner.Query(ctx, "帮我规划一个用户登录功能的开发任务，需要拆解为可跟踪的子任务，包含前端页面、后端 API、数据库表设计三部分，并设置合理的依赖关系。")

	// 6. 消费事件流
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Printf("执行出错: %v", event.Err)
			continue
		}
		// 提取 Agent 的文本输出
		if event.Output != nil && event.Output.MessageOutput != nil {
			mv := event.Output.MessageOutput
			if mv.Message != nil && mv.Message.Content != "" {
				fmt.Printf("Agent: %s\n", mv.Message.Content)
			}
		}
	}

	// 7. （可选）验证任务是否已持久化
	// 通过 backend 直接查看存储的任务文件
	fmt.Println("\n=== 任务存储验证 ===")
	infos, _ := backend.LsInfo(ctx, &plantask.LsInfoRequest{Path: baseDir})
	for _, info := range infos {
		content, _ := backend.Read(ctx, &plantask.ReadRequest{FilePath: info.Path})
		var task map[string]interface{}
		json.Unmarshal([]byte(content.Content), &task)
		fmt.Printf("任务文件 %s: %v\n", info.Path, task)
	}
}
