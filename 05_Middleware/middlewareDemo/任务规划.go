package middlewaredemo

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/plantask"
)

// 注入 TaskCreate、TaskGet、TaskUpdate、TaskList 四个工具，让 Agent 能拆解复杂任务、管理依赖、跟踪进度。
func TaskPlanning() (adk.ChatModelAgentMiddleware, error) {

	ctx := context.Background()

	// 1. 创建本地文件系统 Backend
	// local.Backend 实现了 filesystem.Backend，同时 plantask.Backend 是其精简子集
	// 因此可以直接用于 plantask 的任务持久化
	// 1. 创建实现了 plantask.Backend 接口的存储后端
	baseDir := "/tmp/eino-tasks/session-001"
	taskBackend := &localPlantaskBackend{rootDir: baseDir}

	plantaskMW, err := plantask.New(ctx, &plantask.Config{
		// 必填：任务存储后端（需按 session 隔离，不同会话使用不同 Backend 实例）
		Backend: taskBackend,

		// 必填：任务文件的存储目录
		BaseDir: "/tasks/my-session-001", // 每个 session 独立目录
	})
	if err != nil {
		return nil, err
	}

	/* history := []*schema.Message{
		schema.UserMessage("帮我规划一个用户登录功能的开发任务，需要拆解为可跟踪的子任务，包含前端页面、后端 API、数据库表设计三部分。"),
	} */

	return plantaskMW, nil
}

// localPlantaskBackend 实现 plantask.Backend 接口
// 使用本地文件系统存储任务 JSON 文件
type localPlantaskBackend struct {
	rootDir string
}

// LsInfo 列出目录下的文件信息
func (b *localPlantaskBackend) LsInfo(ctx context.Context, req *plantask.LsInfoRequest) ([]plantask.FileInfo, error) {
	entries, err := os.ReadDir(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var infos []plantask.FileInfo
	for _, entry := range entries {
		info, _ := entry.Info()
		if info == nil {
			continue
		}
		infos = append(infos, plantask.FileInfo{
			Path:       filepath.Join(req.Path, entry.Name()),
			IsDir:      entry.IsDir(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().String(),
		})
	}
	return infos, nil
}

// Read 读取文件内容
func (b *localPlantaskBackend) Read(ctx context.Context, req *plantask.ReadRequest) (*filesystem.FileContent, error) {
	data, err := os.ReadFile(req.FilePath)
	if err != nil {
		return nil, err
	}
	content := string(data)
	// 简单处理分页：Limit 为 0 时返回全部
	if req.Limit > 0 {
		lines := strings.Split(content, "\n")
		start := req.Offset
		if start < 1 {
			start = 1
		}
		start-- // 转为 0-based
		if start >= len(lines) {
			return &filesystem.FileContent{Content: ""}, nil
		}
		end := start + req.Limit
		if end > len(lines) {
			end = len(lines)
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return &filesystem.FileContent{Content: content}, nil
}

// Write 写入文件内容
func (b *localPlantaskBackend) Write(ctx context.Context, req *plantask.WriteRequest) error {
	if err := os.MkdirAll(filepath.Dir(req.FilePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(req.FilePath, []byte(req.Content), 0644)
}

// Delete 删除文件
func (b *localPlantaskBackend) Delete(ctx context.Context, req *plantask.DeleteRequest) error {
	return os.Remove(req.FilePath)
}
