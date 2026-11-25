package chatmodeldemo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"eino-dfdemo/pkg"

	"github.com/cloudwego/eino/schema"
)

func SimpleStream() {
	log.Println("-------------开始简单对话流式输出-------------")
	ctx := context.Background()

	cm := pkg.GetBaiLianLLM(ctx)
	messages := []*schema.Message{
		schema.SystemMessage("你是一个AI助手"),
		schema.UserMessage("请用一句话介绍一下你自己"),
	}

	stream, err := cm.Stream(ctx, messages)
	if err != nil {
		log.Printf("流式调用模型失败: %v", err)
		return
	}
	defer stream.Close()

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Printf("流式接收失败: %v", err)
			return
		}
		fmt.Print(chunk.Content)
	}
	fmt.Println()
}
