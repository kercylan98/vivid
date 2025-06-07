package gui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// MainWindow 主窗口结构
type MainWindow struct {
	app    fyne.App
	window fyne.Window
	guiApp *App

	// 配置编辑器组件
	configEditor *ConfigEditor
	codePreview  *widget.Entry

	// 当前项目信息
	currentProject string
	currentFile    string
}

// NewMainWindow 创建新的主窗口
func NewMainWindow(fyneApp fyne.App, guiApp *App) *MainWindow {
	window := fyneApp.NewWindow("VividCTL - Vivid 配置管理工具")
	window.Resize(fyne.NewSize(1200, 800))
	window.CenterOnScreen()

	mw := &MainWindow{
		app:    fyneApp,
		window: window,
		guiApp: guiApp,
	}

	mw.setupUI()
	return mw
}

// setupUI 设置用户界面
func (mw *MainWindow) setupUI() {
	// 创建菜单栏
	mw.createMenuBar()

	// 创建工具栏
	toolbar := mw.createToolbar()

	// 创建配置编辑器
	mw.configEditor = NewConfigEditor(mw.onConfigChanged)

	// 创建代码预览
	mw.codePreview = widget.NewMultiLineEntry()
	mw.codePreview.SetText("// 生成的代码将在这里显示")
	mw.codePreview.Wrapping = fyne.TextWrapWord

	// 创建分割面板
	leftPanel := mw.createLeftPanel()
	rightPanel := mw.createRightPanel()

	mainSplit := container.NewHSplit(leftPanel, rightPanel)
	mainSplit.Offset = 0.6 // 左侧占 60%

	// 主布局
	content := container.NewBorder(
		toolbar,   // 顶部工具栏
		nil,       // 底部
		nil,       // 左侧
		nil,       // 右侧
		mainSplit, // 中心内容
	)

	mw.window.SetContent(content)
}

// createMenuBar 创建菜单栏
func (mw *MainWindow) createMenuBar() {
	// 文件菜单
	fileMenu := fyne.NewMenu("文件",
		fyne.NewMenuItem("新建配置", mw.newConfig),
		fyne.NewMenuItem("打开配置", mw.openConfig),
		fyne.NewMenuItem("保存配置", mw.saveConfig),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("退出", func() { mw.app.Quit() }),
	)

	// 编辑菜单
	editMenu := fyne.NewMenu("编辑",
		fyne.NewMenuItem("添加字段", mw.addField),
		fyne.NewMenuItem("删除字段", mw.deleteField),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("清空配置", mw.clearConfig),
	)

	// 工具菜单
	toolsMenu := fyne.NewMenu("工具",
		fyne.NewMenuItem("生成代码", mw.generateCode),
		fyne.NewMenuItem("导出配置", mw.exportConfig),
		fyne.NewMenuItem("导入模板", mw.importTemplate),
	)

	// 帮助菜单
	helpMenu := fyne.NewMenu("帮助",
		fyne.NewMenuItem("使用说明", mw.showHelp),
		fyne.NewMenuItem("关于", mw.showAbout),
	)

	mainMenu := fyne.NewMainMenu(fileMenu, editMenu, toolsMenu, helpMenu)
	mw.window.SetMainMenu(mainMenu)
}

// createToolbar 创建工具栏
func (mw *MainWindow) createToolbar() *widget.Toolbar {
	return widget.NewToolbar(
		widget.NewToolbarAction(fyne.NewMenuItem("", nil).Icon, mw.newConfig),
		widget.NewToolbarAction(fyne.NewMenuItem("", nil).Icon, mw.openConfig),
		widget.NewToolbarAction(fyne.NewMenuItem("", nil).Icon, mw.saveConfig),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(fyne.NewMenuItem("", nil).Icon, mw.addField),
		widget.NewToolbarAction(fyne.NewMenuItem("", nil).Icon, mw.deleteField),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(fyne.NewMenuItem("", nil).Icon, mw.generateCode),
	)
}

// createLeftPanel 创建左侧面板（配置编辑器）
func (mw *MainWindow) createLeftPanel() fyne.CanvasObject {
	// 项目信息
	projectInfo := widget.NewCard("项目信息", "",
		container.NewVBox(
			widget.NewLabel("包名: "),
			widget.NewLabel("配置名: "),
			widget.NewLabel("文件路径: "),
		),
	)

	// 配置编辑器
	editorCard := widget.NewCard("配置编辑器", "", mw.configEditor.Content())

	return container.NewVBox(projectInfo, editorCard)
}

// createRightPanel 创建右侧面板（代码预览）
func (mw *MainWindow) createRightPanel() fyne.CanvasObject {
	// 代码预览标签页
	codeTab := container.NewAppTabs(
		container.NewTabItem("Go 代码", mw.codePreview),
		container.NewTabItem("使用示例", widget.NewMultiLineEntry()),
	)

	return widget.NewCard("代码预览", "", codeTab)
}

// 事件处理函数
func (mw *MainWindow) onConfigChanged() {
	// 配置变更时更新代码预览
	mw.updateCodePreview()
}

