package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggingMW_LogsNormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	r := gin.New()
	r.Use(RequestLoggingMW(logger))

	r.GET("/hello", func(c *gin.Context) {
		c.String(http.StatusOK, "world")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/hello?foo=bar", nil)
	req.Header.Set("User-Agent", "unit-test-agent")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	logs := observedLogs.All()
	require.Len(t, logs, 1, "should have logged once")

	entry := logs[0]
	require.Equal(t, zapcore.InfoLevel, entry.Level)
	require.Contains(t, entry.ContextMap(), "path")
	require.Equal(t, "/hello", entry.ContextMap()["path"])
	require.Equal(t, "GET", entry.ContextMap()["method"])
	require.Equal(t, "foo=bar", entry.ContextMap()["query"])
	require.Contains(t, entry.ContextMap(), "latency")
	require.GreaterOrEqual(t, entry.ContextMap()["latency"].(int64), int64(0))
}

func TestRequestLoggingMW_LogsErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	r := gin.New()
	r.Use(RequestLoggingMW(logger))

	r.GET("/fail", func(c *gin.Context) {
		c.Error(gin.Error{Err: http.ErrHandlerTimeout})
		c.String(http.StatusInternalServerError, "error")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/fail", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	logs := observedLogs.All()
	require.NotEmpty(t, logs, "should have logged at least once")
	require.Equal(t, zapcore.ErrorLevel, logs[0].Level)
	require.Contains(t, logs[0].Message, http.ErrHandlerTimeout.Error())
}
