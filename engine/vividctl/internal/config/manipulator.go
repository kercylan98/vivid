package config

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"
)

// ConfigManipulator 配置文件操作器
type ConfigManipulator struct {
	fset *token.FileSet
	file *ast.File
}

// NewConfigManipulator 创建配置文件操作器
func NewConfigManipulator(fset *token.FileSet, file *ast.File) *ConfigManipulator {
	return &ConfigManipulator{
		fset: fset,
		file: file,
	}
}

// AddField 添加字段到配置结构体
func (m *ConfigManipulator) AddField(fieldName, fieldType, comment, defaultValue string) error {
	configStruct := m.findConfigStruct()
	if configStruct == nil {
		return fmt.Errorf("未找到配置结构体")
	}

	// 添加私有字段到结构体
	field := &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(fieldName)},
		Type:  parseTypeExpr(fieldType),
	}

	if comment != "" {
		field.Comment = &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: "// " + comment},
			},
		}
	}

	// 添加字段到结构体
	configStruct.Fields.List = append(configStruct.Fields.List, field)

	// 添加公共方法
	//m.addGetterMethod(fieldName, privateFieldName, fieldType, comment)
	m.addSetterMethod(fieldName, fieldName, fieldType, comment)
	m.addOptionFunction(fieldName, fieldName, fieldType, comment)

	// 更新构造函数的默认值
	if defaultValue != "" {
		m.updateConstructorDefault(fieldName, defaultValue)
	}

	return nil
}

// RemoveField 从配置结构体删除字段
func (m *ConfigManipulator) RemoveField(fieldName string) error {
	privateFieldName := strings.ToLower(fieldName[:1]) + fieldName[1:]

	// 从结构体删除字段
	configStruct := m.findConfigStruct()
	if configStruct == nil {
		return fmt.Errorf("未找到配置结构体")
	}

	// 删除字段
	var newFields []*ast.Field
	for _, field := range configStruct.Fields.List {
		keep := true
		for _, name := range field.Names {
			if name.Name == privateFieldName {
				keep = false
				break
			}
		}
		if keep {
			newFields = append(newFields, field)
		}
	}
	configStruct.Fields.List = newFields

	// 删除相关方法
	m.removeMethod("Get" + fieldName)
	m.removeMethod("With" + fieldName)
	m.removeGlobalFunction("With" + fieldName)

	// 从构造函数删除默认值
	m.removeConstructorDefault(privateFieldName)

	return nil
}

// FieldExists 检查字段是否存在
func (m *ConfigManipulator) FieldExists(fieldName string) bool {
	privateFieldName := strings.ToLower(fieldName[:1]) + fieldName[1:]
	configStruct := m.findConfigStruct()
	if configStruct == nil {
		return false
	}

	for _, field := range configStruct.Fields.List {
		for _, name := range field.Names {
			if name.Name == privateFieldName {
				return true
			}
		}
	}
	return false
}

// findConfigStruct 查找配置结构体
func (m *ConfigManipulator) findConfigStruct() *ast.StructType {
	for _, decl := range m.file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if strings.HasSuffix(typeSpec.Name.Name, "Configuration") {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							return structType
						}
					}
				}
			}
		}
	}
	return nil
}

// addGetterMethod 添加 Getter 方法
func (m *ConfigManipulator) addGetterMethod(publicName, privateName, fieldType, comment string) {
	methodName := "Get" + publicName

	// 生成方法注释
	methodComment := fmt.Sprintf("// %s 获取%s", methodName, comment)
	if comment == "" {
		methodComment = fmt.Sprintf("// %s 获取%s", methodName, privateName)
	}

	method := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: methodComment},
				{Text: "//"},
				{Text: "// 返回:"},
				{Text: fmt.Sprintf("//   - %s: %s", fieldType, comment)},
			},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("c")},
					Type: &ast.StarExpr{
						X: m.getConfigTypeName(),
					},
				},
			},
		},
		Name: ast.NewIdent(methodName),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: parseTypeExpr(fieldType)},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.SelectorExpr{
							X:   ast.NewIdent("c"),
							Sel: ast.NewIdent(privateName),
						},
					},
				},
			},
		},
	}

	m.file.Decls = append(m.file.Decls, method)
}

