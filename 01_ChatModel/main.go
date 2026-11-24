package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

func main() {
	chatModelDemo1()
}

// 简单对话直接输出和流式输出
func chatModelDemo1() {

	ctx := context.Background()

	// 创建模型
	model := getBaiLianLLM(ctx)

	//创建对话消息
	messages := []*schema.Message{
		{
			Role:    "system",
			Content: "你是一个AI智能助手",
		},
		{
			Role:    "user",
			Content: "请介绍一下你自己",
		},
	}

	//调用模型
	// result, err := model.Generate(ctx, messages)
	// if err != nil {
	// 	log.Fatal("普通调用模型失败")
	// }
	// log.Println(result.Content)

	//流水调用
	stream, err := model.Stream(ctx, messages)
	if err != nil {
		log.Fatal("流式调用模型失败")
	}
	defer stream.Close()

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatalf("流式接收失败: %v", err)
		}
		fmt.Print(chunk.Content)
	}

}

func chatModelToolDemo1() {
	ctx := context.Background()
	model := getBaiLianLLM(ctx)

	err := model.BindTools([]*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "根据用户的姓名和邮箱，查询用户的公司和职位信息",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "用户的姓名",
					},
					"email": {
						Type: "string",
						Desc: "用户的邮箱",
					},
				}),
		},
		{
			Name: "user_salary",
			Desc: "根据用户的姓名和邮箱，查询用户的薪酬信息",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "用户的姓名",
					},
					"email": {
						Type: "string",
						Desc: "用户的邮箱",
					},
				}),
		},
	})

	if err != nil {
		log.Fatal("绑定工具失败")
	}

	messages := []*schema.Message{
		schema.SystemMessage("你是一名房产经纪人，结合用户的薪酬和工作，使用 user_company、user_salary 两个 API，为其提供相关的房产信息。邮箱是必须的"),
		schema.UserMessage("我的姓名是 zhangsan，我的邮箱是 zhangsan@bytedance.com，请帮我推荐一些适合我的房子。"),
	}

	resp, err := model.Generate(ctx, messages)
	if err != nil {
		log.Fatal("调用模型失败")
	}
	log.Println(resp)

	/*
		2026/11/24 10:17:36 assistant:
		tool_calls:
		index[0]:{Index:0x126a3c3683f0 ID:call_6d0be6750ed946808c7a19 Type:function Function:{Name:user_company Arguments:{"email": "zhangsan@bytedance.com", "name": "zhangsan"}} Extra:map[]}
		index[1]:{Index:0x126a3c368410 ID:call_9193d37abe5a4dcc96cb30 Type:function Function:{Name:user_salary Arguments:{"email": "zhangsan@bytedance.com", "name": "zhangsan"}} Extra:map[]}

		finish_reason: tool_calls
		usage: &{385 {0} 64 449 {0}}
	*/

}

func getBaiLianLLM(ctx context.Context) *openai.ChatModel {
	if err := godotenv.Load(); err != nil {
		log.Fatal("加载 .env 文件失败")
	}

	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
		Model:   os.Getenv("DASHSCOPE_MODE"),
		BaseURL: os.Getenv("DASHSCOPE_BASE_URL"),

		Timeout: 30 * time.Second,

		// // 生成参数
		Temperature: new(float32(0.7)), // 控制输出随机性，范围 [0.0, 2.0]，越高越随机
		TopP:        new(float32(0.7)), // 核采样参数，范围 [0.0, 1.0]，越低越聚焦
		MaxTokens:   new(int(500)),     // 限制最大生成 token 数量，范围 [1, 8192]

		// // 停止序列 - 遇到这些文本时停止生成
		// Stop: []string{"\\n\\n", "总结:"},

		// // 惩罚参数 - 控制重复度
		// PresencePenalty:  new(float32(0.6)), // 存在惩罚，范围 [-2.0, 2.0]，正值增加新话题可能性
		// FrequencyPenalty: new(float32(0.5)), // 频率惩罚，范围 [-2.0, 2.0]，正值减少重复词语
	})
	if err != nil {
		log.Fatal("阿里云百炼  创建 ChatModel 失败")
	}
	return model
}
