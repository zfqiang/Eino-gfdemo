package chatmodelagentdemo

import (
	"context"
	"eino-dfdemo/pkg"
	"fmt"
	"log"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
)

// 1. 定义工具的参数结构
type WeatherParams struct {
	City string `json:"city" jsonschema:"required" jsonschema_description:"城市名称"`
}

// 2. 工具的处理函数
func getWeather(ctx context.Context, params WeatherParams) (string, error) {
	// 实际场景中这里调用天气 API
	return fmt.Sprintf("%s：晴，25°C", params.City), nil
}

func AgentWeather() {
	ctx := context.Background()

	// 3. 创建 Tool（使用 InferTool，自动从 struct tag 推导 Schema）
	weatherTool, err := utils.InferTool(
		"get_weather",
		"获取指定城市的天气信息。",
		getWeather,
	)
	if err != nil {
		log.Fatalln("创建工具失败：", err)
	}

	// 4. 创建 ChatModel（必须实现 ToolCallingChatModel 接口）
	cm := pkg.GetBaiLianLLM(ctx)

	// 5. 创建 ChatModelAgent，把 Model 和 Tool 组装起来
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "weather_agent",
		Description: "一个可以查询天气的助手",
		Instruction: "你是一个天气助手。用户询问天气时，使用 get_weather 工具查询并回复。",
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{weatherTool},
			},
		},
	})
	if err != nil {
		log.Fatalf("创建 ChatModelAgent 失败: %v", err)
	}

	// 6. 运行 Agent
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})

	iter := runner.Query(ctx, "厦门今天天气怎么样？")

	for {
		event, ok := iter.Next()
		if !ok {
			log.Fatal(err)
		}
		if event.Err != nil {
			log.Fatal(event.Err)
		}
		msg, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Agent 响应: %s\n", msg.Content)
	}

}
