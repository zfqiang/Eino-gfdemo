package workflowdemo

import (
	"context"
	"log"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func WorkflowDemo2() {

	type counter struct {
		FullStr string // 因为需要对该字段进行字段映射，所以导出该字段
		SubStr  string // 因为需要对该字段进行字段映射，所以导出该字段
	}

	// wordCounter 是一个 Lambda 函数，用于统计 FullStr 中 SubStr 的出现次数
	wordCounter := func(ctx context.Context, c counter) (int, error) {
		return strings.Count(c.FullStr, c.SubStr), nil
	}

	type message struct {
		*schema.Message        // 因为需要对该字段进行字段映射，所以导出该字段
		SubStr          string // 因为需要对该字段进行字段映射，所以导出该字段
	}

	// 创建工作流，方式与创建图（Graph）类似
	wf := compose.NewWorkflow[message, map[string]any]()

	// 添加 Lambda 节点 c1，方式与在图中添加节点类似
	wf.AddLambdaNode("c1", compose.InvokableLambda(wordCounter)).
		AddInput(compose.START, // 添加来自 START 的输入，并指定两个字段映射
			// 将 START 的 SubStr 字段映射到 Lambda 节点 c1 的 SubStr 字段
			compose.MapFields("SubStr", "SubStr"),
			// 将 START 的 Message.Content 字段映射到 Lambda 节点 c1 的 FullStr 字段
			compose.MapFieldPaths([]string{"Message", "Content"}, []string{"FullStr"}))

	// 添加 Lambda 节点 c2，方式与在图中添加节点类似
	wf.AddLambdaNode("c2", compose.InvokableLambda(wordCounter)).
		AddInput(compose.START, // 添加来自 START 的输入，并指定两个字段映射
			// 将 START 的 SubStr 字段映射到 Lambda 节点 c1 的 SubStr 字段
			compose.MapFields("SubStr", "SubStr"),
			// 将 START 的 Message.ReasoningContent 字段映射到 Lambda 节点 c1 的 FullStr 字段
			compose.MapFieldPaths([]string{"Message", "ReasoningContent"}, []string{"FullStr"}))

	wf.End(). // 获取 compose.END，以便进行方法链式调用
		// 添加来自 c1 的输入，
		// 将 c1 的完整输出映射到映射键“content_count”
		AddInput("c1", compose.ToField("content_count")).
		// 同时添加来自 c2 的输入，
		// 将 c2 的完整输出映射到映射键“reasoning_content_count”
		AddInput("c2", compose.ToField("reasoning_content_count"))

	// 编译工作流，方式与编译图类似
	run, err := wf.Compile(context.Background())
	if err != nil {
		log.Fatalf("workflow compile error: %v", err)
		return
	}

	// 调用工作流，方式与调用图类似
	result, err := run.Invoke(context.Background(), message{
		Message: &schema.Message{
			Role:             schema.Assistant,
			Content:          "Hello world!",
			ReasoningContent: "I need to say something meaningful ooooo",
		},
		SubStr: "o", // 统计字符“o”的出现次数
	})
	if err != nil {
		log.Fatalf("workflow run err: %v", err)
		return
	}

	log.Printf("%v", result)
}