func (mw *MainWindow) updateCodePreview() {
	if mw.configEditor == nil {
		return
	}

	code := mw.configEditor.GenerateCode()
	mw.codePreview.SetText(code)
}

// 菜单和工具栏事件处理
func (mw *MainWindow) newConfig() {
	dialog := NewConfigDialog(mw.window, func(pkg, name, output string) {
		mw.configEditor.NewConfig(pkg, name, output)
		mw.currentFile = output
		mw.updateCodePreview()
	})
	dialog.Show()
}

func (mw *MainWindow) openConfig() {
	dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
		if err != nil || file == nil {
			return
		}
		defer file.Close()

		path := file.URI().Path()
		if err := mw.configEditor.LoadFromFile(path); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.currentFile = path
		mw.updateCodePreview()
	}, mw.window)
}

func (mw *MainWindow) saveConfig() {
	if mw.currentFile == "" {
		mw.saveAsConfig()
		return
	}

	if err := mw.configEditor.SaveToFile(mw.currentFile); err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	dialog.ShowInformation("保存成功", fmt.Sprintf("配置已保存到: %s", mw.currentFile), mw.window)
}

func (mw *MainWindow) saveAsConfig() {
	dialog.ShowFileSave(func(file fyne.URIWriteCloser, err error) {
		if err != nil || file == nil {
			return
		}
		defer file.Close()

		path := file.URI().Path()
		if err := mw.configEditor.SaveToFile(path); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.currentFile = path
		dialog.ShowInformation("保存成功", fmt.Sprintf("配置已保存到: %s", path), mw.window)
	}, mw.window)
}

func (mw *MainWindow) addField() {
	dialog := NewFieldDialog(mw.window, func(name, fieldType, comment, defaultVal string) {
		mw.configEditor.AddField(name, fieldType, comment, defaultVal)
		mw.updateCodePreview()
	})
	dialog.Show()
}

func (mw *MainWindow) deleteField() {
	if mw.configEditor.SelectedField() == "" {
		dialog.ShowInformation("提示", "请先选择要删除的字段", mw.window)
		return
	}

	dialog.ShowConfirm("确认删除",
		fmt.Sprintf("确定要删除字段 '%s' 吗？", mw.configEditor.SelectedField()),
		func(confirmed bool) {
			if confirmed {
				mw.configEditor.DeleteField(mw.configEditor.SelectedField())
				mw.updateCodePreview()
			}
		}, mw.window)
}

func (mw *MainWindow) clearConfig() {
	dialog.ShowConfirm("确认清空", "确定要清空所有配置吗？此操作不可撤销。",
		func(confirmed bool) {
			if confirmed {
				mw.configEditor.Clear()
				mw.updateCodePreview()
			}
		}, mw.window)
}

func (mw *MainWindow) generateCode() {
	if mw.currentFile == "" {
		dialog.ShowError(fmt.Errorf("请先保存配置文件"), mw.window)
		return
	}

	// 生成代码到文件
	outputPath := strings.TrimSuffix(mw.currentFile, filepath.Ext(mw.currentFile)) + ".go"
	if err := mw.configEditor.GenerateToFile(outputPath); err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	dialog.ShowInformation("生成成功", fmt.Sprintf("代码已生成到: %s", outputPath), mw.window)
}

func (mw *MainWindow) exportConfig() {
	dialog.ShowFileSave(func(file fyne.URIWriteCloser, err error) {
		if err != nil || file == nil {
			return
		}
		defer file.Close()

		if err := mw.configEditor.ExportTemplate(file); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		dialog.ShowInformation("导出成功", "配置模板已导出", mw.window)
	}, mw.window)
}

func (mw *MainWindow) importTemplate() {
	dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
		if err != nil || file == nil {
			return
		}
		defer file.Close()

		if err := mw.configEditor.ImportTemplate(file); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.updateCodePreview()
		dialog.ShowInformation("导入成功", "配置模板已导入", mw.window)
	}, mw.window)
}

func (mw *MainWindow) showHelp() {
	helpText := `VividCTL 配置管理工具使用说明

基本操作：
1. 新建配置：文件 -> 新建配置
2. 添加字段：编辑 -> 添加字段，或点击工具栏按钮
3. 编辑字段：双击字段列表中的项目
4. 删除字段：选中字段后，编辑 -> 删除字段
5. 生成代码：工具 -> 生成代码

快捷键：
- Ctrl+N: 新建配置
- Ctrl+O: 打开配置
- Ctrl+S: 保存配置
- Ctrl+G: 生成代码

更多帮助请访问项目文档。`

	dialog.ShowInformation("使用说明", helpText, mw.window)
}

func (mw *MainWindow) showAbout() {
	aboutText := `VividCTL v1.0.0

Vivid 框架官方开发工具集
提供可视化的配置管理和代码生成功能

开发团队：Vivid Framework Team
许可证：MIT License`

	dialog.ShowInformation("关于 VividCTL", aboutText, mw.window)
}

// ShowAndRun 显示窗口并运行应用
func (mw *MainWindow) ShowAndRun() {
	mw.window.ShowAndRun()
}
