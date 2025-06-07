package gui

import (
	"fmt"
	"net/http"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/widget"
)

// App GUI 应用核心
type App struct {
	modules     map[string]Module
	moduleOrder []string // 保持模块顺序
}

// Module 功能模块接口
type Module interface {
	ID() string
	Name() string
	Description() string
	Icon() string
	Enabled() bool

	// 桌面应用相关 (暂时返回字符串，后续可扩展)
	CreateDesktopContent() interface{}

	// Web应用相关
	RegisterWebRoutes(mux *http.ServeMux)
	GetWebContent() string
}

// NewApp 创建应用实例
func NewApp() *App {
	app := &App{
		modules:     make(map[string]Module),
		moduleOrder: []string{},
	}

	// 注册内置模块
	app.registerBuiltinModules()

	return app
}

// RegisterModule 注册功能模块
func (a *App) RegisterModule(module Module) {
	a.modules[module.ID()] = module
	// 避免重复添加
	for _, id := range a.moduleOrder {
		if id == module.ID() {
			return
		}
	}
	a.moduleOrder = append(a.moduleOrder, module.ID())
}

// GetModule 获取功能模块
func (a *App) GetModule(id string) (Module, bool) {
	module, exists := a.modules[id]
	return module, exists
}

// GetModules 获取所有模块（保持顺序）
func (a *App) GetModules() []Module {
	var modules []Module
	for _, id := range a.moduleOrder {
		if module, exists := a.modules[id]; exists {
			modules = append(modules, module)
		}
	}
	return modules
}

// GetModulesMap 获取所有模块的map形式（兼容旧接口）
func (a *App) GetModulesMap() map[string]Module {
	return a.modules
}

// registerBuiltinModules 注册内置模块
func (a *App) registerBuiltinModules() {
	// 配置管理模块
	a.RegisterModule(&ConfigModule{})

	// 代码生成模块
	a.RegisterModule(&GeneratorModule{})

	// 项目管理模块 (暂未实现)
	a.RegisterModule(&ProjectModule{enabled: false})

	// 开发工具模块 (暂未实现)
	a.RegisterModule(&ToolsModule{enabled: false})

	// 系统设置模块
	a.RegisterModule(&SystemModule{})
}

// StartDesktopApp 启动桌面应用
func StartDesktopApp() error {
	myApp := app.New()
	myApp.SetIcon(theme.ComputerIcon())
	myApp.Settings().SetTheme(theme.LightTheme())

	// 创建应用实例
	guiApp := NewApp()

	// 创建主窗口
	mainWindow := NewMainWindow(myApp, guiApp)

	// 显示并运行
	mainWindow.ShowAndRun()
	return nil
}

// StartWebApp 启动 Web 应用
func StartWebApp(port int) error {
	// 创建应用实例
	guiApp := NewApp()

	// 创建 Web 服务器
	server := NewWebServer(guiApp)

	fmt.Printf("🌐 启动 Web GUI 界面...\n")
	fmt.Printf("📱 访问地址: http://localhost:%d\n", port)
	fmt.Printf("💡 使用 Ctrl+C 停止服务\n")
	fmt.Printf("📋 可用模块:\n")

	// 显示可用模块
	modules := guiApp.GetModules()
	for _, module := range modules {
		status := "❌"
		if module.Enabled() {
			status = "✅"
		}
		fmt.Printf("   %s %s %s - %s\n", status, module.Icon(), module.Name(), module.Description())
	}

	fmt.Printf("\n🔧 服务器正在启动，请稍等...\n")

	err := http.ListenAndServe(fmt.Sprintf(":%d", port), server)
	if err != nil {
		fmt.Printf("❌ 服务器启动失败: %v\n", err)
		return err
	}

	return nil
}

// 应用图标资源 (嵌入的 PNG 数据)
var resourceIconPng = &widget.Card{}
