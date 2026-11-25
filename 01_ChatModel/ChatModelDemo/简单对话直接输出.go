package chatmodeldemo

import (
	"context"
	"eino-dfdemo/pkg"
	"log"

	"github.com/cloudwego/eino/schema"
)

func SimpleGenerate() {
	log.Println("-------------开始简单对话-------------")
	ctx := context.Background()

	// 创建模型
	cm := pkg.GetBaiLianLLM(ctx)

	// 创建会话
	messages := []*schema.Message{
		schema.SystemMessage("你是一个AI助手"),
		schema.UserMessage("请用一句话介绍一下你自己"),
	}

	// 调用模型
	result, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Fatal("普通调用模型失败")
	}

	log.Println("模型输出：", result.Content)
}
