package memorydemo

import (
	"bufio"
	"context"
	"eino-dfdemo/pkg"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const summaryContentTypeExtraKey = "_eino_summarization_content_type"

// 工具结构体
type weatherParams struct {
	City string `json:"city" jsonschema:"required,description=城市名称"`
}

// 工具方法
func getWeather(ctx context.Context, params weatherParams) (string, error) {
	// 模拟天气数据，实际可替换为 API 调用
	return fmt.Sprintf("%s：晴，25°C，湿度 60%%", params.City), nil
}

func RedisMyssqlStore() {
	ctx := context.Background()

	// 先创建天气工具；初始化失败时直接结束当前示例，避免继续把 nil 工具交给 Agent。
	getWeatherTool, err := utils.InferTool(
		"get_weather",
		"查询指定城市的天气",
		getWeather,
	)
	if err != nil {
		log.Printf("创建天气工具失败: %v", err)
		return
	}

	// 模型工厂使用 Fatal 处理创建失败，因此先创建模型，再打开数据库与 Redis，
	// 避免模型初始化失败时遗留已经打开的存储连接；该模型实例也会复用于摘要生成。
	cm := pkg.GetBaiLianLLM(ctx)

	// 显式处理存储初始化错误，避免 NewChatStoreDefault 内部 Fatal 直接 os.Exit，
	// 并在成功初始化后注册资源清理逻辑。
	store, err := pkg.NewChatStore(ctx, os.Getenv("MYSQL_DSN"), os.Getenv("REDIS_ADDR"))
	if err != nil {
		log.Printf("初始化会话存储失败: %v", err)
		return
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("关闭会话存储失败: %v", err)
		}
	}()

	sessionID := "demo-session-001"

	// 将摘要中间件挂载到 Agent。达到消息阈值时，中间件会压缩较早的上下文，
	// 并通过回调把摘要持久化；返回回调错误可让本轮执行明确暴露保存失败。
	summaryMW, err := summarization.New(ctx, &summarization.Config{
		Model: cm,
		Trigger: &summarization.TriggerCondition{
			ContextMessages: 25,
		},
		UserInstruction: "请总结以下对话历史，保留用户的核心需求、已确认的信息和待办事项。输出纯文本摘要，不要调用工具。",
		Callback: func(ctx context.Context, before, after adk.TypedChatModelAgentState[*schema.Message]) error {
			summaryText := extractSummaryText(after.Messages)
			if summaryText == "" {
				return nil
			}

			// 摘要替换掉的消息数可能因系统消息、保留消息而有差异，
			// 因此以中间件执行前后的上下文长度估算，并保证记录值至少为 1。
			msgCount := len(before.Messages) - len(after.Messages) + 1
			if msgCount < 1 {
				msgCount = len(before.Messages)
			}
			if err := store.SaveSummary(ctx, sessionID, summaryText, msgCount); err != nil {
				return fmt.Errorf("保存会话摘要失败: %w", err)
			}
			log.Printf("摘要已保存: %d 条消息 → 摘要", msgCount)
			return nil
		},
	})
	if err != nil {
		log.Printf("创建摘要中间件失败: %v", err)
		return
	}

	// 将天气工具和摘要中间件统一注册到 Agent，保证工具执行与上下文压缩都由
	// 同一个 Runner 生命周期管理。
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "AI助手",
		Description: "你是一个智能的AI助手",
		Instruction: "你是一个智能的AI助手，请根据用户输入进行回答，先给结论再解释。查询天气请调用工具get_weather。",
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{getWeatherTool},
			},
		},
		Handlers: []adk.ChatModelAgentMiddleware{summaryMW},
	})
	if err != nil {
		log.Printf("创建 ChatModelAgent 失败: %v", err)
		return
	}

	// Scanner 在标准输入结束时会返回 false；必须退出循环并检查 Err，
	// 否则忽略 EOF 会造成空输入循环或掩盖终端读取错误。
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("=== Eino ChatModelAgent + Redis/MySQL 存储 Demo ===")
	fmt.Println("输入消息开始对话（输入 quit 退出）")
	fmt.Println("会话 ID:", sessionID)
	fmt.Println()

	for {
		fmt.Print("你: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "quit" {
			break
		}

		// 运行会话
		reply, err := RunChat(ctx, agent, store, sessionID, input)
		if err != nil {
			log.Printf("对话出错: %v", err)
			continue
		}
		fmt.Printf("助手: %s\n\n", reply)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("读取用户输入失败: %v", err)
	}

	// 展示 MySQL 中保存的完整对话。Content 字段本身是 schema.Message 的
	// JSON 编码，先反序列化再只打印正文，避免把 Extra、工具字段等内部结构
	// 整段输出到终端。使用分页遍历而不是只读取第一页，确保超过 50 条时
	// 也能完整展示；最后一页不足页大小时即可确定读取结束。
	fmt.Println("\n=== 从 MySQL 读取全量历史 ===")
	const historyPageSize = 50
	for page := 1; ; page++ {
		history, err := store.GetHistory(ctx, sessionID, page, historyPageSize)
		if err != nil {
			log.Printf("读取历史第 %d 页失败: %v", page, err)
			break
		}
		for _, h := range history {
			var msg schema.Message
			if err := json.Unmarshal([]byte(h.Content), &msg); err != nil {
				log.Printf("解析历史消息 %d 失败: %v", h.ID, err)
				continue
			}
			fmt.Printf("[%s] %s\n", h.Role, msg.Content)
		}
		if len(history) < historyPageSize {
			break
		}
	}

	// 摘要查询也检查错误，区分“没有摘要”和“MySQL 查询失败”，
	// 避免将数据库故障伪装成尚未生成摘要。
	fmt.Println("\n=== 已持久化的摘要 ===")
	summary, err := store.GetLatestSummary(ctx, sessionID)
	if err != nil {
		log.Printf("读取会话摘要失败: %v", err)
		return
	}
	if summary != nil {
		fmt.Printf("摘要 (覆盖 %d 条消息): %s\n", summary.MsgCount, summary.Summary)
	} else {
		fmt.Println("暂无摘要")
	}

}

