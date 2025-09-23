package utilities

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func SanitizeFilename(name string) string {
	name = filepath.Base(name)
	if !utf8.ValidString(name) {
		name = strings.ToValidUTF8(name, "")
	}
	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" {
		name = "file"
	}
	return name
}
