package util

import (
	"crypto/hmac"
	"crypto/sha256"
)

func HmacSha256Hash(message []byte, secret []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write(message)
	return h.Sum(nil)
}