// addSetterMethod 添加 Setter 方法
func (m *ConfigManipulator) addSetterMethod(publicName, privateName, fieldType, comment string) {
	methodName := "With" + publicName
	paramName := strings.ToLower(publicName)

	// 生成方法注释
	methodComment := fmt.Sprintf("// %s 设置%s", methodName, comment)
	if comment == "" {
		methodComment = fmt.Sprintf("// %s 设置%s", methodName, privateName)
	}

	configTypeName := m.getConfigTypeName()

	method := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: methodComment},
				{Text: "//"},
				{Text: "// 参数:"},
				{Text: fmt.Sprintf("//   - %s: %s", paramName, comment)},
				{Text: "//"},
				{Text: "// 返回:"},
				{Text: fmt.Sprintf("//   - *%s: 配置实例，支持链式调用", configTypeName.Name)},
			},
		},
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{ast.NewIdent("c")},
					Type: &ast.StarExpr{
						X: configTypeName,
					},
				},
			},
		},
		Name: ast.NewIdent(methodName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent(paramName)},
						Type:  parseTypeExpr(fieldType),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{
							X: configTypeName,
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   ast.NewIdent("c"),
							Sel: ast.NewIdent(privateName),
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						ast.NewIdent(paramName),
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{
						ast.NewIdent("c"),
					},
				},
			},
		},
	}

	m.file.Decls = append(m.file.Decls, method)
}

// addOptionFunction 添加选项函数
func (m *ConfigManipulator) addOptionFunction(publicName, privateName, fieldType, comment string) {
	functionName := "With" + publicName
	paramName := strings.ToLower(publicName)

	configTypeName := m.getConfigTypeName()
	optionTypeName := strings.TrimSuffix(configTypeName.Name, "Configuration") + "Option"

	function := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: fmt.Sprintf("// %s 创建设置%s的选项", functionName, comment)},
				{Text: "//"},
				{Text: "// 参数:"},
				{Text: fmt.Sprintf("//   - %s: %s", paramName, comment)},
				{Text: "//"},
				{Text: "// 返回:"},
				{Text: fmt.Sprintf("//   - %s: 配置选项", optionTypeName)},
			},
		},
		Name: ast.NewIdent(functionName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent(paramName)},
						Type:  parseTypeExpr(fieldType),
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: ast.NewIdent(optionTypeName),
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.FuncLit{
							Type: &ast.FuncType{
								Params: &ast.FieldList{
									List: []*ast.Field{
										{
											Names: []*ast.Ident{ast.NewIdent("configuration")},
											Type: &ast.StarExpr{
												X: configTypeName,
											},
										},
									},
								},
							},
							Body: &ast.BlockStmt{
								List: []ast.Stmt{
									&ast.ExprStmt{
										X: &ast.CallExpr{
											Fun: &ast.SelectorExpr{
												X:   ast.NewIdent("configuration"),
												Sel: ast.NewIdent("With" + publicName),
											},
											Args: []ast.Expr{
												ast.NewIdent(paramName),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	m.file.Decls = append(m.file.Decls, function)
}

// removeMethod 删除方法
func (m *ConfigManipulator) removeMethod(methodName string) {
	newDecls := []ast.Decl{}
	for _, decl := range m.file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == methodName && funcDecl.Recv != nil {
				continue // 跳过要删除的方法
			}
		}
		newDecls = append(newDecls, decl)
	}
	m.file.Decls = newDecls
}

// removeGlobalFunction 删除全局函数
func (m *ConfigManipulator) removeGlobalFunction(functionName string) {
	newDecls := []ast.Decl{}
	for _, decl := range m.file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == functionName && funcDecl.Recv == nil {
				continue // 跳过要删除的函数
			}
		}
		newDecls = append(newDecls, decl)
	}
	m.file.Decls = newDecls
}

// updateConstructorDefault 更新构造函数默认值
func (m *ConfigManipulator) updateConstructorDefault(fieldName, defaultValue string) {
	constructor := m.findConstructor()
	if constructor == nil {
		return
	}

	// 查找赋值语句
	ast.Inspect(constructor, func(n ast.Node) bool {
		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			if len(assignStmt.Lhs) == 1 {
				if selectorExpr, ok := assignStmt.Lhs[0].(*ast.SelectorExpr); ok {
					if selectorExpr.Sel.Name == fieldName {
						// 更新默认值
						assignStmt.Rhs = []ast.Expr{parseValueExpr(defaultValue)}
						return false
					}
				}
			}
		}
		return true
	})

	// 如果没找到，则添加新的赋值
	m.addConstructorDefault(fieldName, defaultValue)
}

