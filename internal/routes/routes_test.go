package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func testRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutesWithStore(r, NewStore())
	return r
}

func TestOpenAPIRoutesRequireAuthentication(t *testing.T) {
	tests := []struct {
		method, path string
		status       int
	}{
		{"POST", "/v1/auth/google", 400},
		{"POST", "/v1/auth/refresh", 401}, {"POST", "/v1/auth/logout", 401}, {"GET", "/v1/auth/me", 401},
		{"GET", "/v1/bikes", 401}, {"GET", "/v1/bikes/00000000-0000-0000-0000-000000000001", 401}, {"GET", "/v1/bikes/00000000-0000-0000-0000-000000000001/status", 401},
		{"POST", "/v1/rides/start", 401}, {"GET", "/v1/rides/current", 401}, {"GET", "/v1/rides/history", 401}, {"GET", "/v1/rides/00000000-0000-0000-0000-000000000001", 401}, {"POST", "/v1/rides/00000000-0000-0000-0000-000000000001/end", 401},
		{"GET", "/v1/wallet/balance", 401}, {"POST", "/v1/wallet/topup", 401}, {"GET", "/v1/wallet/transactions", 401},
		{"POST", "/v1/reports", 401}, {"GET", "/v1/reports/00000000-0000-0000-0000-000000000001", 401},
		{"GET", "/v1/admin/fleet", 401}, {"GET", "/v1/admin/routers", 401}, {"POST", "/v1/admin/bikes/00000000-0000-0000-0000-000000000001/disable", 401}, {"POST", "/v1/admin/bikes/00000000-0000-0000-0000-000000000001/enable", 401}, {"GET", "/v1/admin/reports", 401}, {"POST", "/v1/admin/reports/00000000-0000-0000-0000-000000000001/resolve", 401},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(`{}`))
			w := httptest.NewRecorder()
			testRouter().ServeHTTP(w, req)
			require.Equal(t, tt.status, w.Code)
		})
	}
}

func TestHealthContract(t *testing.T) {
	w := httptest.NewRecorder()
	testRouter().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "healthy", body["status"])
	require.NotNil(t, body["checks"])
}

func TestAuthAndWalletRideFlow(t *testing.T) {
	r := testRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/auth/google", bytes.NewBufferString(`{"code":"student-1"}`)))
	require.Equal(t, 200, w.Code)
	var login struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &login))
	require.NotEmpty(t, login.Token)

	// A newly authenticated user has no active ride and receives the documented error.
	req := httptest.NewRequest(http.MethodGet, "/v1/rides/current", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 404, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/v1/wallet/balance", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
}
