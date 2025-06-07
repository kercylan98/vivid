# VividCTL - Vivid 开发工具集

VividCTL 是 Vivid 框架的官方开发工具集，提供配置管理、代码生成等功能，支持命令行和可视化界面两种使用方式。

## 🚀 核心特性

- **📝 配置管理**: 可视化创建和管理配置文件，自动生成 Go 代码
- **🔧 代码生成**: 提供各种代码模板和样板生成功能
- **🎮 双重界面**: 支持命令行和 GUI 两种操作方式
- **🏗️ 模块化架构**: 插件式功能扩展，易于添加新功能
- **🌐 Web 界面**: 现代化的 Web 管理界面，支持远程访问

## 📦 安装

```bash
cd engine/vividctl
go build -o vividctl.exe
```

## 💻 命令行使用

### 配置管理

```bash
# 初始化新配置
vividctl config init -n DatabaseConfig

# 添加字段
vividctl config add -n Host -t string
vividctl config add -n Port -t int
vividctl config add -n Timeout -t time.Duration

# 删除字段
vividctl config del -n SomeField
```

## 🎮 图形界面

### 桌面应用模式

```bash
# 启动桌面应用
vividctl gui

# 或指定桌面模式
vividctl gui --mode desktop
```

启动后将打开 Fyne 构建的桌面应用程序，提供：

- 直观的配置编辑器
- 实时代码预览
- 拖拽式字段管理
- 项目模板库

### Web 界面模式

```bash
# 启动 Web 界面 (默认端口 8080)
vividctl gui --mode web

# 指定端口
vividctl gui --mode web --port 9000
```

然后访问 `http://localhost:8080` 使用 Web 界面。

## 🏗️ 架构设计

### 模块化系统

VividCTL 采用模块化架构，每个功能都是独立的模块：

```go
type Module interface {
    ID() string
    Name() string
    Description() string
    Icon() string
    Enabled() bool
    
    // 桌面应用相关
    CreateDesktopContent() interface{}
    
    // Web应用相关
    RegisterWebRoutes(mux *http.ServeMux)
    GetWebContent() string
}
```

### 内置模块

| 模块   | 图标  | 状态    | 功能描述      |
|------|-----|-------|-----------|
| 配置管理 | 📝  | ✅ 已启用 | 创建和管理配置文件 |
| 代码生成 | 🔧  | ✅ 已启用 | 生成代码模板和样板 |
| 项目管理 | 📁  | ❌ 开发中 | 项目结构和依赖管理 |
| 开发工具 | 🛠️ | ❌ 开发中 | 各种开发辅助工具  |
| 系统设置 | ⚙️  | ✅ 已启用 | 系统配置和偏好设置 |

## 🔧 自定义模块

可以轻松添加自定义功能模块：

```go
// 1. 实现 Module 接口
type MyModule struct{}

func (m *MyModule) ID() string { return "my-module" }
func (m *MyModule) Name() string { return "我的模块" }
// ... 实现其他方法

// 2. 注册模块
app := NewApp()
app.RegisterModule(&MyModule{})
```

## 📁 项目结构

```
engine/vividctl/
├── cmd/                    # 命令行命令
│   ├── root.go            # 根命令
│   ├── config.go          # 配置管理命令
│   └── gui.go             # GUI 命令
├── internal/
│   ├── config/            # 配置处理逻辑
│   │   └── manipulator.go # AST 操作
│   └── gui/               # GUI 相关代码
│       ├── app.go         # 应用核心
│       ├── modules.go     # 功能模块实现
│       ├── window.go      # 桌面窗口 (Fyne)
│       ├── editor.go      # 配置编辑器
│       └── web.go         # Web 服务器
├── main.go                # 入口点
├── go.mod                 # Go 模块配置
└── README.md              # 文档
```

## 🎨 界面预览

### Web 界面特性

- **🎨 现代化设计**: 响应式布局，美观的 UI 设计
- **📱 移动端支持**: 支持手机和平板设备访问
- **🌙 暗色主题**: 可切换的主题支持
- **⚡ 实时更新**: 配置更改实时反映在代码预览中
- **🔍 搜索功能**: 快速查找和过滤功能

### 桌面应用特性

- **🖱️ 原生体验**: 原生桌面应用体验
- **📝 富文本编辑**: 支持语法高亮的代码编辑器
- **💾 本地存储**: 配置文件本地保存和管理
- **⌨️ 快捷键**: 完整的键盘快捷键支持

## 🛠️ 开发指南

### 添加新的命令行功能

1. 在 `cmd/` 目录下创建新的命令文件
2. 实现 Cobra 命令结构
3. 在 `root.go` 中注册命令

### 添加新的 GUI 模块

1. 在 `modules.go` 中实现 `Module` 接口
2. 在 `app.go` 的 `registerBuiltinModules()` 中注册
3. 实现桌面和 Web 两种界面

### 代码生成模板

配置生成的代码模板位于 `internal/config/` 目录，支持：

- 私有字段和公有方法
- Options Pattern 实现
- Go Doc 规范的注释
- 类型别名和接口定义

## 📚 使用示例

详细的使用示例请参考 [EXAMPLES.md](EXAMPLES.md)

## 🤝 贡献

欢迎贡献代码！请确保：

- 遵循 Go 编码规范
- 添加适当的测试用例
- 更新相关文档
- 提交前运行 `go fmt` 和 `go vet`

## 📄 许可证

MIT License

---

> 💡 **提示**: 这是一个开发阶段的工具，功能还在不断完善中。如有问题或建议，欢迎提交 Issue！ 