package middlewaredemo

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	"github.com/cloudwego/eino/schema"
)

// 管理工具返回的长结果：截断超长内容并转存文件，清理历史工具结果释放 Token。
func ToolZip() (adk.ChatModelAgentMiddleware, error) {
	ctx := context.Background()
	backend, err := local.NewBackend(ctx, &local.Config{})
	if err != nil {
		panic(err)
	}

	reductionMW, err := reduction.New(ctx, &reduction.Config{
		// ========== 存储后端（必填） ==========
		// 被截断或清理的完整内容会写入此 Backend
		// 注意：如果 SkipTruncation=true 且仅做清理且不需要 offload，可设为 nil
		Backend: backend,

		// ========== 阶段开关 ==========
		SkipTruncation: false, // 设为 true 则跳过截断阶段
		SkipClear:      false, // 设为 true 则跳过清理阶段

		// ========== 截断配置（Truncation） ==========
		// 单条工具输出超过此字符数时触发截断，默认 50000
		MaxLengthForTrunc: 30000, // 示例：收紧到 30000 字符，更早触发截断

		// 不参与截断的工具名列表
		TruncExcludeTools: []string{"read_file", "list_dir"}, // 读取类工具的输出不截断，避免 Agent 无法获取完整内容

		// ========== 清理配置（Clear） ==========
		// 对话总 token 超过此阈值时触发清理，默认 160000（旧版文档写 30000，以代码为准）
		MaxTokensForClear: 100000, // 示例：10 万 token 时开始清理

		// 保留最近 N 轮 assistant 消息不被清理，默认 1
		ClearRetentionSuffixLimit: 2, // 保留最近 2 轮，给模型更多即时上下文

		// 清理至少释放的 token 量，未达标则不执行清理（避免无谓破坏 prompt cache），默认 0
		ClearAtLeastTokens: 5000, // 至少释放 5000 token 才真正执行清理

		// 不参与清理的工具名列表
		ClearExcludeTools: []string{"search_docs"}, // 搜索工具的结果可能后续还需要参考，不清除

		// ========== 文件读取工具 ==========
		// Agent 通过此工具名读取被卸载到 Backend 的内容
		// 如果你用了 filesystem 中间件，默认就是 "read_file"，保持一致即可
		ReadFileToolName: "read_file",

		// ========== 存储路径 ==========
		// 截断内容存到 {RootDir}/trunc/{tool_call_id}
		// 清理内容存到 {RootDir}/clear/{tool_call_id}
		RootDir: "/tmp/eino_reduction", // 示例：改为自定义目录

		// ========== Token 计数（强烈建议自定义） ==========
		// 默认用 字符数/4 粗略估算，误差较大
		// 建议替换为 tiktoken-go 等精确分词器
		TokenCounter: func(ctx context.Context, msgs []adk.Message, tools []*schema.ToolInfo) (int64, error) {
			// 示例：仍用粗略估算，实际项目请替换
			var total int64
			for _, m := range msgs {
				if m != nil {
					total += int64(len(m.Content) / 4)
				}
			}
			return total, nil
		},

		// ========== 清理后置回调 ==========
		// 清理完成后调用，可用于保存状态或发送通知
		ClearPostProcess: func(ctx context.Context, state *adk.ChatModelAgentState) context.Context {
			fmt.Printf("清理完成，当前消息数: %d\n", len(state.Messages))
			return ctx
		},

		// ========== 按工具覆盖配置 ==========
		// 优先级高于全局配置，可为特定工具单独设置 Backend、跳过策略或自定义 Handler
		ToolConfig: map[string]*reduction.ToolReductionConfig{
			"web_search": {
				SkipTruncation: true, // 搜索工具的输出不做截断，直接进入清理逻辑
				// Backend 不设置则沿用全局 Backend
				TruncHandler: func(ctx context.Context, detail *reduction.ToolDetail) (*reduction.TruncResult, error) {
					// detail.ToolResult 包含原始工具输出
					fmt.Printf("搜索工具输出截断：%v\n", detail.ToolResult)
					// 返回截断结果，NeedTrunc 控制是否真正截断
					return &reduction.TruncResult{
						NeedTrunc: true,
						// Backend 写入等由中间件后续处理
					}, nil
				},
			},
			"execute_sql": {
				ClearHandler: func(ctx context.Context, detail *reduction.ToolDetail) (*reduction.ClearResult, error) {
					// 自定义清理逻辑：SQL 查询结果可以只保留前 10 行
					// 返回 nil 表示使用默认清理 handler
					return nil, nil
				},
			},
		},
	})
	return reductionMW, err
}
