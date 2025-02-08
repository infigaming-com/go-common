package errors

type Error struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Cause   error  // the underlying error
	Details any    `json:"details,omitempty"`
}

func NewError(code int64, message string, cause error, details any) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
		Details: details,
	}
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) GetCode() int64 {
	return e.Code
}

func (e *Error) GetMessage() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) GetDetails() any {
	return e.Details
}
