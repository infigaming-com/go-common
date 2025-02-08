package errors

type Error struct {
	Code       int64  `json:"code"`
	Message    string `json:"message"`
	Cause      error  // the underlying error
	Details    any    `json:"details,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
}

func NewError(code int64, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func (e *Error) WithDetails(details any) *Error {
	e.Details = details
	return e
}

func (e *Error) WithStatusCode(statusCode int) *Error {
	e.StatusCode = statusCode
	return e
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

func (e *Error) GetStatusCode() int {
	return e.StatusCode
}
