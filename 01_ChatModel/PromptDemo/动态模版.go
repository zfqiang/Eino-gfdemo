package promptdemo

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

type ConversationStyle string

const (
	StyleProfessional ConversationStyle = "professional" // 创意
	StyleCasual       ConversationStyle = "casual"       // 平衡
	StyleFriendly     ConversationStyle = "friendly"     // 精确
	StyleFormal       ConversationStyle = "formal"
)

func PromptDynamicTemplate() {
	ctx := context.Background()
	promptTemplate := createDynamicTemplate(StyleProfessional, "软件架构")
	variables := map[string]interface{}{
		"query": "什么是微服务架构",
	}

	message, err := promptTemplate.Format(ctx, variables)
	if err != nil {
		panic(err)
	}
	log.Println(message)
}

func createDynamicTemplate(style ConversationStyle, domain string) *prompt.DefaultChatTemplate {
	var systemPrompt string
	switch style {
	case StyleProfessional:
		systemPrompt = fmt.Sprintf("你是一个%s的专家，请以%s风格回答我的问题。", domain, style)
	case StyleCasual:
		systemPrompt = fmt.Sprintf("你是一个%s的朋友，请以轻松通俗的方式聊天。", domain)
	case StyleFriendly:
		systemPrompt = fmt.Sprintf("你是一个热情友好的%s助手，请以温柔鼓励的语气交流。", domain)
	case StyleFormal:
		systemPrompt = fmt.Sprintf("你是一个正式%s的顾问，请以严谨正式的表达方式", domain)
	default:
		systemPrompt = fmt.Sprintf("你是一个%s助手。", domain)

	}
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(systemPrompt),
		schema.UserMessage("{query}"),
	)
}
