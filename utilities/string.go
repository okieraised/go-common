package utilities

import (
	"errors"
	"strings"

	"golang.org/x/text/secure/precis"
)

func NormalizeUsername(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", errors.New("username is empty after trimming")
	}

	normalized, err := precis.UsernameCaseMapped.String(s)
	if err != nil {
		return "", err
	}

	if normalized == "" {
		return "", errors.New("username is empty after normalization")
	}

	return normalized, nil
}
