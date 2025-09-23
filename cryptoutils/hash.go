package cryptoutils

import (
	"golang.org/x/crypto/bcrypt"
)

// Hashing hashes the input string using default cost
func Hashing(password string) (string, error) {
	bPassword := []byte(password)
	hashedPassword, err := bcrypt.GenerateFromPassword(bPassword, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil

}

// CompareHash compares input plaintext with its possible hash value
func CompareHash(hash, input string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input))
	return err == nil
}
