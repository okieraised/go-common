package utilities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	in := "Việt.txt"

	out := SanitizeFilename(in)
	assert.Equal(t, in, out)
}
