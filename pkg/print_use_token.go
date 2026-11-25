package pkg

import (
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// 打印 Token 使用情况
func PrintTokenUsage(response *schema.Message) {
	if response.ResponseMeta != nil && response.ResponseMeta.Usage != nil {
		fmt.Printf("\\nToken 使用统计:\\n")
		fmt.Printf("  输入 Token: %d\\n", response.ResponseMeta.Usage.PromptTokens)
		fmt.Printf("  输出 Token: %d\\n", response.ResponseMeta.Usage.CompletionTokens)
		fmt.Printf("  总计 Token: %d\\n", response.ResponseMeta.Usage.TotalTokens)
		if response.ResponseMeta.Usage.PromptTokenDetails.CachedTokens > 0 {
			fmt.Printf("  缓存 Token: %d\\n", response.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
		}
	}
}
