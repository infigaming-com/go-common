package uid

import (
	"github.com/google/uuid"
)

type UUIDV7 struct{}

func NewUUIDV7() *UUIDV7 {
	return &UUIDV7{}
}

func (u *UUIDV7) New() (string, error) {
	uuid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return uuid.String(), nil
}
