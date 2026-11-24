package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	commontool "github.com/cloudwego/eino-examples/adk/common/tool"
	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	clc "github.com/cloudwego/eino-ext/callbacks/cozeloop"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var ctx context.Context
var model *openai.ChatModel
var err error

func init() {
	ctx = context.Background()

	if err = godotenv.Load(); err != nil {
		log.Fatalf("加载 .env 文件失败: %v", err)
	}

	for _, name := range []string{"DASHSCOPE_API_KEY", "DASHSCOPE_MODE", "DASHSCOPE_BASE_URL"} {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			log.Fatalf("缺少环境变量 %s", name)
		}
	}

	// 创建模型
	model, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
		Model:   os.Getenv("DASHSCOPE_MODE"),
		BaseURL: os.Getenv("DASHSCOPE_BASE_URL"),

		Timeout: 30 * time.Second,

		// // 生成参数
		Temperature: new(float32(0.7)), // 控制输出随机性，范围 [0.0, 2.0]，越高越随机
		TopP:        new(float32(0.7)), // 核采样参数，范围 [0.0, 1.0]，越低越聚焦
		MaxTokens:   new(int(500)),     // 限制最大生成 token 数量，范围 [1, 8192]

		// // 停止序列 - 遇到这些文本时停止生成
		// Stop: []string{"\\n\\n", "总结:"},

		// // 惩罚参数 - 控制重复度
		// PresencePenalty:  new(float32(0.6)), // 存在惩罚，范围 [-2.0, 2.0]，正值增加新话题可能性
		// FrequencyPenalty: new(float32(0.5)), // 频率惩罚，范围 [-2.0, 2.0]，正值减少重复词语
	})
	if err != nil {
		log.Fatal("阿里云百炼  创建 ChatModel 失败")
	}
	fmt.Println("阿里云百炼  创建 ChatModel 成功")
}
func main() {
	Demo()
}

