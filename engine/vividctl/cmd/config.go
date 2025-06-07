package cmd

import (
	"bufio"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/kercylan98/vivid/engine/vividctl/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置文件管理工具",
	Long: `配置文件管理工具，支持创建、修改配置文件。

功能：
  init: 初始化配置文件
  add:  添加配置字段
  del:  删除配置字段`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化配置文件",
	Long:  `初始化一个新的配置文件，包含基本的配置结构。`,
	RunE:  runConfigInit,
}

var configAddCmd = &cobra.Command{
	Use:   "add",
	Short: "添加配置字段",
	Long:  `向现有配置文件添加新的字段和对应的方法。`,
	RunE:  runConfigAdd,
}

var configDelCmd = &cobra.Command{
	Use:   "del",
	Short: "删除配置字段",
	Long:  `从现有配置文件删除指定字段和对应的方法。`,
	RunE:  runConfigDel,
}

var (
	configName   string
	fieldName    string
	fieldType    string
	fieldComment string
	fieldDefault string
	outputPath   string
	packageName  string
)

func init() {
	// 添加子命令
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configAddCmd)
	configCmd.AddCommand(configDelCmd)

	// init 命令参数
	configInitCmd.Flags().StringVarP(&configName, "name", "n", "", "配置名称（必填）")
	configInitCmd.Flags().StringVarP(&outputPath, "output", "o", "", "输出文件路径（必填）")
	configInitCmd.Flags().StringVarP(&packageName, "package", "p", "", "包名（必填）")
	configInitCmd.MarkFlagRequired("name")
	configInitCmd.MarkFlagRequired("output")
	configInitCmd.MarkFlagRequired("package")

	// add 命令参数
	configAddCmd.Flags().StringVarP(&fieldName, "name", "n", "", "字段名称（必填）")
	configAddCmd.Flags().StringVarP(&fieldType, "type", "t", "", "字段类型（必填）")
	configAddCmd.Flags().StringVarP(&fieldComment, "comment", "c", "", "字段注释")
	configAddCmd.Flags().StringVarP(&fieldDefault, "default", "d", "", "默认值")
	configAddCmd.Flags().StringVarP(&outputPath, "file", "f", "", "配置文件路径（必填）")
	configAddCmd.MarkFlagRequired("name")
	configAddCmd.MarkFlagRequired("type")
	configAddCmd.MarkFlagRequired("file")

	// del 命令参数
	configDelCmd.Flags().StringVarP(&fieldName, "name", "n", "", "字段名称（必填）")
	configDelCmd.Flags().StringVarP(&outputPath, "file", "f", "", "配置文件路径（必填）")
	configDelCmd.MarkFlagRequired("name")
	configDelCmd.MarkFlagRequired("file")
}

// runConfigInit 执行配置初始化
func runConfigInit(cmd *cobra.Command, args []string) error {
	// 检查文件是否存在
	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("⚠️ 文件 %s 已存在，是否覆盖？(y/N): ", outputPath)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	// 生成配置文件
	content := generateConfigFile(packageName, configName)

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	fmt.Printf("✅ 配置文件已生成: %s\n", outputPath)
	fmt.Printf("📖 使用方法:\n")
	fmt.Printf("  添加字段: vividctl config add -f %s -n FieldName -t string -c \"字段描述\" -d \"默认值\"\n", outputPath)
	fmt.Printf("  删除字段: vividctl config del -f %s -n FieldName\n", outputPath)
	return nil
}

// runConfigAdd 执行添加字段
func runConfigAdd(cmd *cobra.Command, args []string) error {
	// 解析现有文件
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, outputPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("解析文件失败: %v", err)
	}

	// 创建配置操作器
	manipulator := config.NewConfigManipulator(fset, node)

	// 检查字段是否已存在
	if manipulator.FieldExists(fieldName) {
		return fmt.Errorf("字段 %s 已存在", fieldName)
	}

	// 添加字段
	if err := manipulator.AddField(fieldName, fieldType, fieldComment, fieldDefault); err != nil {
		return fmt.Errorf("添加字段失败: %v", err)
	}

	// 格式化并写入文件
	if err := writeFormattedFile(fset, node, outputPath); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	fmt.Printf("✅ 字段 %s 已添加到配置\n", fieldName)
	fmt.Printf("🔧 已生成相关方法:\n")
	fmt.Printf("  - Get%s(): 获取字段值\n", fieldName)
	fmt.Printf("  - With%s(): 设置字段值 (链式调用)\n", fieldName)
	fmt.Printf("  - With%s(): 选项函数\n", fieldName)
	return nil
}

// runConfigDel 执行删除字段
func runConfigDel(cmd *cobra.Command, args []string) error {
	// 解析现有文件
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, outputPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("解析文件失败: %v", err)
	}

	// 创建配置操作器
	manipulator := config.NewConfigManipulator(fset, node)

	// 检查字段是否存在
	if !manipulator.FieldExists(fieldName) {
		return fmt.Errorf("字段 %s 不存在", fieldName)
	}

	// 二次确认
	fmt.Printf("⚠️ 确定要删除字段 %s 及其相关方法吗？(y/N): ", fieldName)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "y" && input != "yes" {
		fmt.Println("操作已取消")
		return nil
	}

	// 删除字段
	if err := manipulator.RemoveField(fieldName); err != nil {
		return fmt.Errorf("删除字段失败: %v", err)
	}

	// 格式化并写入文件
	if err := writeFormattedFile(fset, node, outputPath); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	fmt.Printf("✅ 字段 %s 已从配置中删除\n", fieldName)
	fmt.Printf("🗑️ 已删除相关方法:\n")
	fmt.Printf("  - Get%s()\n", fieldName)
	fmt.Printf("  - With%s()\n", fieldName)
	fmt.Printf("  - With%s() (选项函数)\n", fieldName)
	return nil
}

// generateConfigFile 生成配置文件内容
func generateConfigFile(pkg, name string) string {
	template := `package %s

import "github.com/kercylan98/vivid/src/configurator"

// New%sConfiguration 创建新的%s配置实例
//
// 支持通过选项模式进行配置，提供灵活的配置方式。
//
// 参数:
//   - options: 可变数量的配置选项
//
// 返回:
//   - *%sConfiguration: 配置实例
func New%sConfiguration(options ...%sOption) *%sConfiguration {
	c := &%sConfiguration{}
	for _, option := range options {
		option(c)
	}
	return c
}

type (
	// %sConfigurator 配置器接口
	%sConfigurator   = configurator.Configurator[*%sConfiguration]
	
	// %sConfiguratorFN 配置器函数类型
	%sConfiguratorFN = configurator.FN[*%sConfiguration]
	
	// %sOption 配置选项函数类型
	%sOption         = configurator.Option[*%sConfiguration]
	
	// %sConfiguration %s配置结构体
	//
	// 所有字段均为私有，通过 GetXxx 方法获取值，通过 WithXxx 方法设置值。
	%sConfiguration struct {
		// 字段将通过 vividctl config add 命令添加
	}
)
`
	return fmt.Sprintf(template,
		pkg, name, name,
		name, name, name, name, name,
		name, name, name,
		name, name, name,
		name, name, name,
		name, name, name)
}

// writeFormattedFile 格式化并写入文件
func writeFormattedFile(fset *token.FileSet, node any, filename string) error {
	var buf strings.Builder
	if err := format.Node(&buf, fset, node); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(buf.String()), 0644)
}
