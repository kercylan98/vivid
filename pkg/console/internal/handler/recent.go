package handler

import (
	"net/http"

	"github.com/kercylan98/vivid/pkg/console/internal/api"
	"github.com/kercylan98/vivid/pkg/console/internal/store"
)

// Recent 提供最近事件与死信队列的 HTTP 只读接口。
type Recent struct {
	Store *store.RecentStore
}

// NewRecent 创建 Recent 处理器，Store 可为 nil（返回空列表）。
func NewRecent(s *store.RecentStore) *Recent {
	return &Recent{Store: s}
}

// RecentDeathLetters 返回最近 5 条死信，GET /api/nodes/current/death-letters。
func (r *Recent) RecentDeathLetters(w http.ResponseWriter, _ *http.Request) {
	if r.Store == nil {
		api.Ok(w, []store.DeathLetterItem(nil))
		return
	}
	api.Ok(w, r.Store.GetDeathLetters())
}

// RecentEvents 返回最近 5 条事件，GET /api/nodes/current/events。
func (r *Recent) RecentEvents(w http.ResponseWriter, _ *http.Request) {
	if r.Store == nil {
		api.Ok(w, []store.EventItem(nil))
		return
	}
	api.Ok(w, r.Store.GetEvents())
}
