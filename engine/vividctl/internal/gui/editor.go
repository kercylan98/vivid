package gui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ConfigField 配置字段结构
type ConfigField struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Comment      string `json:"comment"`
	DefaultValue string `json:"default_value"`
}

// ConfigData 配置数据结构
type ConfigData struct {
	PackageName string        `json:"package_name"`
	ConfigName  string        `json:"config_name"`
	OutputPath  string        `json:"output_path"`
	Fields      []ConfigField `json:"fields"`
}

// ConfigEditor 配置编辑器
type ConfigEditor struct {
	data     *ConfigData
	callback func()

	// UI 组件
	content       *fyne.Container
	fieldsList    *widget.List
	selectedIndex int
}

// NewConfigEditor 创建配置编辑器
func NewConfigEditor(callback func()) *ConfigEditor {
	ce := &ConfigEditor{
		data:          &ConfigData{Fields: []ConfigField{}},
		callback:      callback,
		selectedIndex: -1,
	}

	ce.setupUI()
	return ce
}

// setupUI 设置编辑器界面
func (ce *ConfigEditor) setupUI() {
	// 字段列表
	ce.fieldsList = widget.NewList(
		func() int {
			return len(ce.data.Fields)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(nil),
				widget.NewLabel("字段名"),
				widget.NewLabel("类型"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(ce.data.Fields) {
				return
			}

			field := ce.data.Fields[id]
			hbox := item.(*fyne.Container)

			// 更新显示内容
			hbox.Objects[1].(*widget.Label).SetText(field.Name)
			hbox.Objects[2].(*widget.Label).SetText(field.Type)
		},
	)

	ce.fieldsList.OnSelected = func(id widget.ListItemID) {
		ce.selectedIndex = id
	}

	// 字段操作按钮
	addBtn := widget.NewButton("添加字段", func() {
		ce.showAddFieldDialog()
	})

	editBtn := widget.NewButton("编辑字段", func() {
		if ce.selectedIndex >= 0 && ce.selectedIndex < len(ce.data.Fields) {
			ce.showEditFieldDialog(ce.selectedIndex)
		}
	})

	deleteBtn := widget.NewButton("删除字段", func() {
		if ce.selectedIndex >= 0 && ce.selectedIndex < len(ce.data.Fields) {
			ce.DeleteField(ce.data.Fields[ce.selectedIndex].Name)
		}
	})

	buttons := container.NewHBox(addBtn, editBtn, deleteBtn)

	ce.content = container.NewBorder(
		nil,
		buttons,
		nil,
		nil,
		ce.fieldsList,
	)
}

// Content 返回编辑器内容
func (ce *ConfigEditor) Content() fyne.CanvasObject {
	return ce.content
}

// NewConfig 创建新配置
func (ce *ConfigEditor) NewConfig(pkg, name, output string) {
	ce.data = &ConfigData{
		PackageName: pkg,
		ConfigName:  name,
		OutputPath:  output,
		Fields:      []ConfigField{},
	}
	ce.selectedIndex = -1
	ce.refresh()
}

// AddField 添加字段
func (ce *ConfigEditor) AddField(name, fieldType, comment, defaultVal string) {
	field := ConfigField{
		Name:         name,
		Type:         fieldType,
		Comment:      comment,
		DefaultValue: defaultVal,
	}

	ce.data.Fields = append(ce.data.Fields, field)
	ce.refresh()
}

// DeleteField 删除字段
func (ce *ConfigEditor) DeleteField(fieldName string) {
	for i, field := range ce.data.Fields {
		if field.Name == fieldName {
			ce.data.Fields = append(ce.data.Fields[:i], ce.data.Fields[i+1:]...)
			break
		}
	}
	ce.selectedIndex = -1
	ce.refresh()
}

// SelectedField 获取选中的字段名
func (ce *ConfigEditor) SelectedField() string {
	if ce.selectedIndex >= 0 && ce.selectedIndex < len(ce.data.Fields) {
		return ce.data.Fields[ce.selectedIndex].Name
	}
	return ""
}

// Clear 清空配置
func (ce *ConfigEditor) Clear() {
	ce.data.Fields = []ConfigField{}
	ce.selectedIndex = -1
	ce.refresh()
}

// GenerateCode 生成代码
func (ce *ConfigEditor) GenerateCode() string {
	if ce.data.PackageName == "" || ce.data.ConfigName == "" {
		return "// 请先设置包名和配置名"
	}

	// 基础配置代码
	code := fmt.Sprintf(`package %s

import "github.com/kercylan98/vivid/src/configurator"

// New%sConfiguration 创建新的%s配置实例
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
	%sConfiguration struct {
`, ce.data.PackageName, ce.data.ConfigName, ce.data.ConfigName,
		ce.data.ConfigName, ce.data.ConfigName, ce.data.ConfigName, ce.data.ConfigName,
		ce.data.ConfigName, ce.data.ConfigName, ce.data.ConfigName,
		ce.data.ConfigName, ce.data.ConfigName, ce.data.ConfigName,
		ce.data.ConfigName, ce.data.ConfigName,
		ce.data.ConfigName, ce.data.ConfigName, ce.data.ConfigName)

	// 添加字段
	for _, field := range ce.data.Fields {
		privateName := fmt.Sprintf("%s%s", string(rune(field.Name[0])+32), field.Name[1:])
		if field.Comment != "" {
			code += fmt.Sprintf("\t\t%s %s // %s\n", privateName, field.Type, field.Comment)
		} else {
			code += fmt.Sprintf("\t\t%s %s\n", privateName, field.Type)
		}
	}

	code += "\t}\n)\n"

	// 添加方法
	for _, field := range ce.data.Fields {
		privateName := fmt.Sprintf("%s%s", string(rune(field.Name[0])+32), field.Name[1:])

		// Getter 方法
		code += fmt.Sprintf(`
// Get%s 获取%s
func (c *%sConfiguration) Get%s() %s {
	return c.%s
}

`, field.Name, field.Comment, ce.data.ConfigName, field.Name, field.Type, privateName)

		// Setter 方法
		paramName := fmt.Sprintf("%s%s", string(rune(field.Name[0])+32), field.Name[1:])
		code += fmt.Sprintf(`// With%s 设置%s
func (c *%sConfiguration) With%s(%s %s) *%sConfiguration {
	c.%s = %s
	return c
}

`, field.Name, field.Comment, ce.data.ConfigName, field.Name, paramName, field.Type, ce.data.ConfigName, privateName, paramName)

		// 选项函数
		code += fmt.Sprintf(`// With%s 创建设置%s的选项
func With%s(%s %s) %sOption {
	return func(configuration *%sConfiguration) {
		configuration.With%s(%s)
	}
}

`, field.Name, field.Comment, field.Name, paramName, field.Type, ce.data.ConfigName, ce.data.ConfigName, field.Name, paramName)
	}

	return code
}

// GenerateToFile 生成代码到文件
func (ce *ConfigEditor) GenerateToFile(filename string) error {
	code := ce.GenerateCode()
	return os.WriteFile(filename, []byte(code), 0644)
}

// LoadFromFile 从文件加载配置
func (ce *ConfigEditor) LoadFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, ce.data)
}

