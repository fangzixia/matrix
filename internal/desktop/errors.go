package desktop

import (
	"context"
	"errors"
	"fmt"

	"matrix/internal/query"
)

const (
	ErrorValidation         = "validation_error"
	ErrorNotConfigured      = "not_configured"
	ErrorWorkspace          = "workspace_error"
	ErrorModel              = "model_error"
	ErrorCancelled          = "cancelled"
	ErrorTaskFailed         = "task_failed"
	ErrorDiagnosticDegraded = "diagnostic_degraded"
	ErrorInternal           = "internal_error"
)

// ErrorInfo is the stable error DTO exposed to the frontend.
type ErrorInfo struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
	Retryable bool   `json:"retryable"`
}

// AppError preserves an internal cause while exposing a stable frontend code.
type AppError struct {
	Info  ErrorInfo
	Cause error
}

func NewAppError(code, message string, retryable bool, cause error) *AppError {
	return &AppError{
		Info: ErrorInfo{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
		Cause: cause,
	}
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Info.Message != "" {
		return e.Info.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Info.Code
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func errorInfo(err error) *ErrorInfo {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		info := appErr.Info
		if info.Message == "" && appErr.Cause != nil {
			info.Message = appErr.Cause.Error()
		}
		if info.Details == "" && appErr.Cause != nil {
			info.Details = appErr.Cause.Error()
		}
		return &info
	}
	return &ErrorInfo{Code: ErrorInternal, Message: err.Error()}
}

func runErrorInfo(r query.Result) *ErrorInfo {
	if r.Err == nil && r.StopReason == query.StopCompleted {
		return nil
	}
	switch {
	case errors.Is(r.Err, context.Canceled), r.StopReason == query.StopAborted:
		return &ErrorInfo{Code: ErrorCancelled, Message: "任务已取消", Retryable: true}
	case r.StopReason == query.StopModelError:
		msg := "模型调用失败"
		if r.Err != nil {
			msg = r.Err.Error()
		}
		return &ErrorInfo{Code: ErrorModel, Message: msg, Retryable: true}
	case r.StopReason == query.StopMaxTurns:
		return &ErrorInfo{Code: ErrorTaskFailed, Message: "任务达到最大轮次后停止", Retryable: true}
	case r.Err != nil:
		return &ErrorInfo{Code: ErrorTaskFailed, Message: r.Err.Error(), Retryable: true}
	default:
		return nil
	}
}

func wrapInternal(message string, err error) error {
	if err == nil {
		return nil
	}
	return NewAppError(ErrorInternal, message, false, fmt.Errorf("%s: %w", message, err))
}
