package core

import (
	"golang.org/x/crypto/bcrypt"
)

type PasswordEncoder interface {
	EncodePassword(raw string, salt *string) (string, error)
	IsPasswordValid(encoded string, raw string) (bool, error)
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

func (e *passwordEncoder) IsPasswordValid(encoded string, raw string) (bool, error) {
	rawPassBytes := []byte(raw)
	bcryptPass := []byte(encoded)
	err := bcrypt.CompareHashAndPassword(bcryptPass, rawPassBytes)
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
