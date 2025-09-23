package middlewares

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func getCapturedBodyFromWriter(t *testing.T, w gin.ResponseWriter) string {
	t.Helper()

	rv := reflect.ValueOf(w)
	require.Equal(t, reflect.Ptr, rv.Kind(), "ctx.Writer should be a pointer")
	rv = rv.Elem()
	require.Equal(t, reflect.Struct, rv.Kind(), "ctx.Writer should point to a struct")

	bodyField := rv.FieldByName("Body")
	require.True(t, bodyField.IsValid(), "interceptor should have Body field")
	require.Equal(t, reflect.Slice, bodyField.Kind(), "Body should be a slice")
	require.Equal(t, reflect.Uint8, bodyField.Type().Elem().Kind(), "Body should be []byte")

	return string(bodyField.Bytes())
}

func TestResponseHashMW_SingleWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(ResponseHashMW())

	r.GET("/one", func(c *gin.Context) {
		c.String(http.StatusOK, "hello")
		c.Header("X-Captured-Body", getCapturedBodyFromWriter(t, c.Writer))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/one", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "hello", w.Body.String(), "response body must still reach the client")
	require.Equal(t, "hello", w.Header().Get("X-Captured-Body"), "interceptor should capture the body contents")
}

func TestResponseHashMW_MultipleWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(ResponseHashMW())

	r.GET("/multi", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = c.Writer.Write([]byte("he"))
		_, _ = c.Writer.Write([]byte("llo"))
		c.Header("X-Captured-Body", getCapturedBodyFromWriter(t, c.Writer))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/multi", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "hello", w.Body.String(), "client should get the combined body")
	require.Equal(t, "hello", w.Header().Get("X-Captured-Body"), "interceptor should capture all writes")
}
