package middlewaredemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"strings"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

/*
===========================================
# 工具结果压缩应用场景
	ToolReduction 中间件解决的是 Agent 对话中工具返回结果过大导致上下文膨胀的问题。当一个工具（如读取大文件、查询数据库、调用返回大量 JSON 的 API）返回了数万字符的内容时，如果直接塞进对话历史，不仅浪费 token，还可能挤占后续推理的空间。reduction 通过截断（Truncation） 和清理（Clear） 两个阶段来管理工具结果的 token 占用。

应用场景
	1、文件读取工具返回大文件：Agent 读取一个日志文件或代码文件，返回内容超过数万字符。
	2、数据库/API 查询返回大量记录：查询接口返回数百条 JSON 记录，完整内容远超模型单次调用的合理 token 预算。
	3、多次工具调用累积膨胀：Agent 在多轮 ReAct 循环中反复调用工具，历史中的工具结果越来越多，总 token 逼近上下文上限。

===========================================


*/

// ==================== 模拟一个返回超大结果的工具 ====================

type BigDataInput struct {
	Count int `json:"count" jsonschema:"description=返回的记录条数"`
}

// fetchBigData 模拟一个返回大量数据的 API 或数据库查询
func fetchBigData(ctx context.Context, input *BigDataInput) (string, error) {
	// 生成 count 条模拟记录，每条约 200 字符，总量很容易超过截断阈值
	var sb strings.Builder
	for i := 0; i < input.Count; i++ {
		sb.WriteString(fmt.Sprintf(
			`{"id":%d,"name":"record_%d","description":"这是一条模拟的业务数据记录，包含足够多的字段内容以触发截断机制","status":"active","created_at":"2026-01-01T00:00:00Z"}`,
			i, i,
		))
		if i < input.Count-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

func MainToolZip() {
	ctx := context.Background()

	// 1. 创建本地文件系统 Backend
	// reduction 需要它来存储被截断/清理的内容，Agent 后续可通过 read_file 工具取回
	backend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		log.Fatalf("创建 local backend 失败: %v", err)
	}

	// 2. 创建大结果工具
	bigTool, err := utils.InferTool(
		"fetch_big_data",
		"获取大量业务数据记录，返回完整 JSON 列表。当需要分析批量数据时使用。",
		fetchBigData,
	)
	if err != nil {
		log.Fatalf("创建工具失败: %v", err)
	}

	// 3. 创建 reduction 中间件
	// 关键配置说明：
	//   MaxLengthForTrunc: 3000 —— 单条工具输出超过 3000 字符立即截断
	//   MaxTokensForClear: 5000 —— 对话总 token 超过 5000 时触发清理
	//   实际生产环境建议根据模型上下文窗口和业务场景调整
	reductionMw, err := reduction.New(ctx, &reduction.Config{
		Backend:           backend,
		MaxLengthForTrunc: 3000,        // 示例值调小，便于观察效果；默认 50000
		MaxTokensForClear: 5000,        // 示例值调小，便于观察效果；默认 160000
		ReadFileToolName:  "read_file", // 与 filesystem 中间件的工具名保持一致
	})
	if err != nil {
		log.Fatalf("创建 reduction 中间件失败: %v", err)
	}

	// 4. 创建 Agent
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model: yourChatModel, // 替换为你的 ChatModel 实例
		Handlers: []adk.ChatModelAgentMiddleware{
			reductionMw,
		},
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{bigTool},
			},
		},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 5. 运行 Agent
	// 用户要求获取 50 条记录，工具返回的 JSON 总量会远超 3000 字符
	// reduction 会在工具返回时立即截断，将完整内容存入 backend
	history := []*schema.Message{
		schema.UserMessage("帮我获取 50 条业务数据记录，并简要总结一下这些数据的整体情况。"),
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
