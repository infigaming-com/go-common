package cache

import "errors"

var (
	ErrKeyNotFound   = errors.New("key not found")
	ErrJsonMarshal   = errors.New("failed to marshal value to json")
	ErrJsonUnmarshal = errors.New("failed to unmarshal value from json")
	ErrNotSupport    = errors.New("not supported")
)
