// Package main implements automatic documentation generator for Vivid ecology components.
//
// This tool scans ecology components and generates navigation documentation
// based on component metadata found in Go source files.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// ComponentConfig represents the structure of component.yaml
type ComponentConfig struct {
	Component struct {
		Name        string `yaml:"name"`
		Version     string `yaml:"version"`
		Status      string `yaml:"status"`
		Category    string `yaml:"category"`
		Description string `yaml:"description"`
	} `yaml:"component"`
	Author struct {
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
	} `yaml:"author"`
	Features []string `yaml:"features"`
}

// ComponentInfo represents metadata about an ecology component
type ComponentInfo struct {
	Name         string
	Path         string
	Version      string
	Status       string
	Category     string
	Description  string
	Dependencies []string
	HasReadme    bool
	HasExamples  bool
	Features     []string
}

// StatusIcon returns the emoji icon for component status
func (c ComponentInfo) StatusIcon() string {
	switch c.Status {
	case "stable":
		return "✅ 可用"
	case "beta":
		return "🚧 测试中"
	case "alpha":
		return "🚧 开发中"
	case "development":
		return "🚧 开发中"
	case "planned":
		return "📋 计划中"
	default:
		return "❓ 未知"
	}
}

// ReadmeLink returns the README link if available
func (c ComponentInfo) ReadmeLink() string {
	if c.HasReadme {
		return fmt.Sprintf("[README](./%s/README.md)", c.Name)
	}
	return "-"
}

var (
	target = flag.String("target", "ecology", "Target directory to scan")
	output = flag.String("output", "README.md", "Output file name")
)

func main() {
	flag.Parse()

	components, err := scanComponents(*target)
	if err != nil {
		log.Fatalf("Failed to scan components: %v", err)
	}

	// Handle output path - if it's absolute, use as-is; if relative, join with target
	outputPath := *output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(*target, outputPath)
	}

	if err := generateDoc(components, outputPath); err != nil {
		log.Fatalf("Failed to generate documentation: %v", err)
	}

	fmt.Printf("Generated documentation for %d components to %s\n", len(components), outputPath)
}

// scanComponents scans the target directory for ecology components
func scanComponents(targetDir string) ([]ComponentInfo, error) {
	var components []ComponentInfo

	err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == targetDir {
			return nil
		}

		// Only process component.yaml files
		if d.IsDir() || !strings.HasSuffix(path, "component.yaml") {
			return nil
		}

		component, err := parseComponentConfig(path)
		if err != nil {
			log.Printf("Warning: failed to parse component config %s: %v", path, err)
			return nil
		}

		if component != nil {
			components = append(components, *component)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", targetDir, err)
	}

	// Sort components by name
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	return components, nil
}

// parseComponentConfig parses a component.yaml file
func parseComponentConfig(filePath string) (*ComponentInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var config ComponentConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	// Validate required fields
	if config.Component.Name == "" {
		return nil, fmt.Errorf("component name is required in %s", filePath)
	}

	componentDir := filepath.Dir(filePath)
	component := &ComponentInfo{
		Name:        config.Component.Name,
		Version:     config.Component.Version,
		Status:      config.Component.Status,
		Category:    config.Component.Category,
		Description: config.Component.Description,
		Path:        componentDir,
		Features:    config.Features,
	}

	// Check for README.md
	if _, err := os.Stat(filepath.Join(componentDir, "README.md")); err == nil {
		component.HasReadme = true
	}

	// Check for examples directory
	if _, err := os.Stat(filepath.Join(componentDir, "examples")); err == nil {
		component.HasExamples = true
	}

	// Set defaults
	if component.Status == "" {
		component.Status = "planned"
	}
	if component.Category == "" {
		component.Category = "general"
	}
	if component.Version == "" {
		component.Version = "v0.1.0"
	}
	if component.Description == "" {
		component.Description = fmt.Sprintf("%s 组件", component.Name)
	}

	return component, nil
}

// generateDoc generates the README.md file from component information
func generateDoc(components []ComponentInfo, outputPath string) error {
	tmpl := `# Vivid Ecology

> 可选扩展组件目录 - 按需引入，保持核心轻量

<!-- 此文档由 go generate 自动生成，请勿手动编辑 -->
<!-- go:generate go run ./tools/docgen --target=ecology --output=README.md -->

## 组件导航

| 组件 | 状态 | 描述 | 文档 |
|------|------|------|------|
{{- range .}}
| [{{.Name}}](./{{.Name}}/) | {{.StatusIcon}} | {{.Description}} | {{.ReadmeLink}} |
{{- end}}

## 快速安装

` + "```bash\n# 安装特定组件\ngo get github.com/kercylan98/vivid/ecology/grpc-server\n\n# 查看组件详细信息\ncd ecology/grpc-server && cat README.md\n```" + `

## 组件规范

每个组件必须包含：
- ` + "`component.yaml`" + ` - 组件配置和元数据
- ` + "`go.mod`" + ` - 独立模块定义
- ` + "`README.md`" + ` - 组件文档
- ` + "`component.go`" + ` - 主要实现
- ` + "`examples/`" + ` - 使用示例

### 组件配置格式

` + "```yaml\ncomponent:\n  name: grpc-server\n  version: v1.0.0\n  status: stable\n  category: network\n  description: High-performance gRPC server integration\n\nauthor:\n  name: Vivid Team\n  email: team@vivid.dev\n\ndependencies:\n  go:\n    - google.golang.org/grpc\n    - github.com/kercylan98/vivid/core/vivid\n\nfeatures:\n  - Actor-based gRPC service handling\n  - Automatic lifecycle management\n```" + `
`

	t, err := template.New("readme").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	if err := t.Execute(file, components); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}