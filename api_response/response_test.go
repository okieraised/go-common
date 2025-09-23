package api_response

import (
	"testing"
)

func TestResponse_ToResponse(t *testing.T) {
	resp := New[any](t.Context())
	resp.Populate("", "", map[string]interface{}{
		"key": "val",
	}, map[string]interface{}{
		"key": "val",
	}, 10)
}
