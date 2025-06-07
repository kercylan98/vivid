package cmd

import (
	"github.com/kercylan98/vivid/engine/vividctl/internal/gui"
	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "启动可视化界面",
	Long: `启动 VividCTL 的图形用户界面，提供可视化的配置管理功能。

功能特性：
  - 可视化配置编辑器
  - 实时代码预览
  - 拖拽式字段管理
  - 配置模板库
  - 项目管理`,
	RunE: runGUI,
}

var (
	guiPort int
	guiMode string
)

func init() {
	// 添加 GUI 相关参数
	guiCmd.Flags().IntVarP(&guiPort, "port", "p", 8080, "Web GUI 服务端口 (仅在 web 模式下有效)")
	guiCmd.Flags().StringVarP(&guiMode, "mode", "m", "desktop", "GUI 模式: desktop (桌面应用) 或 web (Web 界面)")
}

// runGUI 启动 GUI 界面
func runGUI(cmd *cobra.Command, args []string) error {
	switch guiMode {
	case "desktop":
		return gui.StartDesktopApp()
	case "web":
		return gui.StartWebApp(guiPort)
	default:
		return cmd.Usage()
	}
}
