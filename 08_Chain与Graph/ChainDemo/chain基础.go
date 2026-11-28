package chaindemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func ChainDemo() {
	ctx := context.Background()

	tpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("你是一个{role}"),
		schema.UserMessage("{input}"),
	)

	cm := pkg.GetBaiLianLLM(ctx)

	// 创建一个链 输入是包含模板变量的 map，输出是模型生成的字符串
	chain := compose.NewChain[map[string]any, string]()

	postProcess := compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (string, error) {
		return fmt.Sprintf("[%s] \n %s", "学习教练", input.Content), nil
	})

	//创建chain 串联节点：模板 -> 模型 -> 提取内容
	chain.AppendChatTemplate(tpl). // 接收 map，用于填充 {question} 等变量
					AppendChatModel(cm).      // 接收填充后的消息，调用大模型
					AppendLambda(postProcess) // 将消息对象转为纯字符串输出

	// 编译链
	runner, err := chain.Compile(ctx)
	if err != nil {
		fmt.Println("创建链失败:", err)
	}

	variables := map[string]interface{}{
		"role":  "友好的 AI 助手",
		"input": "你好，请介绍一下 Eino 框架",
	}

	// 执行链
	output, err := runner.Invoke(ctx, variables)
	if err != nil {
		fmt.Println("运行链失败:", err)
	}

	fmt.Printf("AI 回复：%s", output)

}
