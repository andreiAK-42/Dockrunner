package security

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string, cost int) (string, error) {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func ComparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
