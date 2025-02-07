package errors

import "fmt"

type Error struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Details []any  `json:"details"`
}

func NewError(code int64, message string, details ...any) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: details,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

func (e *Error) GetCode() int64 {
	return e.Code
}

func (e *Error) GetMessage() string {
	return e.Message
}
