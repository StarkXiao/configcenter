package domain

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeInvalid          ErrorCode = "INVALID_ARGUMENT"
	CodeNotFound         ErrorCode = "NOT_FOUND"
	CodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	CodeForbidden        ErrorCode = "FORBIDDEN"
	CodeConflict         ErrorCode = "CONFLICT"
	CodeRevisionConflict ErrorCode = "REVISION_CONFLICT"
	CodeVersionConflict  ErrorCode = "VERSION_CONFLICT"
	CodeNotPublished     ErrorCode = "CONFIG_NOT_PUBLISHED"
	CodeTooLarge         ErrorCode = "CONFIG_TOO_LARGE"
	CodeInternal         ErrorCode = "INTERNAL_ERROR"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }

func NewError(code ErrorCode, message string) error { return &Error{Code: code, Message: message} }

func WrapError(code ErrorCode, message string, cause error) error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func ErrorCodeOf(err error) ErrorCode {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return CodeInternal
}

func ErrorMessage(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Message
	}
	return "internal server error"
}