func Demo() {
	fmt.Println("开始执行")

	// 1. Redis 会话存储
	store := NewRedisSessionStore("127.0.0.1:6379", 86400)
	sessionID := "sess_002" // 业务侧自己生成，比如用户+会话ID

	backend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		log.Printf("创建后端失败: %v", err)
		return
	}

	// 创建 CozeLoop 客户端
	cozeClient, err := cozeloop.NewClient(
		cozeloop.WithAPIToken(os.Getenv("COZELOOP_API_TOKEN")),
		cozeloop.WithWorkspaceID(os.Getenv("COZELOOP_WORKSPACE_ID")),
	)

	if err != nil {
		log.Fatal("创建 CozeLoop Client 失败", err)
	}

	defer func() {
		time.Sleep(5 * time.Second)
		cozeClient.Close(ctx)
	}()

	// 使用 NewHandlerHelper 注册感兴趣的回调  注意：RunInfo 可能为 nil（如顶层调用没有 RunInfo），使用前请检查。
	myhandler := callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			log.Printf("[trace] %s/%s start", info.Component, info.Name)
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			log.Printf("[trace] %s/%s end", info.Component, info.Name)
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			log.Printf("[trace] %s/%s error: %v", info.Component, info.Name, err)
			return ctx
		}).
		Build()

	// 注册为全局 Callback
	callbacks.AppendGlobalHandlers(clc.NewLoopHandler(cozeClient), myhandler)
	log.Println("CozeLoop tracing enabled")

	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectRoot = cwd
		}
	}
	if abs, err := filepath.Abs(projectRoot); err == nil {
		projectRoot = abs
	}

	defaultInstruction := fmt.Sprintf(`You are a helpful assistant that helps users learn the Eino framework.

IMPORTANT: When using filesystem tools (ls, read_file, glob, grep, etc.), you MUST use absolute paths.

The project root directory is: %s

- When the user asks to list files in "current directory", use path: %s
- When the user asks to read a file with a relative path, convert it to absolute path by prepending %s
- Example: if user says "read main.go", you should call read_file with file_path: "%s/main.go"

Always use absolute paths when calling filesystem tools.`, projectRoot, projectRoot, projectRoot, projectRoot)

	agent, err := deep.New(ctx, &deep.Config{
		Name:           "Ch04ToolAgent",
		Description:    "ChatWithDoc agent with CozeLoop tracing.",
		ChatModel:      model,
		Instruction:    defaultInstruction,
		Backend:        backend, // 提供文件系统操作能力
		StreamingShell: backend, // 提供命令执行能力
		MaxIteration:   10,      // 最大迭代次数
		Handlers: []adk.ChatModelAgentMiddleware{
			&approvalMiddleware{},
			&safeToolMiddleware{}, // 将 Tool 错误转换为字符串
		},
		ModelRetryConfig: &adk.ModelRetryConfig{
			MaxRetries: 5,
			IsRetryAble: func(ctx context.Context, err error) bool {
				return strings.Contains(err.Error(), "429") ||
					strings.Contains(err.Error(), "Too Many Requests") ||
					strings.Contains(err.Error(), "qpm limit")
			},
		},
	})

	/*
		Handlers: []adk.ChatModelAgentMiddleware{
		    &middlewareA{},  // 最外层：最先 Wrap，最先拦截请求，但 WrapModel 最后生效
		    &middlewareB{},  // 中间层
		    &middlewareC{},  // 最内层：最后 Wrap
		}
		请求 → A.Wrap → B.Wrap → C.Wrap → 实际 Tool 执行 → C返回 → B返回 → A返回 → 响应
		实用建议： 将 safeToolMiddleware（错误捕获）放在最内层（数组末尾），确保其他 Middleware 抛出的中断错误能正确向外传播。
	*/
	if err != nil {
		log.Printf("创建代理失败: %v", err)
		return
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
		CheckPointStore: store,
	})

	history, err := store.Load(ctx, sessionID)
	if err != nil {
		log.Fatal("加载会话历史失败")
	}
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("开始对话（输入 'exit' 退出）：")

	for {
		fmt.Print("用户：")
		if !scanner.Scan() {
			break
		}
		userInput := scanner.Text()
		if userInput == "" {
			fmt.Println("请输入内容")
			continue
		}
		if userInput == "exit" {
			break
		}

		history = append(history, schema.UserMessage(userInput))
		events := runner.Run(ctx, history)
		var finalAssistantMessage *schema.Message

		for {
			event, ok := events.Next()
			if !ok {
				break // 迭代器关闭，全部事件已消费
			}
			if event == nil {
				continue
			}
			if event.Err != nil {
				log.Printf("运行 Runner 错误: %v", event.Err)
				continue
			}
			if event.Output == nil || event.Output.MessageOutput == nil {
				continue
			}
			if msg, err := event.Output.MessageOutput.GetMessage(); err == nil && msg != nil {
				if event.Output.MessageOutput.Role != schema.Assistant {
					continue
				}
				resp := msg.Content
				fmt.Printf("Agent: %s\n", resp)
				finalAssistantMessage = msg
			}
		}

		if finalAssistantMessage != nil {
			history = append(history, finalAssistantMessage)
			store.Save(ctx, sessionID, history)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("读取用户输入错误: %v", err)
	}
	fmt.Println("结束执行")

}

// SafeToolMiddleware 捕获 Tool 错误并转换为字符串
type safeToolMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
}

// WrapInvokableToolCall 调用时机：该方法在每次工具即将被执行时（请求时）动态调用
func (m *safeToolMiddleware) WrapInvokableToolCall(
	_ context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	_ *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	return func(ctx context.Context, args string, opts ...tool.Option) (string, error) {
		output, err := endpoint(ctx, args, opts...)
		if err != nil {
			fmt.Printf("Tool error: %v\n", err)

			// 忽略中断错误
			if _, ok := compose.IsInterruptRerunError(err); ok {
				return "", err
			}
			return fmt.Sprintf("Tool error: %v", err), nil
		}
		return output, nil
	}, nil
}

type approvalMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
}

