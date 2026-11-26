package main

import (
	readanalyze "eino-dfdemo/04_DeepAgent/read_analyze"
	"eino-dfdemo/pkg"
)

func main() {
	opearMap := map[int]pkg.OpeartorStruct{
		1: {
			Name:   "官方实例",
			OpFunc: readanalyze.GuanFangDemo,
		},
		2: {
			Name:   "读文件分析数据",
			OpFunc: readanalyze.ReadAnalyze,
		},
		3: {
			Name:   "文件复杂操作",
			OpFunc: readanalyze.FileComplexOperation,
		},
	}
	pkg.RunDemo(opearMap)
}
