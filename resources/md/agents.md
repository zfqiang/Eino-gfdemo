# 项目指令

你是一个专业的 Go 后端开发助手，服务于本仓库。

## 常用命令（优先参考）

- 运行全部测试：`go test ./... -v`
- 运行单个包的测试：`go test ./internal/service/... -v`
- 静态检查：`go vet ./...`
- 构建：`go build -o bin/server ./cmd/server`

## 技术栈与目录约定

- Go 1.22+，使用标准库 `net/http` + Gin 框架
- Agent 相关代码统一放在 `internal/agent/` 下
- 工具实现放在 `internal/tools/`，每个工具一个文件
- 配置通过环境变量注入，禁止硬编码敏感信息

## 编码规范

- 所有导出的函数、类型必须有中文注释
- 错误处理必须使用 `fmt.Errorf("...: %w", err)` 包裹，保留调用链
- 禁止在 handler 中直接写业务逻辑，必须委托给 service 层

## 边界（重要）

- ✅ **可以修改**：`internal/` 下的业务代码、`cmd/` 下的入口文件
- ⚠️ **修改前先询问**：`go.mod` 中的依赖版本、数据库迁移文件
- 🚫 **永远不要**：
  - 修改 `vendor/` 目录下的任何文件
  - 提交包含 API Key 或密码的代码
  - 直接修改 `internal/generated/` 下的自动生成代码（会被构建覆盖）

## 已知陷阱

- 运行测试前需要设置 `EINO_API_KEY` 环境变量，否则相关测试会跳过
- `local` backend 目前仅支持 Unix/MacOS，Windows 下需要自定义实现
