package middleware

import (
	"net/http"
	"strings"

	"fileflow/server/config"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// 上下文键
const (
	ContextKeyUser      = "user"
	ContextKeyTokenID   = "token_id"
	ContextKeyAuthType  = "auth_type"
	ContextKeyTokenPerm = "token_permissions"
)

// 认证类型
const (
	AuthTypeJWT   = "jwt"
	AuthTypeToken = "api_token"
)

// Claims JWT Claims
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// AuthMiddleware 认证中间件（支持 JWT 和 API Token）
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 优先尝试 JWT 验证
		if validateJWT(c, tokenString) {
			c.Next()
			return
		}

		// 尝试 API Token 验证
		if validateAPIToken(c, tokenString) {
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证凭证"})
		c.Abort()
	}
}

// JWTOnlyMiddleware 仅允许 JWT 认证（管理员专用）
func JWTOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "需要管理员登录"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if !validateJWT(c, tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的登录凭证"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequirePermission 检查 API Token 权限
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authType := c.GetString(ContextKeyAuthType)

		// JWT 用户拥有全部权限
		if authType == AuthTypeJWT {
			c.Next()
			return
		}

		// 检查 API Token 权限
		permissions, exists := c.Get(ContextKeyTokenPerm)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			c.Abort()
			return
		}

		perms := permissions.([]string)
		for _, p := range perms {
			if p == perm {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足: 需要 " + perm})
		c.Abort()
	}
}

// validateJWT 验证 JWT Token
func validateJWT(c *gin.Context, tokenString string) bool {
	cfg := config.Get()

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return false
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return false
	}

	c.Set(ContextKeyUser, claims.Username)
	c.Set(ContextKeyAuthType, AuthTypeJWT)
	return true
}

// validateAPIToken 验证 API Token
func validateAPIToken(c *gin.Context, tokenValue string) bool {
	token, err := store.ValidateAPIToken(tokenValue)
	if err != nil {
		return false
	}

	c.Set(ContextKeyTokenID, token.ID)
	c.Set(ContextKeyAuthType, AuthTypeToken)
	c.Set(ContextKeyTokenPerm, token.Permissions)
	return true
}

// GenerateJWT 生成 JWT Token
func GenerateJWT(username string) (string, error) {
	cfg := config.Get()

	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "fileflow",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}
