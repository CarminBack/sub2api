package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const primaryAdminUserID int64 = 1

// IsPrimaryAdmin reports whether the authenticated subject is the initial setup admin.
func IsPrimaryAdmin(c *gin.Context) bool {
	subject, ok := GetAuthSubjectFromContext(c)
	return ok && subject.UserID == primaryAdminUserID
}

// IsRestrictedAdmin reports whether the authenticated admin is outside the primary scope.
// Admin-only routes call this after AdminAuth, so a missing subject is not treated as restricted.
func IsRestrictedAdmin(c *gin.Context) bool {
	subject, ok := GetAuthSubjectFromContext(c)
	return ok && subject.UserID != primaryAdminUserID
}

// PrimaryAdminOnly reserves full management access for the initial setup admin.
func PrimaryAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := GetAuthSubjectFromContext(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "User not found in context")
			return
		}

		if !IsPrimaryAdmin(c) {
			AbortWithError(c, http.StatusForbidden, "PRIMARY_ADMIN_REQUIRED", "Primary admin access required")
			return
		}

		c.Next()
	}
}
