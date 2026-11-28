package graphdemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func GraphDemo() {
	ctx := context.Background()
	cm := pkg.GetBaiLianLLM(ctx)

	// 定义三套角色提示词模板：分别扮演 Go 语法老师、Agent 老师和 Go 代码诊断老师，
	// 均要求回答适合初学者、先结论再解释，并通过 {question} 占位符注入用户问题
	goTpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("扮演Go语法老师，回答适合初学者，先结论再解释"),
		schema.UserMessage("{question}"),
	)

	agentTpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("扮演Agent老师，回答适合初学者，先结论再解释"),
		schema.UserMessage("{question}"),
	)

	diagTpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("扮演Go代码诊断老师，回答适合初学者，先结论再解释"),
		schema.UserMessage("{question}"),
	)

	// 分类节点：以 Lambda 形式实现意图识别。将问题转为小写后做关键词匹配，
	// 命中 agent/tool/enio/rag 归为 agent 类，命中 panic/error 归为 diag 类，否则默认 go 类，
	// 最终把判定结果写入 input["category"]，供后续分支路由读取
	classifier := compose.InvokableLambda(func(ctx context.Context, input map[string]any) (output map[string]any, err error) {
		q := strings.ToLower(input["question"].(string))
		cat := "model"

		// 正式环境，请求大模型，返回结构化数据，让大模型判断是哪种类型
		switch {
		case strings.Contains(q, "agent") || strings.Contains(q, "tool") ||
			strings.Contains(q, "enio") || strings.Contains(q, "rag"):
			cat = "agent"
		case strings.Contains(q, "panic") || strings.Contains(q, "error"):
			cat = "diag"
		case strings.Contains(q, "go"):
			cat = "go"

		}

		input["category"] = cat
		return input, nil
	})

	classifierName := "classifier"
	goTplName := "goTpl"
	agentTplName := "agentTpl"
	diagTplName := "diagTpl"
	cmName := "cmModel"

	// 创建 Graph 编排器：输入为 map[string]any（携带 question 等变量），输出为 *schema.Message（模型回答）
	g := compose.NewGraph[map[string]any, *schema.Message]()

	// 添加节点：按顺序添加节点，每个节点都携带一个名称，供后续节点引用
	_ = g.AddLambdaNode(classifierName, classifier)
	_ = g.AddChatTemplateNode(goTplName, goTpl)
	_ = g.AddChatTemplateNode(agentTplName, agentTpl)
	_ = g.AddChatTemplateNode(diagTplName, diagTpl)
	_ = g.AddChatModelNode(cmName, cm)

	// 添加边：指定节点名称，以及输入输出字段映射关系

	// 图的起点直接连接到分类节点
	_ = g.AddEdge(compose.START, classifierName)

	// 在分类节点后挂载分支路由：根据 input["category"] 拼出目标模板节点名（如 go_tpl），
	// 末尾的 map 声明该分支所有可能的目标节点，实现按类别动态选路
	_ = g.AddBranch(classifierName, compose.NewGraphBranch(
		func(ctx context.Context, input map[string]any) (endNode string, err error) {
			cat := input["category"].(string)
			fmt.Println("category:", cat)
			return fmt.Sprintf("%s_tpl", cat), nil
		},
		map[string]bool{
			goTplName:    true,
			agentTplName: true,
			diagTplName:  true,
		},
	))

	// 每个模板节点都连接到模型节点，实现模板注入
	_ = g.AddEdge(goTplName, cmName)
	_ = g.AddEdge(agentTplName, cmName)
	_ = g.AddEdge(diagTplName, cmName)

	// 模型节点连接到图终点
	_ = g.AddEdge(cmName, compose.END)

	// 编译图，生成可执行的 runner；编译失败直接 panic
	runner, err := g.Compile(ctx)
	if err != nil {
		panic(err)
	}

	// 准备测试问题：第一条应归为 go 类，第二条应命中关键词归为 agent 类，用于验证路由是否正确
	questions := []string{
		"Go 的interface 详解, 20字以内",
		"Eino 的ToolNode 详解, 20字以内",
		"你是谁呀, 20字以内",
	}

	// 逐个调用 runner：把问题作为模板变量传入，触发“分类 -> 路由 -> 渲染模板 -> 模型作答”的完整链路
	for _, v := range questions {
		variable := map[string]any{"question": v}

		answer, err := runner.Invoke(ctx, variable)
		if err != nil {
			panic(err)
		}
		fmt.Printf("问题是：%s , 结果是： %s \n", v, answer.Content)
	}
}
