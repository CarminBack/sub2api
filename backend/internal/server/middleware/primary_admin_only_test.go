//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPrimaryAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		subject    *AuthSubject
		wantStatus int
	}{
		{name: "primary admin allowed", subject: &AuthSubject{UserID: 1}, wantStatus: http.StatusNoContent},
		{name: "secondary admin denied", subject: &AuthSubject{UserID: 2}, wantStatus: http.StatusForbidden},
		{name: "missing subject denied", wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			if tt.subject != nil {
				router.Use(func(c *gin.Context) {
					c.Set(string(ContextKeyUser), *tt.subject)
					c.Next()
				})
			}
			router.Use(PrimaryAdminOnly())
			router.GET("/admin", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
		})
	}
}
