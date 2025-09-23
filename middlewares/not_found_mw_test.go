package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNoRouteMW(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.NoRoute(NoRouteMW())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/some/unknown/path", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "status code must be 404")

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err, "response must be valid JSON")
}
