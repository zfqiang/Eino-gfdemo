package middlewaredemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
)

/*
===================================
# Summarization（摘要压缩）应用场景
	Summarization 中间件解决的是 Agent 长对话场景下的上下文窗口溢出问题。当对话历史的 token 数超过阈值时，它会自动调用一个摘要模型，把冗长的历史压缩成一段精简的摘要，同时保留最近的关键用户消息，让对话能够继续下去而不丢失核心上下文。

	应用场景
		1、长时间 ReAct 循环：Agent 反复调用工具、迭代多轮后，历史消息迅速膨胀。
		2、持续多轮对话：用户与 Agent 进行几十轮交互，早期对话虽然不再需要逐字保留，但其中的关键信息（用户偏好、已确认的事实）仍需维持。
		3、文档/报告生成任务：Agent 在生成长篇内容时需要记住已写部分的结构和要点，但不需要保留每一次工具调用的完整输出。
===================================
*/

// ==================== 模拟一个被反复调用的工具，用于累积对话历史 ====================

type ExpandInput struct {
	Topic string `json:"topic" jsonschema:"required,description=要扩写的话题"`
}

// expandContent 每次调用返回一段较长的文本，多轮调用后迅速推高 token 数
func expandContent(ctx context.Context, input *ExpandInput) (string, error) {
	// 模拟返回大段内容，每轮约 500+ 字符
	return fmt.Sprintf(
		"关于「%s」的详细展开：这是模拟的长文本内容，用于在多次工具调用后累积对话历史的 token 数量。"+
			"在实际场景中，这可能是数据库查询结果、文件读取内容或 API 返回的大量数据。"+
			"通过反复调用此工具，Agent 的历史消息会快速增长，从而触发 summarization 中间件的压缩逻辑。"+
			"当前话题：%s。",
		input.Topic, input.Topic,
	), nil
}

func MainSummarization() {
	ctx := context.Background()

	// 1. 创建摘要模型（使用与主 Agent 相同的模型，也可以单独指定一个更便宜的模型）
	// 这里假设你已经有了一个 ChatModel 实例，命名为 yourChatModel
	summaryModel := pkg.GetBaiLianLLM(ctx)

	// 2. 创建 summarization 中间件
	// 关键配置说明：
	//   Trigger.ContextTokens: 触发摘要的 token 阈值。示例设为 3000 便于快速触发，生产环境通常 100000+
	//   PreserveUserMessages: 默认启用，保留最近的用户消息，防止用户原始意图被摘要丢失
	sumMw, err := summarization.New(ctx, &summarization.Config{
		Model: summaryModel,
		Trigger: &summarization.TriggerCondition{
			ContextTokens:   3000, // 示例值，便于观察效果；生产环境建议 100000 以上
			ContextMessages: 30,   // 消息数超过此值触发（与 Tokens 任一满足即可）
		},
		// 可选：自定义摘要指令，覆盖内置 prompt
		UserInstruction: "请总结以下对话，保留用户核心需求、已确认信息和待办事项。不调用任何工具。",
	})
	if err != nil {
		log.Fatalf("创建 summarization 中间件失败: %v", err)
	}

	// 3. 创建扩写工具
	expandTool, err := utils.InferTool(
		"expand_topic",
		"根据给定话题返回一段扩写内容。当需要丰富报告细节时使用。",
		expandContent,
	)
	if err != nil {
		log.Fatalf("创建工具失败: %v", err)
	}

	// 4. 创建 ChatModelAgent，挂载 summarization 中间件
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:       yourChatModel,
		Instruction: "你是一个报告撰写助手。请反复调用 expand_topic 工具来扩写内容，每次调用后继续下一轮，直到完成报告。",
		Handlers: []adk.ChatModelAgentMiddleware{
			sumMw, // 摘要中间件
		},
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{expandTool},
			},
		},
		MaxIterations: 20, // 允许足够多的迭代，让历史有机会增长到触发阈值
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 5. 创建 Runner（官方推荐的生产环境执行入口）
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 6. 通过 Runner 运行 Agent
	// Query 方法直接发送用户问题，Runner 内部会管理 Agent 的完整生命周期
	iter := runner.Query(ctx, "请帮我撰写一份关于人工智能发展历史的详细报告，至少分 10 个部分展开。")

	var finalContent string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Printf("执行出错: %v", event.Err)
			continue
		}
		// 提取 Agent 的输出消息
		if event.Output != nil {
			if msg, _, getErr := adk.GetMessage(event); getErr == nil && msg != nil {
				finalContent = msg.Content
				fmt.Printf("Agent: %s\n", msg.Content)
			}
		}
	}

	// 输出最终结果摘要
	if finalContent != "" {
		fmt.Printf("\n=== 最终输出（前 200 字符）===\n%s\n", truncate(finalContent, 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}
