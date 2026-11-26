package middlewaredemo

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// 当工具库很大时，只让模型通过 tool_search 元工具按需搜索和加载工具，避免把所有工具定义塞入上下文
func DynamicToolSearch() (adk.ChatModelAgentMiddleware, error) {
	ctx := context.Background()

	// 实例代码，实际使用时请根据业务场景自行实现
	tool1, _ := utils.InferTool(
		"get_weather",
		"获取指定城市的天气信息。",
		getWeather,
	)
	tool2, _ := utils.InferTool(
		"get_weather_v2",
		"获取指定城市的天气信息。",
		getWeatherV2,
	)
	tool3, _ := utils.InferTool(
		"get_weather_v3",
		"获取指定城市的天气信息。",
		getWeatherV3,
	)

	toolSearchMW, err := toolsearch.New(ctx, &toolsearch.Config{
		// 必填：可被动态搜索和加载的工具列表
		DynamicTools: []tool.BaseTool{tool1, tool2, tool3 /* ... 大量工具 */},

		// 是否使用模型原生的工具搜索能力
		// false（默认）：中间件自行管理工具可见性，通过 tool_search 结果过滤 ToolInfos
		// true：委托给模型原生能力（如 Claude 的 tool search），或通过 tool_search 代理检索
		UseModelToolSearch: false,
	})
	return toolSearchMW, err
}

// 1. 定义工具的参数结构
type WeatherParams struct {
	City string `json:"city" jsonschema:"required" jsonschema_description:"城市名称"`
}

// 2. 工具的处理函数
func getWeather(ctx context.Context, params WeatherParams) (string, error) {
	// 实际场景中这里调用天气 API
	return fmt.Sprintf("%s：晴，25°C", params.City), nil
}
func getWeatherV2(ctx context.Context, params WeatherParams) (string, error) {
	// 实际场景中这里调用天气 API
	return fmt.Sprintf("%s：晴，25°C", params.City), nil
}
func getWeatherV3(ctx context.Context, params WeatherParams) (string, error) {
	// 实际场景中这里调用天气 API
	return fmt.Sprintf("%s：晴，25°C", params.City), nil
}
