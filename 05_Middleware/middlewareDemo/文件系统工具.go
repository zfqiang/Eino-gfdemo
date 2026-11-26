package middlewaredemo

import (
	"context"

	"github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
)

// 向 Agent 注入 ls、read_file、write_file、edit_file、glob、grep 六个文件操作工具，可选注入 execute 执行 Shell 命令。
func FileSystem() (adk.ChatModelAgentMiddleware, error) {

	ctx := context.Background()
	myBackend, err := local.NewBackend(ctx, &local.Config{})
	if err != nil {
		panic(err)
	}

	fsMW, err := filesystem.New(ctx, &filesystem.MiddlewareConfig{
		// 必填：文件系统后端，提供 ls/read/write/edit/glob/grep 能力
		Backend: myBackend,

		// 可选：Shell 命令执行（与 StreamingShell 互斥）
		Shell: myBackend, // 设置后注册 execute 工具

		// 或使用流式 Shell（与 Shell 互斥）
		// StreamingShell: myStreamingShell,

		// 可选：启用多模态读取（read_file 支持图片/PDF）
		UseMultiModalRead: true, // 默认 false，需 Backend 实现 MultiModalReader

		// ===== 各工具独立配置 =====
		LsToolConfig: &filesystem.ToolConfig{
			Name: "list_dir",    // 覆盖工具名（默认 "ls"）
			Desc: new("列出目录内容"), // 覆盖描述
			// CustomTool: 自定义实现（替换 Backend 默认实现）
			// Disable: true, // 禁用此工具
		},
		ReadFileToolConfig: &filesystem.ToolConfig{
			// Name: "cat_file",
			// Disable: false,
		},
		WriteFileToolConfig: &filesystem.ToolConfig{
			Disable: false, // 设为 true 可禁止 Agent 写文件
		},
		EditFileToolConfig: &filesystem.ToolConfig{},
		GlobToolConfig:     &filesystem.ToolConfig{},
		GrepToolConfig:     &filesystem.ToolConfig{},

		// 可选：覆盖注入的 system prompt（默认使用内置文件工具提示词）
		// CustomSystemPrompt: ptr("你可以操作文件。"),

	})
	return fsMW, err
}
