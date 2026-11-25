package promptdemo

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

func PromptBase() {
	ctx := context.Background()
	promptTpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("你是一个{role},请根据用户的输入，生成一个专业的回答"),
		schema.UserMessage("{input}"),
	)

	variables := map[string]any{
		"role":  "房产经纪人",
		"input": "我要找一个500万左右房子",
	}

	messages, err := promptTpl.Format(ctx, variables)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(messages)
}
