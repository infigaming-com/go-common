package request

import "github.com/infigaming-com/go-common/errors"

const (
	ErrCodeInvalidRequestBody = 10000 + iota
	ErrCodeInvalidSlowRequestThreshold
	ErrCodeFailedToCreateRequest
	ErrCodeFailedToSignRequest
	ErrCodeFailedToSendRequest
	ErrCodeFailedToReadResponseBody
)

var (
	ErrFailedToUnmarshalRequestBody = errors.NewError(ErrCodeInvalidRequestBody, "failed to unmarshal request body")
	ErrInvalidSlowRequestThreshold  = errors.NewError(ErrCodeInvalidSlowRequestThreshold, "invalid slow request threshold")
)
