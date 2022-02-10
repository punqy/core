package core

import (
	"golang.org/x/crypto/bcrypt"
)

type PasswordEncoder interface {
	EncodePassword(raw string, salt *string) (string, error)
	IsPasswordValid(encoded string, raw string) error
}

type passwordEncoder struct {
}

func NewPasswordEncoder() PasswordEncoder {
	return &passwordEncoder{}
}

func (e *passwordEncoder) EncodePassword(raw string, salt *string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), err
}

func (e *passwordEncoder) IsPasswordValid(encoded string, raw string) error {
	rawPassBytes := []byte(raw)
	bcryptPass := []byte(encoded)
	return bcrypt.CompareHashAndPassword(bcryptPass, rawPassBytes)
}
