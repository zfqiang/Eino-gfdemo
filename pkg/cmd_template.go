package pkg

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type OpeartorStruct struct {
	Name   string
	OpFunc func()
}

// 运行demo公共方法
func RunDemo(opearMap map[int]OpeartorStruct) {
	for k, v := range opearMap {
		log.Printf("编号：%d，执行方法名称：%s\n", k, v.Name)
	}

	scanner := bufio.NewScanner(os.Stdin)
	log.Println("输入你要执行的序号，输入'exit'退出!!!")

	for {
		fmt.Print("请输入编号：")
		if !scanner.Scan() {
			break
		}

		userInput := strings.TrimSpace(scanner.Text())
		log.Println("输入的编号", userInput)
		if userInput == "" {
			log.Println("输入你要执行的序号，输入exit退出!")
			continue
		}

		if userInput == "exit" {
			log.Println("退出,bye bye")
			break
		}

		operNum, err := strconv.Atoi(userInput)
		if err != nil {
			log.Println("输入编号错误，请重新输入", err)
			continue
		}
		if v, ok := opearMap[operNum]; ok {
			v.OpFunc()
		} else {
			log.Println("输入编号不存在，请重新输入")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