// extractSummaryText 只读取 Eino 摘要中间件标记过的摘要消息。
// 不能仅凭消息角色或正文中出现“摘要”来判断，否则普通提示或用户对话
// 恰好提到“摘要”时也可能被误存为会话摘要。
// extractSummaryText 从消息列表中提取摘要文本
// 参数:
//   msgs - 消息指针数组，每个元素都是指向Message结构体的指针
// 返回值:
//   string - 返回找到的摘要内容，如果没有找到则返回空字符串
func extractSummaryText(msgs []*schema.Message) string {
	// 遍历消息列表
	for _, m := range msgs {
		// 检查消息或其Extra字段是否为nil
		if m == nil || m.Extra == nil {
			continue
		}
		// 检查Extra中是否存在summaryContentTypeExtraKey键，并且其值为"summary"
		if contentType, ok := m.Extra[summaryContentTypeExtraKey].(string); ok && contentType == "summary" {
			// 如果找到符合条件的消息，直接返回其内容
			return m.Content
		}
	}
	// 如果没有找到符合条件的消息，返回空字符串
	return ""
}

// RunChat 执行一轮对话：加载并清洗历史、持久化用户消息、运行 Agent，
// 最后只持久化最终助手答复，避免把不完整的工具调用序列带入下一轮。
func RunChat(ctx context.Context, agent adk.Agent, store *pkg.ChatStore, sessionID, userInput string) (string, error) {
	if agent == nil {
		return "", fmt.Errorf("run chat: agent is nil")
	}
	if store == nil {
		return "", fmt.Errorf("run chat: store is nil")
	}
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return "", fmt.Errorf("run chat: user input is empty")
	}

	// 1. 从持久化存储加载近期消息及摘要，并先剔除上一版本可能遗留的
	// 工具调用中间态，保证传给模型的历史始终是完整、可接受的消息序列。
	history, err := store.GetRecentMessages(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("load history: %w", err)
	}
	history = conversationHistory(history)

	// 2. 将当前输入追加到上下文，并立即持久化，确保程序意外退出时仍可保留用户输入。
	userMsg := schema.UserMessage(userInput)
	history = append(history, userMsg)

	if err := store.SaveMessage(ctx, sessionID, userMsg); err != nil {
		return "", fmt.Errorf("save user msg: %w", err)
	}

	// 3. 每轮使用新的 Runner，但传入完整历史；Agent 在本轮内部负责工具调用、
	// 工具结果回填和最终回答生成。
	runner := adk.NewRunner(ctx,
		adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
		})

	iter := runner.Run(ctx, history)
	var finalOutput string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return "", fmt.Errorf("agent run: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return "", fmt.Errorf("read agent message: %w", err)
		}
		if msg == nil {
			continue
		}

		// 只保存不含 ToolCalls 的 assistant 消息。工具调用消息需要后续匹配的
		// ToolMessage；本示例不持久化这组内部交互，避免跨轮历史缺少 tool_call_id 响应。
		if msg.Role == schema.Assistant && len(msg.ToolCalls) == 0 {
			if err := store.SaveMessage(ctx, sessionID, msg); err != nil {
				return "", fmt.Errorf("save assistant msg: %w", err)
			}
			finalOutput = msg.Content
		}
	}

	if finalOutput == "" {
		return "", fmt.Errorf("agent run: no final assistant response")
	}
	return finalOutput, nil
}

// conversationHistory 从历史中移除无法独立重放的工具调用消息。
// assistant 的 tool_calls 必须与对应的 ToolMessage 成组出现；当前存储策略只
// 保留最终答复，因此这里也会清除遗留的工具角色消息和未完成工具调用消息。
func conversationHistory(messages []*schema.Message) []*schema.Message {
	history := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.Role == schema.Tool {
			continue
		}
		if message.Role == schema.Assistant && len(message.ToolCalls) > 0 {
			continue
		}
		history = append(history, message)
	}
	return history
}
