package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMW_Panic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RecoveryMW())

	r.GET("/fail", func(c *gin.Context) {
		panic("kaboom")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/fail", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, "should return 500")

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "response must be valid JSON")

}

func TestRecoveryMW_NoPanicPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RecoveryMW())

	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "should pass through")
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, true, body["ok"])
}
