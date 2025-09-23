package middlewares

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func okHandler(c *gin.Context) { c.String(http.StatusOK, "ok") }

func buildRouter(keys map[string]any) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessTokenValidationMW(keys))
	r.GET("/ping", okHandler)
	return r
}

func makeHS256Token(t *testing.T, sub string, dur time.Duration, secret []byte) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(dur)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString(secret)
	require.NoError(t, err)
	return s
}

func makeRS256Token(t *testing.T, sub string, dur time.Duration, priv *rsa.PrivateKey) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(dur)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := token.SignedString(priv)
	require.NoError(t, err)
	return s
}

func TestAccessTokenValidationMW_MissingOrMalformedHeader(t *testing.T) {
	keys := map[string]any{"HS256": []byte("secret")}
	r := buildRouter(keys)

	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code, "missing header should be 401")
	}

	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Authorization", "Basic abc.def.ghi")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code, "non-Bearer should be 401")
	}
}

func TestAccessTokenValidationMW_UnsupportedAlg(t *testing.T) {
	keys := map[string]any{"HS256": []byte("secret")}
	r := buildRouter(keys)

	ecdsaPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	claims := jwt.RegisteredClaims{
		Subject:   "alice",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	esTok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	signed, err := esTok.SignedString(ecdsaPriv)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	r.ServeHTTP(w, req)
}

func TestAccessTokenValidationMW_ValidHS256(t *testing.T) {
	secret := []byte("super-secret")
	keys := map[string]any{"HS256": secret}
	r := buildRouter(keys)

	token := makeHS256Token(t, "alice", time.Hour, secret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "valid HS256 token should pass")
}

func TestAccessTokenValidationMW_ValidRS256(t *testing.T) {
	// RSA keypair
	rsaPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rsaPub := &rsaPriv.PublicKey

	keys := map[string]any{"RS256": rsaPub}
	r := buildRouter(keys)

	token := makeRS256Token(t, "bob", time.Hour, rsaPriv)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "valid RS256 token should pass")
	require.Equal(t, "ok", w.Body.String())
}

func TestAccessTokenValidationMW_ExpiredToken(t *testing.T) {
	secret := []byte("super-secret")
	keys := map[string]any{"HS256": secret}
	r := buildRouter(keys)

	// Expired 1 hour ago
	token := makeHS256Token(t, "eve", -1*time.Hour, secret)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code, "expired token should be 401")
}
