// Package bizerr centralizes error codes and their display messages.
package bizerr

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
)

// Code identifies a business error category.
type Code string

const (
	CodeBadRequest           Code = "E_BAD_REQUEST"
	CodeNameRequired         Code = "E_NAME_REQUIRED"
	CodePasswordTooShort     Code = "E_PASSWORD_TOO_SHORT"
	CodePasswordUnchanged    Code = "E_PASSWORD_UNCHANGED"
	CodeCurrentPasswordWrong Code = "E_CURRENT_PASSWORD_WRONG"
	CodeUnauthorized         Code = "E_UNAUTHORIZED"
	CodeInvalidCredentials   Code = "E_INVALID_CREDENTIALS"
	CodeForbidden            Code = "E_FORBIDDEN"
	CodeNotFound             Code = "E_NOT_FOUND"
	CodeConflict             Code = "E_CONFLICT"
	CodeTooManyAttempts      Code = "E_TOO_MANY_ATTEMPTS"
	CodeInternal             Code = "E_INTERNAL"
	CodeStoreUnavailable     Code = "E_STORE_UNAVAILABLE"
)

var messages = map[Code]string{
	CodeBadRequest:           "请求格式不正确",
	CodeNameRequired:         "请填写名称",
	CodePasswordTooShort:     "密码至少 8 位",
	CodePasswordUnchanged:    "新口令不能与当前口令相同",
	CodeCurrentPasswordWrong: "当前口令不正确",
	CodeUnauthorized:         "登录已失效，请重新登录",
	CodeInvalidCredentials:   "账号或口令错误",
	CodeForbidden:            "无权执行此操作",
	CodeNotFound:             "内容不存在或无权访问",
	CodeConflict:             "操作与当前状态冲突",
	CodeTooManyAttempts:      "尝试次数过多，请 %d 分钟后再试",
	CodeInternal:             "系统出错了，请稍后重试",
	CodeStoreUnavailable:     "系统暂时不可用，请稍后重试",
}

// StatusOf maps a code to the HTTP status used by the API surface.
func StatusOf(code Code) int {
	switch code {
	case CodeUnauthorized, CodeInvalidCredentials:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeTooManyAttempts:
		return http.StatusTooManyRequests
	case CodeInternal:
		return http.StatusInternalServerError
	case CodeStoreUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

// All returns every known Code in ascending order.
func All() []Code {
	out := make([]Code, 0, len(messages))
	for code := range messages {
		out = append(out, code)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Error is an error with a stable Code and a rendered Message.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap enables errors.Is and errors.As against the wrapped cause.
func (e *Error) Unwrap() error { return e.Cause }

// New builds an Error for code, formatting its message template with args.
func New(code Code, args ...any) *Error {
	return build(code, nil, args...)
}

// Wrap builds an Error for code around cause.
func Wrap(code Code, cause error, args ...any) *Error {
	return build(code, cause, args...)
}

func build(code Code, cause error, args ...any) *Error {
	tmpl, ok := messages[code]
	if !ok {
		tmpl = string(code)
	}
	msg := tmpl
	if len(args) > 0 {
		msg = fmt.Sprintf(tmpl, args...)
	}
	return &Error{Code: code, Message: msg, Cause: cause}
}

// As extracts a *Error from err's chain.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// MessageOf returns the message template for code, or empty when unknown.
func MessageOf(code Code) string {
	return messages[code]
}
