package main

import (
	"eino-dfdemo/pkg"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

// Message 结构体用于模拟或真实的消息处理
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 接收前端请求
type ChatRequest struct {
	Message string    `json:"message"`
	History []Message `json:"history,omitempty"`
}

func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 关键：打印完整堆栈
				fmt.Printf("[PANIC] %v\n%s\n", err, debug.Stack())

				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

func main() {
	// 1. 创建无默认中间件的 Gin 引擎，避免 gzip/logger 破坏 SSE
	r := gin.New()
	r.Use(CustomRecovery()) // 仅保留恢复恐慌的中间件

	projectRoot := pkg.GetProjectRoot()
	tmplPath := filepath.Join(projectRoot, "ui", "templates", "*")
	r.LoadHTMLGlob(tmplPath)
	// 2. 提供前端页面
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	// 3. SSE 流式接口
	r.POST("/chat/stream", func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Message = strings.TrimSpace(req.Message)
		if req.Message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message不能为空"})
			return
		}

		// 设置 SSE 响应头
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("X-Accel-Buffering", "no") // 防止 Nginx 缓冲

		// 模拟 Agent 思考与流式输出
		// 在实际生产中，这里应替换为调用 LLM API (如 OpenAI, Eino Runner 等)

		//调用模型
		err := simulateAgentResponse(c, req.Message, req.History)
		if err != nil {
			return
		}

		// 假数据模拟
		/* responseText := fakerSimulateAgentResponse(req.Message)
		words := strings.Split(responseText, "")
		for _, word := range words {
			// 构造 SSE 数据格式: data: content\n\n
			data := fmt.Sprintf("data: %s\n\n", word)
			_, err := c.Writer.WriteString(data)
			if err != nil {
				log.Printf("Client disconnected: %v", err)
				return
			}

			// 关键：立即刷新缓冲区
			flusher.Flush()

			// 模拟打字延迟
			time.Sleep(50 * time.Millisecond)
		} */

		// 发送结束标记
		// c.Writer.WriteString("data: [DONE]\n\n")
		// flusher.Flush()
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server启动失败: %v", err)
	}
}

// 调用模型
func simulateAgentResponse(c *gin.Context, input string, history []Message) error {
	// 获取 Flush 器
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return errors.New("Streaming unsupported")
	}

	ctx := c.Request.Context()

	cm := pkg.GetBaiLianLLM(ctx)
	messages := []*schema.Message{schema.SystemMessage("你是一个AI助手")}
	for _, item := range history {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		switch strings.ToLower(item.Role) {
		case "user":
			messages = append(messages, schema.UserMessage(content))
		case "assistant":
			messages = append(messages, schema.AssistantMessage(content, nil))
		}
	}
	messages = append(messages, schema.UserMessage(input))

	stream, err := cm.Stream(ctx, messages)
	if err != nil {
		log.Printf("流式调用模型失败: %v", err)
		writeSSEData(c.Writer, "模型调用失败，请稍后重试。")
		flusher.Flush()
		return err
	}
	defer stream.Close()

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Printf("流式接收失败: %v", err)
			writeSSEData(c.Writer, "流式接收失败，请稍后重试。")
			flusher.Flush()
			return err
		}
		if err := writeSSEData(c.Writer, chunk.Content); err != nil {
			return err
		}
		flusher.Flush()
	}
	c.Writer.WriteString("data: [DONE]\n\n")
	flusher.Flush()
	return nil
}

func writeSSEData(writer io.Writer, content string) error {
	payload, err := json.Marshal(content)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "data: %s\n\n", payload)
	return err
}

// simulateAgentResponse 模拟 Agent 的逻辑处理
func fakerSimulateAgentResponse(input string) string {
	input = strings.ToLower(input)
	if strings.Contains(input, "你好") || strings.Contains(input, "hello") {
		return "你好！我是你的 Go Agent 助手。我可以帮你查询信息、编写代码或进行数据分析。请问有什么可以帮你的吗？"
	}
	if strings.Contains(input, "时间") {
		return "当前时间是: " + time.Now().Format("2006-01-02 15:04:05")
	}
	return "收到你的消息: \"" + input + "\"。这是一个基于 Gin + SSE 的流式响应演示。在生产环境中，这里将连接到大模型 API 或你的业务逻辑引擎。"

}
