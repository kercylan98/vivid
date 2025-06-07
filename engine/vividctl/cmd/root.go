package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vividctl",
	Short: "Vivid 开发工具集",
	Long: `vividctl 是 Vivid 框架的开发工具集，提供配置管理、代码生成等功能。

支持的功能：
  - config: 配置文件管理工具 (命令行)
  - gui: 可视化界面 (图形界面)
  - 更多功能正在开发中...`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 执行错误: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// 添加子命令
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(guiCmd)
}
