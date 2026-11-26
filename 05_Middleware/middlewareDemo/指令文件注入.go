package middlewaredemo

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/agentsmd"
)

// 将 Agents.md 文件内容自动注入到消息序列中，为 Agent 提供项目级指令（类似 Claude Code 的 CLAUDE.md）
func CommandInjection() (adk.ChatModelAgentMiddleware, error) {
	ctx := context.Background()

	myBackend := &localBackend{
		rootDir: "/project",
	}

	agentsMDMW, err := agentsmd.New(ctx, &agentsmd.Config{
		// 必填：文件读取后端
		Backend: myBackend,

		// 必填：要加载的 Agents.md 文件路径列表（至少一个）
		AgentsMDFiles: []string{
			"/project/agents.md",
			"/project/rules/coding.md",
		},

		// 可选：所有文件总字节上限（0 表示不限制）
		// 超出后后续文件跳过，但每个文件始终完整加载
		AllAgentsMDMaxBytes: 0,

		// 可选：非致命错误回调（文件缺失、循环 @import 等）
		OnLoadWarning: func(filePath string, err error) {
			log.Printf("加载 %s 警告: %v", filePath, err)
		},
	})
	return agentsMDMW, err
}

// localBackend 实现 agentsmd.Backend 接口
// 用于从本地文件系统读取 Agents.md 文件
type localBackend struct {
	rootDir string
}

// Read 实现 agentsmd.Backend 接口
// 关键要求：文件不存在时必须返回包裹 os.ErrNotExist 的错误
func (b *localBackend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	// 如果传入的是相对路径，基于 rootDir 解析
	filePath := req.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(b.rootDir, filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		// 文件不存在时，必须返回包裹 os.ErrNotExist 的错误
		// 这样 loader 会将其视为非致命警告（通过 OnLoadWarning 回调）
		// 而不会中止整个加载流程
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("文件不存在: %w", os.ErrNotExist)
		}
		// 其他错误（权限被拒、I/O 错误）会中止加载
		return nil, err
	}

	return &filesystem.FileContent{Content: string(data)}, nil
}
