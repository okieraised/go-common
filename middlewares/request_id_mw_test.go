package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/okieraised/go-common/constants"
	"github.com/stretchr/testify/require"
)

func TestRequestIDMW_SetsRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestIDMW())

	var capturedID string

	r.GET("/ping", func(c *gin.Context) {
		id := c.Request.Header.Get(constants.APIFieldRequestID)
		require.NotEmpty(t, id, "request ID header must be set")
		require.True(t, uuid.Validate(id) == nil, "request ID must be a valid UUID")

		capturedID = id

		val, exists := c.Get(constants.APIFieldRequestID)
		require.True(t, exists, "request ID must be stored in context")
		require.Equal(t, id, val)

		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "handler must still run")

	require.Contains(t, w.Body.String(), capturedID)
}
