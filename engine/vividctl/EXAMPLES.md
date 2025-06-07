# VividCTL 使用示例

这个文档提供了 `vividctl` 工具的详细使用示例。

## 基础示例

### 1. 创建数据库配置

```bash
# 初始化配置文件
vividctl config init -p database -n Database -o database_config.go

# 添加基础字段
vividctl config add -f database_config.go -n Host -t string -c "数据库主机地址" -d '"localhost"'
vividctl config add -f database_config.go -n Port -t int -c "数据库端口" -d "5432"
vividctl config add -f database_config.go -n Username -t string -c "用户名" -d '"postgres"'
vividctl config add -f database_config.go -n Password -t string -c "密码"

# 添加高级配置
vividctl config add -f database_config.go -n MaxConnections -t int -c "最大连接数" -d "100"
vividctl config add -f database_config.go -n Timeout -t time.Duration -c "连接超时时间" -d "time.Second*30"
vividctl config add -f database_config.go -n EnableSSL -t bool -c "是否启用SSL" -d "false"
```

生成的代码使用方式：

```go
package main

import (
    "time"
    "your-project/database"
)

func main() {
    // 使用默认配置
    config := database.NewDatabaseConfiguration()

    // 使用选项模式
    config = database.NewDatabaseConfiguration(
        database.WithHost("production-db.example.com"),
        database.WithPort(5432),
        database.WithUsername("app_user"),
        database.WithPassword("secret"),
        database.WithMaxConnections(50),
    )

    // 使用链式调用
    config = database.NewDatabaseConfiguration().
        WithHost("staging-db.example.com").
        WithMaxConnections(50).
        WithTimeout(time.Second * 10)

    // 获取配置值
    host := config.GetHost()
    port := config.GetPort()
    maxConn := config.GetMaxConnections()
}
```

### 2. 创建 HTTP 服务器配置

```bash
# 初始化配置文件
vividctl config init -p server -n HTTPServer -o http_server_config.go

# 添加基础服务器配置
vividctl config add -f http_server_config.go -n Addr -t string -c "服务器监听地址" -d '":8080"'
vividctl config add -f http_server_config.go -n ReadTimeout -t time.Duration -c "读取超时时间" -d "time.Second*30"
vividctl config add -f http_server_config.go -n WriteTimeout -t time.Duration -c "写入超时时间" -d "time.Second*30"
vividctl config add -f http_server_config.go -n MaxHeaderBytes -t int -c "最大请求头字节数" -d "1048576"

# 添加 TLS 配置
vividctl config add -f http_server_config.go -n EnableTLS -t bool -c "是否启用TLS" -d "false"
vividctl config add -f http_server_config.go -n CertFile -t string -c "TLS证书文件路径"
vividctl config add -f http_server_config.go -n KeyFile -t string -c "TLS私钥文件路径"

# 添加其他配置
vividctl config add -f http_server_config.go -n EnableCORS -t bool -c "是否启用CORS" -d "true"
vividctl config add -f http_server_config.go -n MaxRequestSize -t int64 -c "最大请求体大小" -d "33554432"
```

### 3. 创建缓存配置

```bash
# 初始化配置文件
vividctl config init -p cache -n Redis -o redis_config.go

# 添加 Redis 连接配置
vividctl config add -f redis_config.go -n Addr -t string -c "Redis服务器地址" -d '"localhost:6379"'
vividctl config add -f redis_config.go -n Password -t string -c "Redis密码"
vividctl config add -f redis_config.go -n DB -t int -c "Redis数据库索引" -d "0"

# 添加连接池配置
vividctl config add -f redis_config.go -n PoolSize -t int -c "连接池大小" -d "10"
vividctl config add -f redis_config.go -n MinIdleConns -t int -c "最小空闲连接数" -d "5"
vividctl config add -f redis_config.go -n MaxRetries -t int -c "最大重试次数" -d "3"

# 添加其他配置
vividctl config add -f redis_config.go -n TTL -t time.Duration -c "默认过期时间" -d "time.Hour*24"
vividctl config add -f redis_config.go -n EnableCluster -t bool -c "是否启用集群模式" -d "false"
```

## 高级示例

### 4. 自定义类型配置

```bash
# 初始化配置文件
vividctl config init -p processor -n Registry -o registry_config.go

# 添加自定义类型字段
vividctl config add -f registry_config.go -n RootUnitIdentifier -t UnitIdentifier -c "顶级单元标识符" -d 'newUnitIdentifier("localhost", "/")'
vividctl config add -f registry_config.go -n Logger -t log.Logger -c "日志记录器"
vividctl config add -f registry_config.go -n Context -t context.Context -c "上下文" -d "context.Background()"
```

### 5. 复杂配置结构

