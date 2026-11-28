package workflowdemo

import (
	"context"
	"log"
	"strconv"

	"github.com/cloudwego/eino/compose"
)

func WorkflowDemo1() {

	// 创建工作流，方式与创建图（Graph）类似
	wf := compose.NewWorkflow[int, string]()

	// 向工作流添加 Lambda 节点，方式与向图添加 Lambda 节点类似
	wf.AddLambdaNode("lambda", compose.InvokableLambda(
		func(ctx context.Context, in int) (string, error) {
			log.Printf("lambda input = %d", in)
			return strconv.Itoa(in), nil
		})).
		// 为该 Lambda 节点添加来自 START 的输入。
		// 这表示将 START 的全部输出映射到 Lambda 的输入。
		// AddInput 会同时设置控制依赖和数据依赖。
		AddInput(compose.START)

	// 获取工作流的 compose.END，以便进行方法链式调用
	wf.End().
		// 为 compose.END 添加输入，
		// 表示“将 Lambda 节点的全部输出作为 END 的输出”。
		AddInput("lambda")

	// 编译工作流，方式与编译图类似
	run, err := wf.Compile(context.Background())
	if err != nil {
		log.Fatalf("workflow compile error: %v", err)
		return
	}

	// 调用工作流，方式与调用图类似
	result, err := run.Invoke(context.Background(), 100)
	if err != nil {
		log.Fatalf("workflow run err: %v", err)
		return
	}

	log.Printf("%v", result)
}