// addConstructorDefault 添加构造函数默认值
func (m *ConfigManipulator) addConstructorDefault(fieldName, defaultValue string) {
	constructor := m.findConstructor()
	if constructor == nil {
		return
	}

	// 查找结构体字面量
	ast.Inspect(constructor, func(n ast.Node) bool {
		if compLit, ok := n.(*ast.CompositeLit); ok {
			// 添加新的字段初始化
			compLit.Elts = append(compLit.Elts, &ast.KeyValueExpr{
				Key:   ast.NewIdent(fieldName),
				Value: parseValueExpr(defaultValue),
			})
			return false
		}
		return true
	})
}

// removeConstructorDefault 删除构造函数默认值
func (m *ConfigManipulator) removeConstructorDefault(fieldName string) {
	constructor := m.findConstructor()
	if constructor == nil {
		return
	}

	ast.Inspect(constructor, func(n ast.Node) bool {
		if compLit, ok := n.(*ast.CompositeLit); ok {
			newElts := []ast.Expr{}
			for _, elt := range compLit.Elts {
				if kvExpr, ok := elt.(*ast.KeyValueExpr); ok {
					if ident, ok := kvExpr.Key.(*ast.Ident); ok {
						if ident.Name == fieldName {
							continue // 跳过要删除的字段
						}
					}
				}
				newElts = append(newElts, elt)
			}
			compLit.Elts = newElts
			return false
		}
		return true
	})
}

// findConstructor 查找构造函数
func (m *ConfigManipulator) findConstructor() *ast.FuncDecl {
	for _, decl := range m.file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if strings.HasPrefix(funcDecl.Name.Name, "New") &&
				strings.HasSuffix(funcDecl.Name.Name, "Configuration") {
				return funcDecl
			}
		}
	}
	return nil
}

// getConfigTypeName 获取配置类型名
func (m *ConfigManipulator) getConfigTypeName() *ast.Ident {
	for _, decl := range m.file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if strings.HasSuffix(typeSpec.Name.Name, "Configuration") {
						return typeSpec.Name
					}
				}
			}
		}
	}
	return ast.NewIdent("Configuration")
}

// parseTypeExpr 解析类型表达式
func parseTypeExpr(typeStr string) ast.Expr {
	switch typeStr {
	case "string":
		return ast.NewIdent("string")
	case "int":
		return ast.NewIdent("int")
	case "int32":
		return ast.NewIdent("int32")
	case "int64":
		return ast.NewIdent("int64")
	case "bool":
		return ast.NewIdent("bool")
	case "float64":
		return ast.NewIdent("float64")
	case "time.Duration":
		return &ast.SelectorExpr{
			X:   ast.NewIdent("time"),
			Sel: ast.NewIdent("Duration"),
		}
	default:
		// 处理复杂类型
		if strings.Contains(typeStr, ".") {
			parts := strings.Split(typeStr, ".")
			if len(parts) == 2 {
				return &ast.SelectorExpr{
					X:   ast.NewIdent(parts[0]),
					Sel: ast.NewIdent(parts[1]),
				}
			}
		}
		return ast.NewIdent(typeStr)
	}
}

// parseValueExpr 解析值表达式
func parseValueExpr(valueStr string) ast.Expr {
	if valueStr == "" {
		return ast.NewIdent("nil")
	}

	// 字符串值
	if strings.HasPrefix(valueStr, `"`) && strings.HasSuffix(valueStr, `"`) {
		return &ast.BasicLit{
			Kind:  token.STRING,
			Value: valueStr,
		}
	}

	// 数字值
	if _, err := strconv.Atoi(valueStr); err == nil {
		return &ast.BasicLit{
			Kind:  token.INT,
			Value: valueStr,
		}
	}

	// 浮点数值
	if _, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return &ast.BasicLit{
			Kind:  token.FLOAT,
			Value: valueStr,
		}
	}

	// 布尔值
	if valueStr == "true" || valueStr == "false" {
		return ast.NewIdent(valueStr)
	}

	// 函数调用或复杂表达式
	return ast.NewIdent(valueStr)
}

// capitalizeFirst 首字母大写
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
