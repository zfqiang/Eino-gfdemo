package middlewaredemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/patchtoolcalls"
	"github.com/cloudwego/eino/schema"
)

/*
===================================
# PatchToolCalls（修补悬空工具调用）应用场景
	PatchToolCalls 中间件解决的是对话历史中“悬空工具调用”问题：Assistant 消息里包含了一个工具调用（ToolCall），但历史中缺少与之对应的工具结果（ToolResult）。这种不完整的消息序列会导致部分模型 API 直接报错，或让模型产生困惑。该中间件在每次模型调用前自动扫描并插入占位符结果，保证消息序列的完整性。

	应用场景
	以下三种情况最容易产生悬空工具调用，也是这个中间件发挥作用的地方：
		1、用户中途打断：Agent 刚发起工具调用，用户就发送了新消息，工具执行被中断，结果从未写入历史。
		2、会话恢复时结果丢失：从存储中恢复对话时，部分工具调用的结果因故未能持久化。
		3、Human-in-the-loop 取消执行：在需要人工审批的场景中，用户拒绝了工具执行，但历史中仍然保留着那个未被响应的调用。
===================================
*/

func MainPatchToolCalls() {
	ctx := context.Background()

	// 1. 创建 PatchToolCalls 中间件
	// 使用默认配置即可，cfg 传 nil
	// 默认占位符消息：工具调用 xxx（ID 为 yyy）已被取消——在其完成之前收到了另一条消息。
	patchMw, err := patchtoolcalls.New(ctx, nil)
	if err != nil {
		log.Fatalf("创建 PatchToolCalls 中间件失败: %v", err)
	}

	// 2. 创建 ChatModelAgent 并挂载中间件
	// 注意：PatchToolCalls 应放在中间件链的最前面，确保在其他中间件处理消息之前修复悬空调用
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model: yourChatModel, // 替换为你的 ChatModel 实例
		Handlers: []adk.ChatModelAgentMiddleware{
			patchMw, // 先修复消息
		},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	// 3. 创建 Runner
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	// 4. 构造一段包含悬空工具调用的历史消息
	// 场景：Assistant 发起了 get_weather 工具调用，但用户紧接着发了新消息
	// 工具结果从未写入历史，形成“悬空调用”
	history := []*schema.Message{
		schema.UserMessage("北京天气怎么样？"),
		{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call_weather_001",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"city":"北京"}`,
					},
				},
			},
		},
		// 注意：这里没有对应的 Tool 消息（ToolCallID: "call_weather_001"）
		// 正常的历史应该是：Assistant(ToolCall) -> Tool(ToolResult) -> Assistant(回复)
		// 当前历史缺少中间那一步，PatchToolCalls 会自动补上
		schema.UserMessage("算了，先帮我查一下上海的吧"),
	}

	// 5. 通过 Runner.Run 继续执行
	// Runner 内部会触发 Agent 的 BeforeModelRewriteState 钩子
	// PatchToolCalls 扫描到 call_weather_001 缺少结果，自动插入占位符 Tool 消息
	iter := runner.Run(ctx, history)

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
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg := event.Output.MessageOutput.Message
			if msg != nil && msg.Content != "" {
				fmt.Printf("Agent: %s\n", msg.Content)
			}
		}
	}
}
