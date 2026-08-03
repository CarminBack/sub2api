package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type restrictedAdminServiceStub struct {
	*stubAdminService
	lastUpdateUserID    int64
	lastUpdateUserInput *service.UpdateUserInput
	lastDeleteUserID    int64
	lastBalanceUserID   int64
}

func (s *restrictedAdminServiceStub) UpdateUser(_ context.Context, id int64, input *service.UpdateUserInput) (*service.User, error) {
	copyInput := *input
	s.lastUpdateUserID = id
	s.lastUpdateUserInput = &copyInput
	return &service.User{ID: id, Email: "updated@example.com", Role: service.RoleUser, AllowedGroups: []int64{7}}, nil
}

func (s *restrictedAdminServiceStub) DeleteUser(_ context.Context, id int64) error {
	s.lastDeleteUserID = id
	return nil
}

func (s *restrictedAdminServiceStub) UpdateUserBalance(_ context.Context, userID int64, balance float64, operation string, notes string) (*service.User, error) {
	s.lastBalanceUserID = userID
	return &service.User{ID: userID, Role: service.RoleUser, Balance: balance}, nil
}

func newRestrictedAdminUserTestRouter(stub *restrictedAdminServiceStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewUserHandler(stub, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 3})
		c.Next()
	})
	router.GET("/admin/users", handler.List)
	router.GET("/admin/users/:id", handler.GetByID)
	router.POST("/admin/users", handler.Create)
	router.PUT("/admin/users/:id", handler.Update)
	router.DELETE("/admin/users/:id", handler.Delete)
	router.POST("/admin/users/:id/balance", handler.UpdateBalance)
	return router
}

func newRestrictedAdminServiceStub() *restrictedAdminServiceStub {
	base := newStubAdminService()
	base.users = []service.User{
		{ID: 10, Email: "user@example.com", Role: service.RoleUser, AllowedGroups: []int64{7}},
		{ID: 3, Email: "downstream@example.com", Role: service.RoleAdmin},
		{ID: 1, Email: "primary@example.com", Role: service.RoleAdmin},
	}
	return &restrictedAdminServiceStub{stubAdminService: base}
}

func TestRestrictedAdminUserListForcesRegularScope(t *testing.T) {
	stub := newRestrictedAdminServiceStub()
	router := newRestrictedAdminUserTestRouter(stub)
	req := httptest.NewRequest(http.MethodGet, "/admin/users?role=admin&group_name=secret&api_key_group_id=7&attr[1]=internal&include_subscriptions=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.RoleUser, stub.lastListUsers.filters.Role)
	require.Empty(t, stub.lastListUsers.filters.GroupName)
	require.Zero(t, stub.lastListUsers.filters.APIKeyGroupID)
	require.Empty(t, stub.lastListUsers.filters.Attributes)
	require.NotNil(t, stub.lastListUsers.filters.IncludeSubscriptions)
	require.False(t, *stub.lastListUsers.filters.IncludeSubscriptions)
}

func TestRestrictedAdminCannotCreatePrivilegedUser(t *testing.T) {
	for _, payload := range []map[string]any{
		{"email": "admin@example.com", "password": "pass123", "role": "admin"},
		{"email": "group@example.com", "password": "pass123", "allowed_groups": []int64{7}},
	} {
		stub := newRestrictedAdminServiceStub()
		router := newRestrictedAdminUserTestRouter(stub)
		rec := doJSON(t, router, http.MethodPost, "/admin/users", payload)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Nil(t, stub.lastCreateUserInput)
	}
}

func TestRestrictedAdminCanCreateAndUpdateRegularUser(t *testing.T) {
	stub := newRestrictedAdminServiceStub()
	router := newRestrictedAdminUserTestRouter(stub)

	createRec := doJSON(t, router, http.MethodPost, "/admin/users", map[string]any{
		"email": "new@example.com", "password": "pass123", "balance": 25, "concurrency": 2,
	})
	require.Equal(t, http.StatusOK, createRec.Code)
	require.Equal(t, service.RoleUser, stub.lastCreateUserInput.Role)
	require.Equal(t, int64(3), stub.lastCreateUserInput.ActorAdminID)

	updateRec := doJSON(t, router, http.MethodPut, "/admin/users/10", map[string]any{
		"email": "updated@example.com", "status": "disabled", "concurrency": 4,
	})
	require.Equal(t, http.StatusOK, updateRec.Code)
	require.Equal(t, int64(10), stub.lastUpdateUserID)
	require.Equal(t, service.RoleUser, stub.lastUpdateUserInput.Role)
	require.Nil(t, stub.lastUpdateUserInput.AllowedGroups)
}

func TestRestrictedAdminCannotManageAdminTargets(t *testing.T) {
	tests := []struct {
		method  string
		path    string
		payload map[string]any
	}{
		{method: http.MethodGet, path: "/admin/users/1"},
		{method: http.MethodPut, path: "/admin/users/1", payload: map[string]any{"email": "x@example.com"}},
		{method: http.MethodDelete, path: "/admin/users/1"},
		{method: http.MethodPost, path: "/admin/users/1/balance", payload: map[string]any{"balance": 1, "operation": "add"}},
	}
	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			stub := newRestrictedAdminServiceStub()
			router := newRestrictedAdminUserTestRouter(stub)
			var rec *httptest.ResponseRecorder
			if tt.payload == nil {
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(tt.method, tt.path, nil)
				router.ServeHTTP(rec, req)
			} else {
				rec = doJSON(t, router, tt.method, tt.path, tt.payload)
			}
			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Zero(t, stub.lastUpdateUserID)
			require.Zero(t, stub.lastDeleteUserID)
			require.Zero(t, stub.lastBalanceUserID)
		})
	}
}

func TestRestrictedAdminCanDeleteAndAdjustRegularUser(t *testing.T) {
	stub := newRestrictedAdminServiceStub()
	router := newRestrictedAdminUserTestRouter(stub)

	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, httptest.NewRequest(http.MethodDelete, "/admin/users/10", nil))
	require.Equal(t, http.StatusOK, deleteRec.Code)
	require.Equal(t, int64(10), stub.lastDeleteUserID)

	balanceRec := doJSON(t, router, http.MethodPost, "/admin/users/10/balance", map[string]any{
		"balance": 5, "operation": "add", "notes": "downstream adjustment",
	})
	require.Equal(t, http.StatusOK, balanceRec.Code)
	require.Equal(t, int64(10), stub.lastBalanceUserID)
}
