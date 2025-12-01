package httputils

import (
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/okieraised/go-common/constants"
)

// Canon returns the canonical MIME header key (e.g., "content-type" -> "Content-Type").
func Canon(k string) string {
	return textproto.CanonicalMIMEHeaderKey(k)
}

// ContentTypeByExt returns a content-type for a filename extension like ".png" or "png".
// It uses mime.TypeByExtension and falls back to a few common types. Empty string if unknown.
func ContentTypeByExt(extOrName string) string {
	ext := extOrName
	if !strings.HasPrefix(ext, ".") {
		ext = filepath.Ext(extOrName)
	}
	ext = strings.ToLower(ext)
	if ext == "" {
		return ""
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	switch ext {
	case ".json":
		return constants.ContentTypeJSON
	case ".jsonl", ".ndjson":
		return constants.ContentTypeNDJSON
	case ".svg":
		return constants.ContentTypeSVG
	case ".wasm":
		return constants.ContentTypeWASM
	case ".mjs", ".js":
		return constants.ContentTypeJS
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".txt", ".log":
		return constants.ContentTypeTextUTF8
	case ".yaml", ".yml":
		return "application/yaml"
	case ".tar":
		return constants.ContentTypeTar
	case ".gz":
		return constants.ContentTypeGZip
	case ".7z":
		return constants.ContentTypeSevenZip
	}
	return ""
}

// DetectContentTypeFromFile tries to determine a content-type from a file name,
// optionally sniffing up to the first 512 bytes if provided.
// If no good guess is available, returns application/octet-stream.
func DetectContentTypeFromFile(fileName string, first512 []byte) string {
	if ct := ContentTypeByExt(fileName); ct != "" {
		return ct
	}
	if len(first512) > 0 {
		if ct := http.DetectContentType(first512); ct != "" && ct != "application/octet-stream" {
			return ct
		}
	}
	return constants.ContentTypeOctetStream
}

func DetectContentTypeFromHeader(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer func(file multipart.File) {
		cErr := file.Close()
		if cErr != nil && err == nil {
			err = cErr
		}
	}(file)

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}

	contentType := http.DetectContentType(buffer)
	return contentType, nil
}

// Disposition builds a Content-Disposition header value.
// If inline==true => "inline; filename*=utf-8”<url-escaped>"; else "attachment; ...".
func Disposition(fileName string, inline bool) string {
	dispositionType := "attachment"
	if inline {
		dispositionType = "inline"
	}
	escaped := url.PathEscape(fileName) // RFC 5987
	return dispositionType + `; filename="` + fileName + `"; filename*=utf-8''` + escaped
}

func DetectMIMEType(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer func(file multipart.File) {
		cErr := file.Close()
		if cErr != nil && err == nil {
			err = cErr
		}
	}(file)

	// Read the first 512 bytes for content detection
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}

	// Detect content type
	contentType := http.DetectContentType(buffer)
	return contentType, nil
}
