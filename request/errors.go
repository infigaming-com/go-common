package request

import (
	"github.com/infigaming-com/go-common/errors"
)

type RequestError struct {
	baseErr      errors.Error
	Method       string `json:"method,omitempty"`
	URL          string `json:"url,omitempty"`
	RequestBody  []byte `json:"request_body,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	ResponseBody []byte `json:"response_body,omitempty"`
}

const (
	ErrCodeInvalidRequestBody = 10000 + iota
	ErrCodeInvalidSlowRequestThreshold
	ErrCodeFailedToCreateRequest
	ErrCodeFailedToSignRequest
	ErrCodeFailedToSendRequest
	ErrCodeRequestTimeout
	ErrCodeFailedToReadResponseBody
)

type requestErrorOption func(*RequestError)

func withMethod(method string) requestErrorOption {
	return func(e *RequestError) {
		e.Method = method
	}
}

func withURL(url string) requestErrorOption {
	return func(e *RequestError) {
		e.URL = url
	}
}

func withRequestBody(requestBody []byte) requestErrorOption {
	return func(e *RequestError) {
		e.RequestBody = requestBody
	}
}

func withStatusCode(statusCode int) requestErrorOption {
	return func(e *RequestError) {
		e.StatusCode = statusCode
	}
}

func withResponseBody(responseBody []byte) requestErrorOption {
	return func(e *RequestError) {
		e.ResponseBody = responseBody
	}
}

func NewRequestError(code int64, message string, cause error, details any, options ...requestErrorOption) *RequestError {
	e := &RequestError{
		baseErr: errors.NewError(code, message, cause, details),
	}
	for _, opt := range options {
		opt(e)
	}
	return e
}

func (e *RequestError) Error() string {
	return e.baseErr.Error()
}

func (e *RequestError) GetCode() int64 {
	return e.baseErr.GetCode()
}

func (e *RequestError) GetMessage() string {
	return e.baseErr.GetMessage()
}

func (e *RequestError) Unwrap() error {
	return e.baseErr.Unwrap()
}

func (e *RequestError) GetCause() error {
	return e.baseErr.Cause
}

func (e *RequestError) GetDetails() any {
	return e.baseErr.GetDetails()
}

func (e *RequestError) GetMethod() string {
	return e.Method
}

func (e *RequestError) GetURL() string {
	return e.URL
}

func (e *RequestError) GetStatusCode() int {
	return e.StatusCode
}

func (e *RequestError) GetResponseBody() []byte {
	return e.ResponseBody
}
