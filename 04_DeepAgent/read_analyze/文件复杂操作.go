package readanalyze

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
)

func FileComplexOperation() {
	ctx := context.Background()

	// 1. 创建 ChatModel（需支持工具调用）
	cm := pkg.GetBaiLianLLM(ctx)

	// 2. 创建本地文件系统 Backend
	// 本地 Backend 强制使用绝对路径，并实现了 StreamingShell（可执行 shell 命令）
	workspace, err := filepath.Abs("./deepagent_workspace")
	if err != nil {
		log.Fatalf("resolve workspace: %v", err)
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		log.Fatalf("create workspace: %v", err)
	}

	backend, err := local.NewBackend(ctx, &local.Config{
		// 可选：限制允许执行的命令，生产环境建议配置
		ValidateCommand: func(cmd string) error {
			// 允许的命令白名单（首词匹配）
			allowed := map[string]bool{
				"ls":    true,
				"cat":   true,
				"grep":  true,
				"wc":    true, // 用于统计行数
				"echo":  true,
				"mkdir": true,
			}
			// 解析命令首词
			parts := strings.Fields(cmd)
			if len(parts) == 0 {
				return fmt.Errorf("empty command")
			}

			if !allowed[parts[0]] {
				return fmt.Errorf("command not allowed: %s", parts[0])
			}
			return nil
		},
		// 多模态读取配置，可选配置
		/* MultiModalRead: local.MultiModalReadConfig{
			MaxImageSizeMB:        20,  // 图片上限从默认 10MB 提到 20MB
			MaxPDFSizeMB:          50,  // 整文 PDF 上限从 20MB 提到 50MB
			MaxPagedPDFSizeMB:     200, // 分页 PDF 上限从 100MB 提到 200MB
			MaxPDFPagesPerRequest: 10,  // 单次最多读 10 页，比默认 20 更保守
			PDFRenderDPI:          200, // 渲染精度从 150 提到 200，图片更清晰但体积更大

			// PDFium worker pool 调优
			PDFiumPool: local.PDFiumPoolConfig{
				MinIdle:  1, // 至少保持 1 个空闲 worker
				MaxIdle:  2, // 最多保持 2 个空闲 worker
				MaxTotal: 4, // 最大 worker 数，默认是 max(2, NumCPU)
			},

			PDFiumAcquireTimeout: 60 * time.Second, // 等待 worker 的上限，默认 30s
		}, */
	})
	if err != nil {
		log.Fatalf("create local backend: %v", err)
	}

	// 3. 创建 DeepAgent，挂载 Backend 与 StreamingShell
	// Backend 注册 ls/read_file/write_file/edit_file/glob/grep
	// StreamingShell 注册 execute（流式 shell 命令执行）
	agent, err := deep.New(ctx, &deep.Config{
		Name:        "dev_assistant",
		Description: "一个可以规划任务、操作文件、执行命令的开发助手",
		ChatModel:   cm,
		Instruction: fmt.Sprintf(
			"你是一个开发助手。当前工作目录是 %s。执行文件操作时使用绝对路径。先规划步骤，再逐步执行并验证结果。",
			workspace,
		),
		Backend:        backend, // 文件系统工具
		StreamingShell: backend, // Shell 执行工具（本地 Backend 只实现流式版本）
		MaxIteration:   30,
	})
	if err != nil {
		log.Fatalf("create deep agent: %v", err)
	}

	// 4. 运行 Agent
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})

	query := fmt.Sprintf(
		"请在 %s 目录下完成以下任务：\n"+
			"1. 创建一个 data 子目录\n"+
			"2. 在 data 目录下生成一个 numbers.txt，内容为 1 到 10 每行一个数字\n"+
			"3. 用 shell 命令统计 numbers.txt 的行数，并把结果写入 data/count.txt\n"+
			"4. 读取 count.txt 告诉我最终结果",
		workspace,
	)

	iter := runner.Query(ctx, query)

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Printf("event error: %v", event.Err)
			continue
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				continue
			}
			// 打印 Agent 名称与消息内容，便于观察规划与执行过程
			fmt.Printf("[%s] %s\n", event.AgentName, msg.Content)
		}
	}
}
