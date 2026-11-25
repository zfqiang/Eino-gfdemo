package main

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	// 1. 创建 AgenticModel 实例
	am := pkg.NewAgenticModel(ctx)

	// 2. 构造 AgenticMessage（使用 ContentBlock）
	input := []*schema.AgenticMessage{
		{
			Role: "user",
			ContentBlocks: []*schema.ContentBlock{
				{
					Type:          "user_input_text",
					UserInputText: &schema.UserInputText{Text: "请写一个关于机器学习的文章"},
				},
			},
		},
	}

	// 3. 在请求时通过选项传入工具（可选）
	toolInfo := []*schema.ToolInfo{
		{
			Name: "search",
			Desc: "Search for information",
		},
	}
	resp, err := am.Generate(ctx, input, model.WithTools(toolInfo))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Response Blocks: %+v\n", resp.ContentBlocks)
}