// WrapInvokableToolCall 为 adk 中间件的 Tool 调用钩子，用于给名为 "execute" 的工具
// 在其原有调用端点外层装配一次人工审批（Interrupt / Resume）流程。
// 非 "execute" 的工具会原样透传；命中审批的工具在首次调用时发起可持久化中断，
// 在恢复阶段根据用户返回的审批结果决定继续执行、返回拒绝信息，或在缺失审批结果时再次发起审批。
//
// 参数：
//   - _ : 上下文对象，本实现中未使用，恢复所需的上下文由返回闭包内的 ctx 承担
//   - endpoint : 被包装的原始 InvokableToolCall 端点，审批通过后由包装闭包回调它完成真实调用
//   - tCtx : 工具调用上下文，携带工具名称等元信息，用于判定当前调用是否需要走审批
//
// 返回值：
//   - adk.InvokableToolCallEndpoint : 经审批逻辑包装后的调用端点；不需要审批时返回原始 endpoint
//   - error : 端点装配过程中的错误，本实现固定返回 nil
func (m *approvalMiddleware) WrapInvokableToolCall(
	_ context.Context,
	endpoint adk.InvokableToolCallEndpoint,
	tCtx *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	// 只拦截需要审批的 Tool
	if tCtx.Name != "execute" || tCtx.Name != "delete" {
		return endpoint, nil
	}

	// 以闭包形式重新构造 endpoint：将本次入参持久化到中断状态，以便恢复时回放原始调用
	return func(ctx context.Context, args string, opts ...tool.Option) (string, error) {
		// 读取中断状态：wasInterrupted 表示此前是否已发起过审批中断，storedArgs 为中断时保存的原始入参
		wasInterrupted, _, storedArgs := tool.GetInterruptState[string](ctx)

		// 首次调用尚未处于中断态：携带本次入参触发可持久化的审批中断，等待外部给出决策
		if !wasInterrupted {
			return "", tool.StatefulInterrupt(ctx, &commontool.ApprovalInfo{
				ToolName:        tCtx.Name,
				ArgumentsInJSON: args,
			}, args)
		}

		// 进入恢复流程：尝试从恢复上下文中读取本次操作携带的审批结果
		isTarget, hasData, data := tool.GetResumeContext[*commontool.ApprovalResult](ctx)

		// 命中审批结果类型且携带数据：按“通过 / 拒绝”分支处理
		if isTarget && hasData {
			if data.Approved {
				return endpoint(ctx, storedArgs, opts...)
			}
			if data.DisapproveReason != nil {
				return fmt.Sprintf("tool '%s' disapproved: %s", tCtx.Name, *data.DisapproveReason), nil
			}
			return fmt.Sprintf("tool '%s' disapproved", tCtx.Name), nil
		}

		// 未拿到审批结果时，进一步探测是否存在任意类型的恢复上下文，以区分“恢复但目标非本工具”与“完全无恢复数据”
		isTarget, _, _ = tool.GetResumeContext[any](ctx)

		// 当前恢复上下文与本工具无关，视为审批尚未完成，使用中断时保存的参数再次触发审批中断
		if !isTarget {
			return "", tool.StatefulInterrupt(ctx, &commontool.ApprovalInfo{
				ToolName:        tCtx.Name,
				ArgumentsInJSON: storedArgs,
			}, storedArgs)
		}

		// 存在其他形式的恢复数据，视为可以放行，回放中断前保存的参数以执行原始端点
		return endpoint(ctx, storedArgs, opts...)
	}, nil
}

// RedisSessionStore 基于 Redis 实现会话存储
type RedisSessionStore struct {
	client *redis.Client
	ttl    time.Duration // 过期时间，秒
}

func NewRedisSessionStore(addr string, ttl int) *RedisSessionStore {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	return &RedisSessionStore{
		client: rdb,
		ttl:    time.Duration(ttl) * time.Second,
	}
}

// Load 加载会话历史
func (s *RedisSessionStore) Load(ctx context.Context, sessionID string) ([]adk.Message, error) {
	val, err := s.client.Get(ctx, sessionID).Result()
	if err == redis.Nil {
		return []*schema.Message{}, nil // 会话不存在，返回空历史
	}
	if err != nil {
		return nil, err
	}

	var msgs []adk.Message
	if err := json.Unmarshal([]byte(val), &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// Save 保存会话历史
func (s *RedisSessionStore) Save(ctx context.Context, sessionID string, msgs []*schema.Message) error {
	data, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, sessionID, data, 0).Err()
}

func (r *RedisSessionStore) Set(ctx context.Context, key string, value []byte) error {
	return r.client.Set(ctx, "eino:checkpoint:"+key, value, r.ttl).Err()
}

func (r *RedisSessionStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	data, err := r.client.Get(ctx, "eino:checkpoint:"+key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	return data, err == nil, err
}
