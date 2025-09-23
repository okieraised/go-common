package httputils

import (
	"mime/multipart"
	"net/http"
)

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
