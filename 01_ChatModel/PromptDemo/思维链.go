package promptdemo

import (
	"context"
	"eino-dfdemo/pkg"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

func PromptChain() {
	ctx := context.Background()

	systemMessage := `你是一个逻辑推理专家。
请按照以下步骤解决问题：
1. 理解问题：复述问题的要求
2. 分析：列出解决问题需要的步骤
3. 计算：逐步执行计算
4. 验证：检测答案是否合理
5. 结论：给出最终答案`
	template := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(systemMessage),
		schema.UserMessage("{question}"),
	)

	variables := map[string]interface{}{
		"question": "10+2+52-34=?",
	}
	messages, err := template.Format(ctx, variables)
	if err != nil {
		panic(err)
	}

	chatModel := pkg.GetBaiLianLLM(ctx)

	// 流式输出
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		fmt.Print(chunk.Content)
	}
	fmt.Println()

}
