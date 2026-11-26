package middlewaredemo

import (
	"context"
	"eino-dfdemo/pkg"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Summarization（摘要压缩）
func Summarization() (adk.ChatModelAgentMiddleware, error) {
	ctx := context.Background()
	cm := pkg.GetBaiLianLLM(ctx)

	// 构造默认后处理器（显式传入你需要的配置）
	summarization.NewFinalizer().Build()

	summaryMW, err := summarization.New(ctx, &summarization.Config{
		// 必填：用于生成摘要的模型（通常复用主模型）
		Model: cm,

		// 可选：传给摘要模型的额外选项
		ModelOptions: []model.Option{},

		// 触发条件：满足任一条件即触发摘要
		Trigger: &summarization.TriggerCondition{
			ContextTokens:   160000, // Token 数超过此值触发（默认 160k）
			ContextMessages: 30,     // 消息数超过此值触发（与 Tokens 任一满足即可）
		},

		// 可选：自定义摘要指令，覆盖内置 prompt
		UserInstruction: "请总结以下对话，保留用户核心需求、已确认信息和待办事项。不调用任何工具。",

		// 可选：摘要后保留最近用户原始消息，避免丢失最新意图
		// PreserveUserMessages: &summarization.PreserveUserMessages{
		// 	Enabled:   true, // 启用（默认就是 true）
		// 	MaxTokens: 5000, // 保留用户消息的 Token 上限，默认是 Trigger.ContextTokens/3
		// 	// Filter: 可选，自定义过滤哪些用户消息被保留
		// },

		// 可选：重试策略（默认不重试）  ShouldRetry 决定了是否要进行重试  BackoffFunc 只决定重试时等多久
		Retry: &summarization.RetryConfig{
			MaxRetries: new(3), // 最多重试 3 次
			ShouldRetry: func(ctx context.Context, resp *schema.Message, err error) bool {
				if err == nil {
					return false
				}
				// 伪代码：只对限流或超时重试
				// return isRateLimitError(err) || isTimeoutError(err)

				// 没报错，但摘要内容为空或过短，也重试
				if resp == nil || len(resp.Content) < 10 {
					return true
				}
				return false
			}, // 自定义重试条件 默认 err != nil 时重试
			BackoffFunc: func(ctx context.Context, attempt int, resp *schema.Message, err error) time.Duration {
				// 第1次重试等1秒，第2次等2秒，以此类推
				return time.Duration(attempt) * time.Second
			}, // 默认指数退避 + 抖动
		},

		// 可选：降级策略（主模型失败时切换备用模型）
		Failover: &summarization.FailoverConfig{
			MaxRetries: new(2), // 最多重试 2 次
			ShouldFailover: func(ctx context.Context, resp *schema.Message, err error) bool {
				if err == nil {
					return false
				}
				// 伪代码：只对限流或超时重试
				// return isRateLimitError(err) || isTimeoutError(err)

				// 没报错，但摘要内容为空或过短，也重试
				if resp == nil || len(resp.Content) < 10 {
					return true
				}
				return false
			}, // 自定义重试条件 默认 err != nil 时重试
			BackoffFunc: func(ctx context.Context, attempt int, resp *schema.Message, err error) time.Duration {
				// 第1次重试等1秒，第2次等2秒，以此类推
				return time.Duration(attempt) * time.Second
			}, // 默认指数退避 + 抖动
			GetFailoverModel: func(ctx context.Context, failoverCtx *summarization.TypedFailoverContext[*schema.Message]) (failoverModel model.BaseModel[*schema.Message], failoverModelInputMsgs []*schema.Message, failoverErr error) {
				// failoverCtx.Attempt 表示当前降级尝试次数
				backupModel := pkg.NewChatModel(ctx)
				return backupModel, failoverCtx.OriginalMessages, nil
			},
		},

		// 可选：摘要前后回调，可用于持久化摘要结果
		Callback: func(ctx context.Context, before, after adk.ChatModelAgentState) error {
			// before.Messages → after.Messages 是压缩前后的状态
			// 可在此将摘要写入 MySQL 等存储
			return nil
		},

		// 可选：是否发送内部事件（用于调试/监控）
		EmitInternalEvents: false,

		// 可选：摘要生成完成后，对摘要消息进行最终处理
		Finalize: func(ctx context.Context, originalMessages []*schema.Message, summary *schema.Message) ([]*schema.Message, error) {
			return []*schema.Message{summary}, nil
		},
	})
	// 注入：Middlewares: []adk.ChatModelAgentMiddleware{summaryMW}
	return summaryMW, err
}
