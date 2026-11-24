package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

var ctx context.Context
var model *openai.ChatModel
var err error

// // 创建 reduction middleware：管理工具输出长度
// var reductionMW, _ = reduction.New(ctx, &reduction.Config{
// 	Backend:           filesystemBackend, // 存储后端
// 	MaxLengthForTrunc: 50000,             // 单次工具输出最大长度
// 	MaxTokensForClear: 30000,             // 触发清理的 token 阈值
// })

// // 创建 summarization middleware：自动压缩对话历史
// var summarizationMW, _ = summarization.New(ctx, &summarization.Config{
// 	Model: chatModel, // 用于生成摘要的模型
// 	Trigger: &summarization.TriggerCondition{
// 		ContextTokens: 190000, // 触发摘要的 token 阈值
// 	},
// })

// // 组合多个 middleware（概念示例，使用 DeepAgent 时将 adk.NewChatModelAgent 替换为 deep.New）
// var agent, _ = adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
// 	Handlers: []adk.ChatModelAgentMiddleware{ // 注意：配置字段名为 Handlers，概念上与 Middlewares 等价
// 		summarizationMW, // 最外层：对话历史摘要
// 		reductionMW,     // 中间层：工具输出缩减
// 	},
// })

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
	backend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		log.Printf("创建后端失败: %v", err)
		return
	}

	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectRoot = cwd
		}
	}
	if abs, err := filepath.Abs(projectRoot); err == nil {
		projectRoot = abs
	}

	defaultInstruction := fmt.Sprintf(`你是一个乐于助人的助手，帮助用户学习Eino框架。

重要提示：在使用文件系统工具（如ls、read_file、glob、grep等）时，必须使用绝对路径。

项目根目录为：%s

- 当用户要求列出“当前目录”中的文件时，使用路径：%s - 当用户要求读取带有相对路径的文件时，通过在前面添加%s将其转换为绝对路径 - 示例：如果用户说“读取main.go”，则应使用文件路径“%s/main.go”调用read_file函数

在调用文件系统工具时，请始终使用绝对路径。`, projectRoot, projectRoot, projectRoot, projectRoot)

	agent, err := deep.New(ctx, &deep.Config{
		Name:           "Ch04ToolAgent",
		Description:    "通过LocalBackend访问文件系统的ChatWithDoc代理。",
		ChatModel:      model,
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
