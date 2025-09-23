package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/okieraised/go-common/constants"
)

func TestApiKeyValidationMW(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const expectedKey = "super-secret"
	headerName := constants.HeaderXAPIKey

	type want struct {
		status        int
		nextWasCalled bool
	}

	tests := []struct {
		name      string
		headerVal string
		setupHdr  bool
		want      want
	}{
		{
			name:      "missing header -> 401 and abort",
			setupHdr:  false,
			headerVal: "",
			want: want{
				status:        http.StatusUnauthorized,
				nextWasCalled: false,
			},
		},
		{
			name:      "wrong key -> 401 and abort",
			setupHdr:  true,
			headerVal: "nope",
			want: want{
				status:        http.StatusUnauthorized,
				nextWasCalled: false,
			},
		},
		{
			name:      "correct key -> 200 and next called",
			setupHdr:  true,
			headerVal: expectedKey,
			want: want{
				status:        http.StatusOK,
				nextWasCalled: true,
			},
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()

			downstreamCalled := false

			r.Use(ApiKeyValidationMW(expectedKey))
			r.GET("/ping", func(c *gin.Context) {
				downstreamCalled = true
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			if tc.setupHdr {
				req.Header.Set(headerName, tc.headerVal)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.want.status {
				t.Fatalf("status = %d, want %d; body=%q", w.Code, tc.want.status, w.Body.String())
			}
			if downstreamCalled != tc.want.nextWasCalled {
				t.Fatalf("downstream called = %v, want %v; status=%d body=%q",
					downstreamCalled, tc.want.nextWasCalled, w.Code, w.Body.String())
			}
		})
	}
}
