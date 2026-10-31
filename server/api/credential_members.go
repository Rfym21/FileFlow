package api

import (
	"fileflow/server/middleware"
	"fileflow/server/store"
	"github.com/gin-gonic/gin"
	"net/http"
)

// credentialMember validates the member and prevents creating a credential above their role.
func credentialMember(c *gin.Context, userID string, permissions []string) (*store.User, bool) {
	if userID == "" {
		userID = middleware.CurrentUser(c).ID
	}
	user, err := store.GetUserByID(userID)
	if err != nil || !user.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择启用的团队成员"})
		return nil, false
	}
	if !canManageCredentialUser(c, user.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权管理其他管理员或所有者的凭证"})
		return nil, false
	}
	allowed := store.PermissionsForRole(user.Role)
	for _, permission := range permissions {
		found := false
		for _, grant := range allowed {
			if permission == grant && permission != "manage" {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{"error": "凭证权限不能超过所属成员的权限"})
			return nil, false
		}
	}
	return user, true
}

func canManageCredentialUser(c *gin.Context, userID string) bool {
	actor := middleware.CurrentUser(c)
	if actor == nil {
		return false
	}
	if actor.Role == "owner" || actor.ID == userID {
		return true
	}
	user, err := store.GetUserByID(userID)
	return err == nil && (user.Role == "member" || user.Role == "viewer")
}