// SaveToFile 保存配置到文件
func (ce *ConfigEditor) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(ce.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// ExportTemplate 导出配置模板
func (ce *ConfigEditor) ExportTemplate(writer io.Writer) error {
	data, err := json.MarshalIndent(ce.data, "", "  ")
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	return err
}

// ImportTemplate 导入配置模板
func (ce *ConfigEditor) ImportTemplate(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, ce.data)
}

// refresh 刷新界面
func (ce *ConfigEditor) refresh() {
	ce.fieldsList.Refresh()
	if ce.callback != nil {
		ce.callback()
	}
}

// showAddFieldDialog 显示添加字段对话框
func (ce *ConfigEditor) showAddFieldDialog() {
	// 这里应该显示字段编辑对话框
	// 为了简化，暂时使用基本输入
}

// showEditFieldDialog 显示编辑字段对话框
func (ce *ConfigEditor) showEditFieldDialog(index int) {
	// 这里应该显示字段编辑对话框
	// 为了简化，暂时使用基本输入
}

// NewConfigDialog 创建配置对话框
func NewConfigDialog(parent fyne.Window, callback func(pkg, name, output string)) dialog.Dialog {
	pkgEntry := widget.NewEntry()
	pkgEntry.SetPlaceHolder("输入包名，如: database")

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("输入配置名，如: Database")

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("输入输出路径，如: database_config.json")

	form := container.NewVBox(
		widget.NewLabel("包名:"),
		pkgEntry,
		widget.NewLabel("配置名:"),
		nameEntry,
		widget.NewLabel("输出路径:"),
		outputEntry,
	)

	return dialog.NewCustomConfirm(
		"新建配置",
		"确定",
		"取消",
		form,
		func(confirmed bool) {
			if confirmed && pkgEntry.Text != "" && nameEntry.Text != "" && outputEntry.Text != "" {
				callback(pkgEntry.Text, nameEntry.Text, outputEntry.Text)
			}
		},
		parent,
	)
}

// NewFieldDialog 创建字段对话框
func NewFieldDialog(parent fyne.Window, callback func(name, fieldType, comment, defaultVal string)) dialog.Dialog {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("输入字段名，如: Host")

	typeSelect := widget.NewSelect(
		[]string{"string", "int", "int32", "int64", "bool", "float64", "time.Duration"},
		nil,
	)
	typeSelect.SetSelected("string")

	commentEntry := widget.NewEntry()
	commentEntry.SetPlaceHolder("输入字段描述")

	defaultEntry := widget.NewEntry()
	defaultEntry.SetPlaceHolder("输入默认值（可选）")

	form := container.NewVBox(
		widget.NewLabel("字段名:"),
		nameEntry,
		widget.NewLabel("字段类型:"),
		typeSelect,
		widget.NewLabel("字段描述:"),
		commentEntry,
		widget.NewLabel("默认值:"),
		defaultEntry,
	)

	return dialog.NewCustomConfirm(
		"添加字段",
		"确定",
		"取消",
		form,
		func(confirmed bool) {
			if confirmed && nameEntry.Text != "" && typeSelect.Selected != "" {
				callback(nameEntry.Text, typeSelect.Selected, commentEntry.Text, defaultEntry.Text)
			}
		},
		parent,
	)
}
