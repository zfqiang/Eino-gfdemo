package promptdemo

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

type UserProfile struct {
	Name      string
	Age       int
	Interests []string
	VIPLevel  int
}

func PromptStruct() {
	ctx := context.Background()
	template := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("你是一个智能推荐助手"),
		schema.UserMessage(`
		用户信息：
			姓名：{name}
			年龄：{age}
			兴趣爱好：{interests}
			会员等级：{vipLevel}

			根据用户的信息，推荐给用户最合适的产品。
		`),
	)

	//准备用户数据
	user := UserProfile{Name: "张三", Age: 18, Interests: []string{"阅读", "游戏"}, VIPLevel: 1}

	variables := map[string]interface{}{
		"name":      user.Name,
		"age":       user.Age,
		"interests": fmt.Sprintf("%v", user.Interests),
		"vipLevel":  user.VIPLevel,
	}

	messages, err := template.Format(ctx, variables)
	if err != nil {
		panic(err)
	}
	for _, m := range messages {
		fmt.Println(m.Content)
	}
}
