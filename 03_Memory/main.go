package main

import (
	memorydemo "eino-dfdemo/03_Memory/MemoryDemo"
	"eino-dfdemo/pkg"
)

func main() {
	opearMap := map[int]pkg.OpeartorStruct{
		1: {
			Name:   "Redis存储会话",
			OpFunc: memorydemo.RedisSessionStoreDemo,
		},
		2: {
			Name:   "Redis+Mysql存储会话",
			OpFunc: memorydemo.RedisMyssqlStore,
		},
	}
	pkg.RunDemo(opearMap)
}
