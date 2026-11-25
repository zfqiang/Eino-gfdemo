package main

import (
	chatmodeldemo "eino-dfdemo/01_ChatModel/ChatModelDemo"
	promptdemo "eino-dfdemo/01_ChatModel/PromptDemo"
	"eino-dfdemo/pkg"
)

func main() {
	opearMap := map[int]pkg.OpeartorStruct{
		1: {
			Name:   "简单对话直接输出",
			OpFunc: chatmodeldemo.SimpleGenerate,
		},
		2: {
			Name:   "简单对话流式输出",
			OpFunc: chatmodeldemo.SimpleStream,
		},
		3: {
			Name:   "简单模版输出",
			OpFunc: promptdemo.PromptBase,
		},
		4: {
			Name:   "动态模版输出",
			OpFunc: promptdemo.PromptDynamicTemplate,
		},
		5: {
			Name:   "结构体模版输出",
			OpFunc: promptdemo.PromptStruct,
		},
		6: {
			Name:   "思维链输出",
			OpFunc: promptdemo.PromptChain,
		},
	}
	pkg.RunDemo(opearMap)

}