```bash
# 创建监控配置
vividctl config init -p monitoring -n Monitoring -o monitoring_config.go

vividctl config add -f monitoring_config.go -n EnableMetrics -t bool -c "是否启用指标收集" -d "true"
vividctl config add -f monitoring_config.go -n MetricsInterval -t time.Duration -c "指标收集间隔" -d "time.Second*10"
vividctl config add -f monitoring_config.go -n MetricsPort -t int -c "指标服务端口" -d "9090"
vividctl config add -f monitoring_config.go -n HealthCheckPath -t string -c "健康检查路径" -d '"/health"'

# 创建日志配置
vividctl config init -p logging -n Logging -o logging_config.go

vividctl config add -f logging_config.go -n Level -t string -c "日志级别" -d '"info"'
vividctl config add -f logging_config.go -n Format -t string -c "日志格式" -d '"json"'
vividctl config add -f logging_config.go -n OutputPath -t string -c "日志输出路径" -d '"stdout"'
vividctl config add -f logging_config.go -n MaxSize -t int -c "单个日志文件最大大小(MB)" -d "100"
vividctl config add -f logging_config.go -n MaxBackups -t int -c "保留的旧日志文件数量" -d "3"
vividctl config add -f logging_config.go -n MaxAge -t int -c "保留旧日志文件的最大天数" -d "28"
```

## 配置管理工作流

### 6. 完整的开发工作流

```bash
# Step 1: 初始化项目配置
vividctl config init -p myproject -n Application -o app_config.go

# Step 2: 添加基础配置
vividctl config add -f app_config.go -n Name -t string -c "应用程序名称" -d '"myapp"'
vividctl config add -f app_config.go -n Version -t string -c "应用程序版本" -d '"1.0.0"'
vividctl config add -f app_config.go -n Environment -t string -c "运行环境" -d '"development"'

# Step 3: 添加服务器配置
vividctl config add -f app_config.go -n ServerAddr -t string -c "服务器地址" -d '":8080"'
vividctl config add -f app_config.go -n ServerTimeout -t time.Duration -c "服务器超时时间" -d "time.Second*30"

# Step 4: 添加数据库配置
vividctl config add -f app_config.go -n DatabaseURL -t string -c "数据库连接URL"
vividctl config add -f app_config.go -n DatabaseMaxConns -t int -c "数据库最大连接数" -d "100"

# Step 5: 如果需要修改，删除某个字段
vividctl config del -f app_config.go -n ServerTimeout

# Step 6: 重新添加修改后的字段
vividctl config add -f app_config.go -n ServerTimeout -t time.Duration -c "服务器超时时间" -d "time.Minute*2"
```

### 7. 配置的使用模式

生成的配置文件可以通过多种方式使用：

```go
// app_config.go 使用示例
package main

import (
    "myproject"
    "time"
)

func main() {
    // 方式1: 默认配置
    config := myproject.NewApplicationConfiguration()

    // 方式2: 选项模式 - 适合少量配置
    config = myproject.NewApplicationConfiguration(
        myproject.WithName("production-app"),
        myproject.WithEnvironment("production"),
        myproject.WithServerAddr(":80"),
    )

    // 方式3: 链式调用 - 适合大量配置
    config = myproject.NewApplicationConfiguration().
        WithName("staging-app").
        WithEnvironment("staging").
        WithServerAddr(":8080").
        WithDatabaseMaxConns(50).
        WithServerTimeout(time.Minute * 5)

    // 方式4: 混合使用
    config = myproject.NewApplicationConfiguration(
        myproject.WithEnvironment("production"),
    ).WithServerAddr(":443").
        WithDatabaseMaxConns(200)

    // 获取配置值
    name := config.GetName()
    env := config.GetEnvironment()
    serverAddr := config.GetServerAddr()

    // 在实际应用中使用
    startServer(config)
}

func startServer(config *myproject.ApplicationConfiguration) {
    server := &http.Server{
        Addr:         config.GetServerAddr(),
        ReadTimeout:  config.GetServerTimeout(),
        WriteTimeout: config.GetServerTimeout(),
    }

    log.Printf("Starting %s v%s in %s mode on %s",
        config.GetName(),
        config.GetVersion(),
        config.GetEnvironment(),
        config.GetServerAddr())

    server.ListenAndServe()
}
```

## 最佳实践示例

### 8. 推荐的项目结构

```
project/
├── internal/
│   ├── config/
│   │   ├── app_config.go          # 应用主配置
│   │   ├── database_config.go     # 数据库配置
│   │   ├── cache_config.go        # 缓存配置
│   │   └── logging_config.go      # 日志配置
│   └── ...
└── cmd/
    └── main.go
```

### 9. 配置验证扩展

```go
// 可以在生成的配置基础上添加验证方法
func (c *DatabaseConfiguration) Validate() error {
    if c.GetHost() == "" {
        return errors.New("database host is required")
    }
    
    if c.GetPort() <= 0 || c.GetPort() > 65535 {
        return errors.New("database port must be between 1 and 65535")
    }
    
    if c.GetMaxConnections() <= 0 {
        return errors.New("max connections must be positive")
    }
    
    return nil
}
```

这些示例展示了 `vividctl` 工具的强大功能和灵活性。通过这些模式，你可以快速生成标准化、类型安全的配置代码，大大提高开发效率。 