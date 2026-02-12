package api

import (
	"encoding/json"
	"net/http"

	"github.com/kercylan98/vivid/pkg/console/internal/errors"
)

// Response 标准 HTTP 响应体，所有接口统一使用此结构。
type Response struct {
	Code    int         `json:"code"`    // 0 表示成功，非 0 为业务错误码
	Message string      `json:"message"` // 描述信息
	Data    interface{} `json:"data"`    // 成功时为业务数据，失败时可为 null
}

// WriteJSON 将 Response 序列化为 JSON 并写入 w，设置 Content-Type 与 statusCode。
func WriteJSON(w http.ResponseWriter, statusCode int, res *Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(res)
}

// Ok 写入成功响应（HTTP 200，code=0）。
func Ok(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, &Response{Code: errors.CodeSuccess, Message: "ok", Data: data})
}

// Fail 写入业务错误响应（HTTP 200，code≠0）。
func Fail(w http.ResponseWriter, code int, message string) {
	if code == errors.CodeSuccess {
		code = errors.CodeUnknown
	}
	WriteJSON(w, http.StatusOK, &Response{Code: code, Message: message, Data: nil})
}

// FailServerError 写入服务端错误（HTTP 500）。
func FailServerError(w http.ResponseWriter, message string) {
	WriteJSON(w, http.StatusInternalServerError, &Response{Code: errors.CodeInternal, Message: message, Data: nil})
}

// FailBadRequest 写入请求错误（HTTP 400）。
func FailBadRequest(w http.ResponseWriter, message string) {
	WriteJSON(w, http.StatusBadRequest, &Response{Code: errors.CodeBadRequest, Message: message, Data: nil})
}

// FailServiceUnavailable 写入服务不可用（HTTP 503）。
func FailServiceUnavailable(w http.ResponseWriter, code int, message string) {
	if code == errors.CodeSuccess {
		code = errors.CodeServiceUnavailable
	}
	WriteJSON(w, http.StatusServiceUnavailable, &Response{Code: code, Message: message, Data: nil})
}
