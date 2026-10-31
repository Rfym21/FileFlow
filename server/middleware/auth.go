package middleware

import (
	"fileflow/server/config"
	"fileflow/server/store"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

const (
	ContextKeyUser      = "user"
	ContextKeyUserID    = "user_id"
	ContextKeyProfile   = "user_profile"
	ContextKeyTokenID   = "token_id"
	ContextKeyAuthType  = "auth_type"
	ContextKeyTokenPerm = "token_permissions"
	AuthTypeJWT         = "jwt"
	AuthTypeToken       = "api_token"
)

type Claims struct {
	Username    string `json:"username"`
	AuthVersion int    `json:"authVersion"`
	jwt.RegisteredClaims
}

func AuthMiddleware() gin.HandlerFunc { return authenticate(false) }

// JWTOnlyMiddleware requires an interactive member session, not an administrator role.
func JWTOnlyMiddleware() gin.HandlerFunc { return authenticate(true) }
func authenticate(jwtOnly bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if strings.HasPrefix(header, "Bearer ") {
			value := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			if validateJWT(c, value) || (!jwtOnly && validateAPIToken(c, value)) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已失效，请重新登录"})
	}
}
func CurrentUser(c *gin.Context) *store.User {
	profile, exists := c.Get(ContextKeyProfile)
	if !exists {
		return nil
	}
	user, _ := profile.(*store.User)
	return user
}
func Permissions(c *gin.Context) []string {
	value, _ := c.Get(ContextKeyTokenPerm)
	permissions, _ := value.([]string)
	return permissions
}
func HasPermission(c *gin.Context, permission string) bool {
	for _, granted := range Permissions(c) {
		if granted == permission {
			return true
		}
	}
	return false
}
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !HasPermission(c, permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足: 需要 " + permission})
			return
		}
		c.Next()
	}
}
func setUserContext(c *gin.Context, user *store.User, authType string, permissions []string) {
	c.Set(ContextKeyUser, user.Username)
	c.Set(ContextKeyUserID, user.ID)
	c.Set(ContextKeyProfile, user)
	c.Set(ContextKeyAuthType, authType)
	c.Set(ContextKeyTokenPerm, permissions)
}
func validateJWT(c *gin.Context, value string) bool {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Get().JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("fileflow"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.Subject == "" || claims.IssuedAt == nil {
		return false
	}
	user, err := store.GetUserByID(claims.Subject)
	if err != nil || !user.IsActive || user.AuthVersion != claims.AuthVersion {
		return false
	}
	setUserContext(c, user, AuthTypeJWT, store.PermissionsForRole(user.Role))
	return true
}

// EffectivePermissions intersects each credential with its member's current role.
func EffectivePermissions(role string, requested []string) []string {
	allowed := store.PermissionsForRole(role)
	result := make([]string, 0, len(requested))
	for _, permission := range requested {
		if permission == "manage" {
			continue
		}
		for _, grant := range allowed {
			if permission == grant {
				result = append(result, permission)
				break
			}
		}
	}
	return result
}
func validateAPIToken(c *gin.Context, value string) bool {
	token, err := store.ValidateAPIToken(value)
	if err != nil || token.UserID == "" {
		return false
	}
	user, err := store.GetUserByID(token.UserID)
	if err != nil || !user.IsActive {
		return false
	}
	setUserContext(c, user, AuthTypeToken, EffectivePermissions(user.Role, token.Permissions))
	c.Set(ContextKeyTokenID, token.ID)
	return true
}
func GenerateJWT(username string) (string, error) {
	user, err := store.GetUserByUsername(username)
	if err != nil {
		return "", err
	}
	return GenerateJWTForUser(user)
}
func GenerateJWTForUser(user *store.User) (string, error) {
	now := time.Now()
	claims := &Claims{
		Username: user.Username, AuthVersion: user.AuthVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "fileflow", Subject: user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.Get().JWTSecret))
}
