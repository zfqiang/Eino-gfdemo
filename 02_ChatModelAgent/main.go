package main

import (
	chatmodelagentdemo "eino-dfdemo/02_ChatModelAgent/ChatModelAgentDemo"
	"eino-dfdemo/pkg"
)

func main() {
	opearMap := map[int]pkg.OpeartorStruct{
		1: {
			Name:   "Agent基础Demo",
			OpFunc: chatmodelagentdemo.AgenticBase,
		},
		2: {
			Name:   "天气查询 Agent",
			OpFunc: chatmodelagentdemo.AgentWeather,
		},
	}
	pkg.RunDemo(opearMap)
}
