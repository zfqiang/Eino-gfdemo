package main

import (
	"bufio"
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	clc "github.com/cloudwego/eino-ext/callbacks/cozeloop"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go"
)

/*
Eino 的 Callback 机制核心定位是旁路观测钩子：它在 Agent、ChatModel、Tool 等组件生命周期的固定点位触发，抽取输入、输出、错误、Token 消耗等信息，而不干扰主业务流程。

核心应用场景
	全链路追踪与可观测性
		这是最主要的场景。Agent 在运行时像一个“黑盒”，你只知道最终输出，却不清楚中间调用了多少次模型、每个 Tool 执行了多久、Token 消耗了多少。Callback 可以在 OnStart/OnEnd/OnError 等时机收集这些数据，接入 CozeLoop 等平台后，就能看到完整的调用链路和耗时分布。典型输出类似：[trace] chat_model_generate: model=gpt-4.1-mini tokens=150、[trace] tool_call: name=list_files duration=23ms。

	性能与成本分析
		通过 Callback 收集的 Token 消耗数据和模型调用延迟，可以识别哪些对话消耗最高、哪个 Tool 执行最慢，从而优化 Prompt 或调整模型选择。全局 Handler 尤其适合这种业务无关的统一指标收集，通过 callbacks.AppendGlobalHandlers 在服务初始化时注入一次即可。

	调试与问题定位
		当 Agent 输出不符合预期时，Callback 能帮你判断问题出在模型调用、工具执行还是中间件处理环节。例如，在 OnError 中记录完整的错误上下文，或在 OnEnd 中打印模型返回的 ToolCalls 内容，快速定位是 Prompt 问题还是工具参数问题。

	Agent 级别的生命周期观测（v0.8.0+）
		ADK 在 v0.8.0 引入了 Agent 级别的 Callback 支持，使用 adk.WithCallbacks 在 agent.Run 时注入 Handler。AgentCallbackOutput 中包含一个 AsyncIterator[*AgentEvent] 的事件流副本，Handler 可以异步消费它来实时观测 Agent 的每一步事件，而不会阻塞 Agent 执行。官方特别提醒：每个 Handler 拿到的是独立的事件流副本，必须异步消费，否则会阻塞 Agent 运行。


使用方式速览
	全局注入：callbacks.AppendGlobalHandlers(handler)，对所有后续运行生效，适合 tracing 和 logging。
	Graph 运行时注入：compose.WithCallbacks(...)，可通过 DesignateNode 或 DesignateNodeByPath 精确指定生效的节点。
	Agent 级别注入：agent.Run(ctx, input, adk.WithCallbacks(handler))。
	Graph 外单独使用组件：需要手动用 callbacks.InitCallbacks 注入初始 RunInfo 和 Handlers，组件内部嵌套调用时用 ReuseHandlers 传递上下文，否则 Callback 无法正确触发。

*/

func main() {
	Demo()
}

func Demo() {
	fmt.Println("开始执行")
	ctx := context.Background()
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

	cm := pkg.GetBaiLianLLM(ctx)
	agent, err := deep.New(ctx, &deep.Config{
		Name:           "Ch04ToolAgent",
		Description:    "ChatWithDoc agent with CozeLoop tracing.",
		ChatModel:      cm,
		Instruction:    defaultInstruction,
		Backend:        backend, // 提供文件系统操作能力
		StreamingShell: backend, // 提供命令执行能力
		MaxIteration:   10,      // 最大迭代次数
		Handlers: []adk.ChatModelAgentMiddleware{
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
	})

	history := make([]adk.Message, 0, 10)
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
