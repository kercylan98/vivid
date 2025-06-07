package gui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// WebServer Web GUI 服务器
type WebServer struct {
	mux *http.ServeMux
	app *App
}

// NewWebServer 创建新的 Web 服务器
func NewWebServer(app *App) *WebServer {
	ws := &WebServer{
		mux: http.NewServeMux(),
		app: app,
	}

	ws.setupRoutes()
	return ws
}

// ServeHTTP 实现 http.Handler 接口
func (ws *WebServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 添加 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	ws.mux.ServeHTTP(w, r)
}

// setupRoutes 设置路由
func (ws *WebServer) setupRoutes() {
	// 基础路由
	ws.mux.HandleFunc("/", ws.handleIndex)
	ws.mux.HandleFunc("/api/modules", ws.handleModulesAPI)

	// 注册各模块的路由
	for _, module := range ws.app.GetModules() {
		if module.Enabled() {
			module.RegisterWebRoutes(ws.mux)
		}
	}
}

// handleIndex 处理首页
func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// 生成模块菜单
	var moduleMenus []string

	modules := ws.app.GetModules() // 现在返回有序的slice
	for _, module := range modules {
		status := "disabled"
		if module.Enabled() {
			status = "enabled"
		}

		moduleMenus = append(moduleMenus, fmt.Sprintf(
			`<div class="module %s" data-module-id="%s" onclick="loadModule('%s')">%s %s</div>`,
			status, module.ID(), module.ID(), module.Icon(), module.Name(),
		))
	}

	// 生成JavaScript switch语句内容
	var switchCases []string
	for _, module := range modules {
		if module.Enabled() {
			// 处理模块内容，转义HTML特殊字符
			content := module.GetWebContent()
			content = strings.ReplaceAll(content, "\\", "\\\\")
			content = strings.ReplaceAll(content, "'", "\\'")
			content = strings.ReplaceAll(content, "\n", "\\n")
			content = strings.ReplaceAll(content, "\r", "")

			switchCases = append(switchCases, fmt.Sprintf(
				"case '%s': content.innerHTML = '%s'; break;",
				module.ID(), content,
			))
		} else {
			switchCases = append(switchCases, fmt.Sprintf(
				"case '%s': content.innerHTML = '<h2>%s %s</h2><p class=\"text-muted\">此功能暂未启用</p>'; break;",
				module.ID(), module.Icon(), module.Name(),
			))
		}
	}

	// 完整的switch语句
	switchStatement := fmt.Sprintf(`switch(moduleId) {
		%s
		default:
			content.innerHTML = '<h2>❌ 模块未找到</h2><p>请求的模块 "<strong>' + moduleId + '</strong>" 不存在或未启用。</p>';
			debug('未知模块: ' + moduleId);
	}`, strings.Join(switchCases, "\n\t\t"))

	// HTML 模板
	htmlTemplate := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>VividCTL - Web 管理界面</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Microsoft YaHei', sans-serif; 
            background: #f8f9fa;
            line-height: 1.6;
        }
        .header { 
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); 
            color: white; 
            padding: 1.5rem; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .container { display: flex; height: calc(100vh - 120px); }
        .sidebar { 
            width: 280px; 
            background: white; 
            padding: 1.5rem; 
            box-shadow: 2px 0 10px rgba(0,0,0,0.1);
            overflow-y: auto;
        }
        .main { flex: 1; padding: 1.5rem; overflow-y: auto; }
        .module { 
            padding: 1rem; 
            margin: 0.5rem 0; 
            cursor: pointer; 
            border-radius: 8px; 
            transition: all 0.3s ease;
            border: 1px solid #e9ecef;
            user-select: none;
        }
        .module:hover:not(.disabled) { 
            background: #f8f9fa; 
            transform: translateX(4px); 
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .module.active { 
            background: #007bff !important; 
            color: white !important; 
            box-shadow: 0 4px 12px rgba(0,123,255,0.3);
            transform: translateX(4px);
        }
        .module.disabled { 
            opacity: 0.5; 
            cursor: not-allowed; 
        }
        .module.disabled:hover { 
            background: none !important; 
            transform: none !important; 
            box-shadow: none !important;
        }
        .content { 
            background: white; 
            border-radius: 12px; 
            padding: 2rem; 
            min-height: 500px; 
            box-shadow: 0 4px 20px rgba(0,0,0,0.1);
        }
        .btn { 
            padding: 0.5rem 1rem; 
            border: none; 
            border-radius: 6px; 
            cursor: pointer; 
            margin: 0.25rem;
            transition: all 0.3s ease;
            font-size: 14px;
        }
        .btn-primary { background: #007bff; color: white; }
        .btn-primary:hover { background: #0056b3; transform: translateY(-1px); box-shadow: 0 2px 4px rgba(0,0,0,0.2); }
        .btn-secondary { background: #6c757d; color: white; }
        .btn-secondary:hover { background: #545b62; transform: translateY(-1px); box-shadow: 0 2px 4px rgba(0,0,0,0.2); }
        .text-muted { color: #6c757d; }
        .module-content { margin-top: 1rem; }
        .action-buttons { margin: 1rem 0; }
        .settings-group { margin: 1rem 0; padding: 1rem; background: #f8f9fa; border-radius: 6px; }
        .settings-group h4 { margin-bottom: 0.5rem; color: #495057; }
        .settings-group label { display: block; margin: 0.5rem 0; cursor: pointer; }
        .debug-info { 
            position: fixed; 
            top: 10px; 
            right: 10px; 
            background: rgba(0,0,0,0.8); 
            color: white; 
            padding: 10px; 
            border-radius: 4px; 
            font-family: monospace; 
            font-size: 12px;
            display: none;
            z-index: 1000;
        }
    </style>
</head>
<body>
    <div class="debug-info" id="debug-info">调试信息</div>
    
    <div class="header">
        <h1>🎮 VividCTL Web 管理界面</h1>
        <p>Vivid 框架开发工具集 - 模块化可视化管理平台</p>
    </div>
    
    <div class="container">
        <div class="sidebar">
            <h3 style="margin-bottom: 1rem; color: #495057;">功能模块</h3>
            %s
        </div>
        
        <div class="main">
            <div class="content" id="content">
                <h2>🎮 欢迎使用 VividCTL</h2>
                <p>这是一个模块化的开发工具集，提供与命令行工具相同的功能。</p>
                
                <div style="margin-top: 2rem;">
                    <h3>✨ 系统特性</h3>
                    <ul style="margin-left: 2rem; margin-top: 1rem; line-height: 1.6;">
                        <li>🏗️ <strong>模块化架构</strong> - 插件式功能扩展</li>
                        <li>🎨 <strong>现代化界面</strong> - 响应式设计，美观易用</li>
                        <li>⚡ <strong>实时同步</strong> - 与命令行工具功能一致</li>
                        <li>🔧 <strong>可扩展性</strong> - 轻松添加新功能模块</li>
                    </ul>
                </div>
                
                <div style="margin-top: 2rem;">
                    <h3>📋 可用模块</h3>
                    <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-top: 1rem;">
                        %s
                    </div>
                </div>
                
                <div style="margin-top: 2rem; padding: 1rem; background: #e9ecef; border-radius: 8px;">
                    <p><strong>提示：</strong>点击左侧模块开始使用，或按 F12 打开开发者工具查看调试信息</p>
                </div>
            </div>
        </div>
    </div>
    
    <script>
        // 调试函数
        function debug(message) {
            console.log('[VividCTL]', message);
            const debugElement = document.getElementById('debug-info');
            if (debugElement) {
                debugElement.textContent = new Date().toLocaleTimeString() + ': ' + message;
                debugElement.style.display = 'block';
                setTimeout(function() {
                    debugElement.style.display = 'none';
                }, 3000);
            }
        }
        
        // 加载模块函数
        function loadModule(moduleId) {
            debug('尝试加载模块: ' + moduleId);
            
            // 获取当前点击的元素
            const clickedElement = document.querySelector('[data-module-id="' + moduleId + '"]');
            
            if (!clickedElement) {
                debug('错误: 找不到模块元素');
                return;
            }
            
            // 检查模块是否被禁用
            if (clickedElement.classList.contains('disabled')) {
                debug('模块已禁用: ' + moduleId);
                return;
            }
            
            // 更新侧边栏状态
            document.querySelectorAll('.module').forEach(function(m) {
                m.classList.remove('active');
            });
            clickedElement.classList.add('active');
            
            debug('模块状态已更新');
            
            // 加载模块内容
            const content = document.getElementById('content');
            
            if (!content) {
                debug('错误: 找不到内容容器');
                return;
            }
            
            try {
                loadModuleContent(moduleId, content);
                debug('模块内容已加载: ' + moduleId);
            } catch (error) {
                debug('加载模块时出错: ' + error.message);
                content.innerHTML = '<h2>❌ 加载错误</h2><p>模块加载时发生错误: ' + error.message + '</p>';
            }
        }
        
        // 模块内容加载函数
        function loadModuleContent(moduleId, content) {
            %s
        }
        
        // 页面初始化
        document.addEventListener('DOMContentLoaded', function() {
            debug('页面初始化开始');
            
            const firstEnabledModule = document.querySelector('.module.enabled');
            if (firstEnabledModule) {
                const moduleId = firstEnabledModule.getAttribute('data-module-id');
                debug('找到第一个可用模块: ' + moduleId);
                
                setTimeout(function() {
                    loadModule(moduleId);
                }, 100);
            } else {
                debug('没有找到可用的模块');
            }
        });
        
        // 键盘快捷键
        document.addEventListener('keydown', function(e) {
            if (e.key === 'F12') {
                e.preventDefault();
                const debugElement = document.getElementById('debug-info');
                debugElement.style.display = debugElement.style.display === 'none' ? 'block' : 'none';
            }
        });
        
        debug('VividCTL Web GUI 已初始化');
    </script>
</body>
</html>`

	html := fmt.Sprintf(htmlTemplate,
		strings.Join(moduleMenus, "\n"),
		ws.generateModuleCards(),
		switchStatement,
	)

	// 设置正确的字符编码
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	w.Write([]byte(html))
}

// generateModuleCards 生成模块卡片
func (ws *WebServer) generateModuleCards() string {
	var cards []string

	modules := ws.app.GetModules() // 使用有序的模块列表
	for _, module := range modules {
		status := "🔴 未启用"
		statusClass := "disabled"
		if module.Enabled() {
			status = "🟢 已启用"
			statusClass = "enabled"
		}

		statusColor := "#dc3545"
		if module.Enabled() {
			statusColor = "#28a745"
		}

		card := fmt.Sprintf(`
			<div class="module-card %s" style="padding: 1rem; background: #f8f9fa; border-radius: 8px; text-align: center;">
				<div style="font-size: 2rem; margin-bottom: 0.5rem;">%s</div>
				<h4 style="margin: 0.5rem 0;">%s</h4>
				<p style="font-size: 0.9rem; color: #6c757d; margin: 0.5rem 0;">%s</p>
				<small style="color: %s;">%s</small>
			</div>`,
			statusClass, module.Icon(), module.Name(), module.Description(), statusColor, status,
		)
		cards = append(cards, card)
	}

	return strings.Join(cards, "\n")
}

// handleModulesAPI 处理模块相关 API
func (ws *WebServer) handleModulesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var modules []map[string]interface{}

	orderedModules := ws.app.GetModules() // 使用有序的模块列表
	for _, module := range orderedModules {
		modules = append(modules, map[string]interface{}{
			"id":          module.ID(),
			"name":        module.Name(),
			"description": module.Description(),
			"icon":        module.Icon(),
			"enabled":     module.Enabled(),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    modules,
	})
}
