package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func testAdminAuthBySubject(c *gin.Context) {
	userID := int64(3)
	if c.GetHeader("X-Test-Primary") == "true" {
		userID = 1
	}
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: userID})
	c.Set(string(servermiddleware.ContextKeyUserRole), service.RoleAdmin)
	c.Next()
}

func TestRestrictedAdminCannotAccessUserManagementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}}
	adminAuth := servermiddleware.AdminAuthMiddleware(testAdminAuthBySubject)
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, auditLog, stepUp, nil, nil)

	for _, path := range []string{
		"/api/v1/admin/users",
		"/api/v1/admin/users/10",
		"/api/v1/admin/usage/search-users?q=user",
		"/api/v1/admin/usage/search-api-keys?q=key",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusForbidden, recorder.Code, path)
	}
}

func TestRestrictedAdminCannotAccessAdministrativePaymentRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	jwtAuth := servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() })
	adminAuth := servermiddleware.AdminAuthMiddleware(testAdminAuthBySubject)
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	RegisterPaymentRoutes(router.Group("/api/v1"), nil, nil, (*adminhandler.PaymentHandler)(nil), jwtAuth, adminAuth, auditLog, nil, nil)

	for _, path := range []string{"/api/v1/admin/payment/orders", "/api/v1/admin/payment/dashboard"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusForbidden, recorder.Code, path)
	}
}
