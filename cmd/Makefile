.PHONY: build clean install test example help

# 默认目标
all: build

# 构建工具
build:
	@echo "🔨 构建 vividctl..."
	@go build -o vividctl
	@echo "✅ 构建完成: vividctl"

# Windows 构建
build-windows:
	@echo "🔨 构建 vividctl (Windows)..."
	@go build -o vividctl.exe
	@echo "✅ 构建完成: vividctl.exe"

# 清理生成的文件
clean:
	@echo "🧹 清理文件..."
	@rm -f vividctl vividctl.exe
	@rm -f test_*.go example_*.go
	@echo "✅ 清理完成"

# 安装到系统路径
install: build
	@echo "📦 安装工具到系统路径..."
	@cp vividctl $(GOPATH)/bin/vividctl
	@echo "✅ 安装完成，可以在任意位置使用 'vividctl' 命令"

# 运行测试
test: build
	@echo "🧪 运行测试..."
	@go test ./...

# 运行示例
example: build
	@echo "🚀 运行配置生成示例..."
	@echo "生成数据库配置示例..."
	@./vividctl config init -p example -n Database -o example_database_config.go
	@./vividctl config add -f example_database_config.go -n Host -t string -c "数据库主机地址" -d '"localhost"'
	@./vividctl config add -f example_database_config.go -n Port -t int -c "数据库端口" -d "5432"
	@./vividctl config add -f example_database_config.go -n Username -t string -c "用户名" -d '"postgres"'
	@./vividctl config add -f example_database_config.go -n MaxConnections -t int -c "最大连接数" -d "100"
	@./vividctl config add -f example_database_config.go -n Timeout -t time.Duration -c "连接超时时间" -d "time.Second*30"
	@echo ""
	@echo "生成HTTP服务器配置示例..."
	@./vividctl config init -p example -n HTTPServer -o example_http_config.go
	@./vividctl config add -f example_http_config.go -n Addr -t string -c "服务器监听地址" -d '":8080"'
	@./vividctl config add -f example_http_config.go -n ReadTimeout -t time.Duration -c "读取超时时间" -d "time.Second*30"
	@./vividctl config add -f example_http_config.go -n EnableTLS -t bool -c "是否启用TLS" -d "false"
	@echo ""
	@echo "✅ 示例完成！查看生成的文件："
	@ls -la example_*.go

# 显示版本信息
version: build
	@echo "vividctl version: $(shell git describe --tags --always --dirty 2>/dev/null || echo 'dev')"
	@echo "Go version: $(shell go version)"

# 格式化代码
fmt:
	@echo "🎨 格式化代码..."
	@go fmt ./...
	@echo "✅ 代码格式化完成"

# 静态检查
lint:
	@echo "🔍 运行静态检查..."
	@go vet ./...
	@echo "✅ 静态检查完成"

# 依赖管理
deps:
	@echo "📦 更新依赖..."
	@go mod tidy
	@go mod download
	@echo "✅ 依赖更新完成"

# 快速测试配置命令
test-config: build
	@echo "🧪 快速测试配置命令..."
	@./vividctl config init -p test -n TestConfig -o test_config.go
	@./vividctl config add -f test_config.go -n TestField -t string -c "测试字段" -d '"test"'
	@echo "✅ 测试完成，查看生成的文件: test_config.go"

# 帮助信息
help:
	@echo "VividCTL 构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  build         构建 vividctl"
	@echo "  build-windows 构建 Windows 版本"
	@echo "  clean         清理生成的文件"
	@echo "  install       安装到系统路径"
	@echo "  test          运行测试"
	@echo "  example       运行示例"
	@echo "  test-config   快速测试配置命令"
	@echo "  version       显示版本信息"
	@echo "  fmt           格式化代码"
	@echo "  lint          静态检查"
	@echo "  deps          更新依赖"
	@echo "  help          显示此帮助信息"
	@echo ""
	@echo "使用示例:"
	@echo "  make build                    # 构建工具"
	@echo "  make example                  # 运行示例"
	@echo "  make test-config              # 快速测试"
	@echo "  ./vividctl config init        # 初始化配置"
	@echo "  ./vividctl config add         # 添加字段"
	@echo "  ./vividctl config del         # 删除字段" 