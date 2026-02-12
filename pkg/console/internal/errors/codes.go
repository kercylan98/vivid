package errors

// 业务错误码：与标准响应 code 字段对应，0 为成功，非 0 为各类错误。
const (
	CodeSuccess int = 0

	CodeUnknown          = -1
	CodeBadRequest       = 400
	CodeUnauthorized     = 401
	CodeForbidden        = 403
	CodeNotFound         = 404
	CodeServiceUnavailable = 503
	CodeInternal         = 500
)
