package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const primaryAdminUserID int64 = 1

// PrimaryAdminOnly reserves full management access for the initial setup admin.
func PrimaryAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "User not found in context")
			return
		}

		if subject.UserID != primaryAdminUserID {
			AbortWithError(c, http.StatusForbidden, "PRIMARY_ADMIN_REQUIRED", "Primary admin access required")
			return
		}

		c.Next()
	}
}
