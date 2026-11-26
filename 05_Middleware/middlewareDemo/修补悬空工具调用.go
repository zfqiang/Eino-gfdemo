package middlewaredemo

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/patchtoolcalls"
	"github.com/cloudwego/eino/schema"
)

// 扫描消息历史，为“有 ToolCall 但缺少 Tool 响应”的悬空调用自动插入占位符消息，防止模型 API 报错。
func PatchToolCall() (adk.ChatModelAgentMiddleware, error) {
	// 方式一：默认配置（cfg 传 nil）
	ctx := context.Background()
	patchMW, err := patchtoolcalls.New(ctx, nil)

	// 方式二：自定义占位符内容
	patchMW, err = patchtoolcalls.New(ctx, &patchtoolcalls.Config{
		// 可选：自定义生成占位符消息的函数
		PatchedContentGenerator: func(ctx context.Context, toolName, toolCallID string) (string, error) {
			return fmt.Sprintf("[系统提示] 工具 %s 的执行被跳过（调用ID: %s）", toolName, toolCallID), nil
		},
		// 可选：自定义生成补丁工具结果的函数
		PatchedToolResultGenerator: func(ctx context.Context, toolName, toolCallID string, toolArgument *schema.ToolArgument) (*patchtoolcalls.PatchedToolResult, error) {
			return &patchtoolcalls.PatchedToolResult{}, nil
		},
	})

	return patchMW, err
}
