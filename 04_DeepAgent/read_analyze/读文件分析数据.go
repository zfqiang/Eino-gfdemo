package readanalyze

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
)

func ReadAnalyze() {
	ctx := context.Background()

	// 1. 创建支持工具调用的 ChatModel
	model := pkg.NewChatModel(ctx)

	// 2. 准备文件系统后端（使用内存后端，适合演示）
	backend, err := local.NewBackend(ctx, &local.Config{})
	if err != nil {
		log.Fatal("创建后端失败")
	}

	projectRoot := pkg.GetProjectRoot()
	filePath := filepath.Join(projectRoot, "resources", "data", "sales.csv")
	filePathForAgent := strings.ReplaceAll(filePath, "\\", "/")

	// 可选：预先写入一个 CSV 文件，让 Agent 有东西可分析
	if err := backend.Write(ctx, &filesystem.WriteRequest{
		FilePath: filePath,
		Content:  "product,revenue\nA,100\nB,200\nC,150\n",
	}); err != nil {
		log.Fatalf("写入 CSV 失败: %v", err)
	}

	// 3. 创建 DeepAgent
	agent, err := deep.New(ctx, &deep.Config{
		Name:        "分析数据",
		Description: "使用 DeepAgent 分析数据",
		Instruction: fmt.Sprintf("使用 DeepAgent 分析数据。只允许使用 read_file 或其他只读工具读取文件：%s。不要调用 write_file、edit_file 或任何修改文件的工具。严格使用正斜杠 / 作为路径分隔符，不要使用反斜杠。", filePathForAgent),
		ChatModel:   model,
		Backend:     backend,
		// Shell 和 StreamingShell 按需配置，本 Demo 不涉及
	})

	if err != nil {
		log.Fatal(err)
	}

	// 4. 使用 Runner 执行，而非直接调用 agent.Run
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := runner.Query(ctx, fmt.Sprintf("分析 %s 文件，生成一份摘要报告，指出营收最高的产品", filePathForAgent))

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Printf("Agent 执行错误: %v", event.Err)
			continue
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, _ := event.Output.MessageOutput.GetMessage()
			fmt.Printf("[%s] %s\n", event.AgentName, msg.Content)
		}
	}
}
