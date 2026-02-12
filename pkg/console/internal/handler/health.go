package handler

import (
	"net/http"

	"github.com/kercylan98/vivid/pkg/console/internal/api"
)

// Health 返回 health 资源，GET /api/health，始终返回标准成功格式。
func Health(w http.ResponseWriter, _ *http.Request) {
	api.Ok(w, map[string]string{"status": "up"})
}
