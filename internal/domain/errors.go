package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound         = errors.New("记录不存在")
	ErrConflict         = errors.New("revision 冲突")
	ErrForbidden        = errors.New("当前岗位无权执行该操作")
	ErrInvalidState     = errors.New("当前状态不允许该操作")
	ErrValidation       = errors.New("输入校验失败")
	ErrAlreadyReleased  = errors.New("已放行方案不可修改")
	ErrEvidenceRequired = errors.New("风险项缺少有效证据")
	ErrReviewIncomplete = errors.New("风险项尚未全部通过复核")
	ErrDigestMismatch   = errors.New("内容摘要不匹配")
	ErrDuplicateRequest = errors.New("请求标识已用于其他命令")
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return ErrValidation.Error()
	}
	return fmt.Sprintf("%s: %s %s", ErrValidation, e.Fields[0].Field, e.Fields[0].Message)
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

func NewValidation(field, message string) error {
	return &ValidationError{Fields: []FieldError{{Field: field, Message: message}}}
}

func CombineValidation(fields ...FieldError) error {
	if len(fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: fields}
}
