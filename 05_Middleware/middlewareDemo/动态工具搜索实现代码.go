package middlewaredemo

import (
	"bytes"
	"context"
	"eino-dfdemo/pkg"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

/*
==============================

# 动态工具搜索应用场景
    toolsearch 中间件解决的是 Agent 工具库过大导致上下文膨胀的问题。当你有几十上百个工具时，把所有工具的 Schema 一次性塞给模型，不仅浪费 token，还可能超出上下文限制，甚至干扰模型判断。toolsearch 的做法是：初始只暴露一个 tool_search 元工具，模型通过搜索找到相关工具后，这些工具才会出现在后续调用的可见列表中。

    应用场景
    这个中间件在以下情况尤其有用：
        1、工具数量庞大：例如一个通用的企业助手，接入了 CRM、ERP、工单系统、文档库等几十个 API，每个 API 对应一个工具。
        2、工具使用频率不均：大部分工具在单次对话中根本用不到，但每次请求都要携带它们的完整定义，成本很高。
        3、按需发现：模型需要根据用户当前的问题，动态判断该用哪些工具，而不是一开始就面对一个庞大的工具列表。

==============================
*/

// ==================== 输入参数结构体（保持不变） ====================

type CRMSearchInput struct {
	Query string `json:"query" jsonschema:"required,description=客户搜索关键词"`
}

type ERPInventoryInput struct {
	SKU string `json:"sku" jsonschema:"required,description=商品SKU编码"`
}

type TicketCreateInput struct {
	Title    string `json:"title" jsonschema:"required,description=工单标题"`
	Priority string `json:"priority" jsonschema:"description=优先级: low/medium/high"`
}

type DocSearchInput struct {
	Keyword string `json:"keyword" jsonschema:"required,description=文档搜索关键词"`
}

// ==================== 真实 HTTP 调用实现 ====================

// 全局 HTTP 客户端，带超时控制
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// searchCRM 真实调用 CRM 系统的搜索 API
func searchCRM(ctx context.Context, input *CRMSearchInput) (string, error) {
	// 1. 构造请求 URL（替换为你的实际 CRM API 地址）
	apiURL := fmt.Sprintf("https://your-crm-api.example.com/api/v1/customers/search?q=%s", input.Query)

	// 2. 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建 CRM 请求失败: %w", err)
	}

	// 3. 设置认证头（替换为你的实际认证方式）
	req.Header.Set("Authorization", "Bearer YOUR_CRM_TOKEN")
	req.Header.Set("Content-Type", "application/json")

	// 4. 发起请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 CRM API 失败: %w", err)
	}
	defer resp.Body.Close()

	// 5. 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 CRM 响应失败: %w", err)
	}

	// 6. 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CRM API 返回错误状态 %d: %s", resp.StatusCode, string(body))
	}

	// 7. 解析并返回（这里假设 API 返回 JSON，直接透传或重新序列化）
	// 如果需要提取特定字段，可以反序列化到结构体后再处理
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析 CRM 响应失败: %w", err)
	}

	// 将结果序列化为字符串返回给 Agent
	output, _ := json.Marshal(result)
	return string(output), nil
}

// queryERPInventory 真实调用 ERP 系统的库存查询 API（POST 示例）
func queryERPInventory(ctx context.Context, input *ERPInventoryInput) (string, error) {
	// 1. 构造请求体
	reqBody := map[string]string{"sku": input.SKU}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化 ERP 请求体失败: %w", err)
	}

	// 2. 创建 POST 请求（替换为你的实际 ERP API 地址）
	apiURL := "https://your-erp-api.example.com/api/v1/inventory/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建 ERP 请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer YOUR_ERP_TOKEN")
	req.Header.Set("Content-Type", "application/json")

	// 3. 发起请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 ERP API 失败: %w", err)
	}
	defer resp.Body.Close()

	// 4. 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 ERP 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ERP API 返回错误状态 %d: %s", resp.StatusCode, string(body))
	}

	// 5. 返回结果
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析 ERP 响应失败: %w", err)
	}

	output, _ := json.Marshal(result)
	return string(output), nil
}

// createTicket 真实调用工单系统 API（可参照上述模式自行实现）
func createTicket(ctx context.Context, input *TicketCreateInput) (string, error) {
	// 逻辑与 queryERPInventory 类似，构造 POST 请求发送到工单系统
	// 此处省略具体实现，保持你之前模拟逻辑或替换为真实调用
	return `{"ticket_id":"TK-001","status":"created"}`, nil
}

// searchDocs 真实调用文档库搜索 API（可参照上述模式自行实现）
func searchDocs(ctx context.Context, input *DocSearchInput) (string, error) {
	// 逻辑与 searchCRM 类似，构造 GET 请求发送到文档库
	// 此处省略具体实现
	return `{"docs":["文档1","文档2"]}`, nil
}

func MainToolSearch() {
	ctx := context.Background()

	// 工具创建部分保持不变（InferTool 会自动处理 JSON 反序列化）
	crmTool, err := utils.InferTool("crm_search_customer", "在CRM系统中搜索客户信息，支持按姓名、公司名、手机号搜索", searchCRM)
	if err != nil {
		log.Fatalf("创建 CRM 工具失败: %v", err)
	}

	erpTool, err := utils.InferTool("erp_query_inventory", "查询ERP系统中的商品库存数量和仓库位置", queryERPInventory)
	if err != nil {
		log.Fatalf("创建 ERP 工具失败: %v", err)
	}

	ticketTool, err := utils.InferTool("ticket_create", "在工单系统中创建一个新的工单，需要标题和优先级", createTicket)
	if err != nil {
		log.Fatalf("创建工单工具失败: %v", err)
	}

	docTool, err := utils.InferTool("doc_search", "在文档库/知识库中搜索相关文档，返回文档标题列表", searchDocs)
	if err != nil {
		log.Fatalf("创建文档工具失败: %v", err)
	}

	dynamicTools := []tool.BaseTool{crmTool, erpTool, ticketTool, docTool}

	// 创建 toolsearch 中间件（配置保持不变）
	toolSearchMw, err := toolsearch.New(ctx, &toolsearch.Config{
		DynamicTools:       dynamicTools,
		UseModelToolSearch: false,
	})
	if err != nil {
		log.Fatalf("创建 toolsearch 中间件失败: %v", err)
	}

	// Agent 创建和运行部分保持不变
	yourChatModel := pkg.GetBaiLianLLM(ctx)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Model:    yourChatModel, // 替换为你的 ChatModel
		Handlers: []adk.ChatModelAgentMiddleware{toolSearchMw},
	})
	if err != nil {
		log.Fatalf("创建 Agent 失败: %v", err)
	}

	history := []*schema.Message{
		schema.UserMessage("帮我查一下客户“张三”的信息，然后看看他最近买的商品 SKU-001 还有多少库存"),
	}

	iter := agent.Run(ctx, &adk.AgentInput{Messages: history})
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Printf("执行出错: %v", event.Err)
			continue
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			fmt.Printf("Agent: %s\n", event.Output.MessageOutput.Message.Content)
		}
	}
}
