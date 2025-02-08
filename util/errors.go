package util

import "github.com/infigaming-com/go-common/errors"

type UtilError struct {
	baseErr *errors.Error
}

const (
	ErrCodeValueNotFoundInContext = 10000 + iota
	ErrCodeInvalidValueInContext
)

func NewUtilError(code int64, message string, cause error, details any) *UtilError {
	return &UtilError{
		baseErr: errors.NewError(code, message, cause, details),
	}
}

func (e *UtilError) Error() string {
	return e.baseErr.Error()
}

func (e *UtilError) GetCode() int64 {
	return e.baseErr.GetCode()
}

func (e *UtilError) GetMessage() string {
	return e.baseErr.GetMessage()
}

func (e *UtilError) Unwrap() error {
	return e.baseErr.Unwrap()
}

func (e *UtilError) GetDetails() any {
	return e.baseErr.GetDetails()
}
