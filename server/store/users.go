package store

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

var (
	ErrUserNotFound       = errors.New("成员不存在")
	ErrInvalidUser        = errors.New("成员参数无效")
	ErrUsernameExists     = errors.New("用户名已存在")
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrOwnerProtected     = errors.New("不能停用所有者或修改所有者角色")
	usernamePattern       = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{2,63}$`)
)

// User is a persisted identity. API responses must omit PasswordHash and AuthVersion.
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	PasswordHash string `json:"passwordHash"`
	Role         string `json:"role"`
	IsActive     bool   `json:"isActive"`
	AuthVersion  int    `json:"authVersion"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type UserUpdate struct {
	DisplayName string
	Role        string
	IsActive    bool
	Password    string
}

func PermissionsForRole(role string) []string {
	switch role {
	case RoleOwner, RoleAdmin:
		return []string{"read", "write", "delete", "manage"}
	case RoleMember:
		return []string{"read", "write", "delete"}
	case RoleViewer:
		return []string{"read"}
	default:
		return []string{}
	}
}

func (u *User) HasPermission(permission string) bool {
	if !u.IsActive {
		return false
	}
	for _, p := range PermissionsForRole(u.Role) {
		if p == permission {
			return true
		}
	}
	return false
}

func GetUsers() []User {
	dataLock.RLock()
	defer dataLock.RUnlock()
	if data == nil {
		return []User{}
	}
	return append([]User{}, data.Users...)
}

func userIndexByID(id string) int {
	if data != nil {
		for i := range data.Users {
			if data.Users[i].ID == id {
				return i
			}
		}
	}
	return -1
}

func GetUserByID(id string) (*User, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()
	if i := userIndexByID(id); i >= 0 {
		result := data.Users[i]
		return &result, nil
	}
	return nil, ErrUserNotFound
}

func GetUserByUsername(username string) (*User, error) {
	dataLock.RLock()
	defer dataLock.RUnlock()
	if data != nil {
		for _, user := range data.Users {
			if strings.EqualFold(user.Username, strings.TrimSpace(username)) {
				return &user, nil
			}
		}
	}
	return nil, ErrUserNotFound
}

func AuthenticateUser(username, password string) (*User, error) {
	user, err := GetUserByUsername(username)
	if err != nil || !user.IsActive {
		return nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

func validatePassword(password string) error {
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("%w: 密码长度需为 8 至 72 字节", ErrInvalidUser)
	}
	return nil
}

// BootstrapOwner migrates the original administrator once without rewriting existing file data.
func BootstrapOwner(username, password string) error {
	dataLock.Lock()
	defer dataLock.Unlock()
	if data == nil {
		return fmt.Errorf("%w: 存储尚未初始化", ErrInvalidUser)
	}
	previousUsers := append([]User(nil), data.Users...)
	previousTokens := append([]Token(nil), data.Tokens...)
	previousCredentials := append([]WebDAVCredential(nil), data.WebDAVCredentials...)
	ownerID := ""
	changed := false
	if len(data.Users) == 0 {
		if strings.TrimSpace(username) == "" || password == "" || len(password) > 72 {
			return fmt.Errorf("%w: 初始管理员用户名或密码无效", ErrInvalidUser)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("初始化所有者失败: %w", err)
		}
		now := NowString()
		user := User{ID: uuid.New().String(), Username: strings.TrimSpace(username), DisplayName: strings.TrimSpace(username), PasswordHash: string(hash), Role: RoleOwner, IsActive: true, AuthVersion: 1, CreatedAt: now, UpdatedAt: now}
		data.Users = []User{user}
		ownerID = user.ID
		changed = true
	} else {
		for _, user := range data.Users {
			if user.Role == RoleOwner && user.IsActive {
				ownerID = user.ID
				break
			}
		}
		if ownerID == "" {
			return fmt.Errorf("%w: 缺少有效的团队所有者", ErrInvalidUser)
		}
	}
	for i := range data.Tokens {
		if data.Tokens[i].UserID == "" {
			data.Tokens[i].UserID = ownerID
			changed = true
		}
	}
	for i := range data.WebDAVCredentials {
		if data.WebDAVCredentials[i].UserID == "" {
			data.WebDAVCredentials[i].UserID = ownerID
			changed = true
		}
	}
	if changed {
		if err := save(); err != nil {
			data.Users = previousUsers
			data.Tokens = previousTokens
			data.WebDAVCredentials = previousCredentials
			return err
		}
	}
	return nil
}

func CreateUser(user *User, password string) error {
	if user == nil {
		return ErrInvalidUser
	}
	username := strings.TrimSpace(user.Username)
	if !usernamePattern.MatchString(username) || len(strings.TrimSpace(user.DisplayName)) > 160 {
		return fmt.Errorf("%w: 用户名需为 3 至 64 位字母、数字或 ._-", ErrInvalidUser)
	}
	if user.Role != RoleAdmin && user.Role != RoleMember && user.Role != RoleViewer {
		return fmt.Errorf("%w: 无效的成员角色", ErrInvalidUser)
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码处理失败: %w", err)
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	if data == nil {
		return ErrInvalidUser
	}
	for _, existing := range data.Users {
		if strings.EqualFold(existing.Username, username) {
			return ErrUsernameExists
		}
	}
	result := *user
	result.ID = uuid.New().String()
	result.Username = username
	result.DisplayName = strings.TrimSpace(result.DisplayName)
	if result.DisplayName == "" {
		result.DisplayName = username
	}
	result.PasswordHash = string(hash)
	result.AuthVersion = 1
	result.CreatedAt = NowString()
	result.UpdatedAt = result.CreatedAt
	previous := data.Users
	data.Users = append(append([]User(nil), data.Users...), result)
	if err := save(); err != nil {
		data.Users = previous
		return err
	}
	*user = result
	return nil
}

func UpdateUser(id string, updates UserUpdate) error {
	if updates.Role != RoleOwner && updates.Role != RoleAdmin && updates.Role != RoleMember && updates.Role != RoleViewer {
		return fmt.Errorf("%w: 无效的成员角色", ErrInvalidUser)
	}
	if len(strings.TrimSpace(updates.DisplayName)) > 160 {
		return fmt.Errorf("%w: 显示名称过长", ErrInvalidUser)
	}
	passwordHash := ""
	if updates.Password != "" {
		if err := validatePassword(updates.Password); err != nil {
			return err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(updates.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("密码处理失败: %w", err)
		}
		passwordHash = string(hash)
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	i := userIndexByID(id)
	if i < 0 {
		return ErrUserNotFound
	}
	previous := data.Users[i]
	if previous.Role == RoleOwner && (updates.Role != RoleOwner || !updates.IsActive) {
		return ErrOwnerProtected
	}
	if previous.Role != RoleOwner && updates.Role == RoleOwner {
		return ErrOwnerProtected
	}
	updated := previous
	updated.DisplayName = strings.TrimSpace(updates.DisplayName)
	if updated.DisplayName == "" {
		updated.DisplayName = previous.Username
	}
	updated.Role, updated.IsActive = updates.Role, updates.IsActive
	if passwordHash != "" {
		updated.PasswordHash = passwordHash
	}
	if previous.Role != updated.Role || previous.IsActive != updated.IsActive || passwordHash != "" {
		updated.AuthVersion++
	}
	updated.UpdatedAt = NowString()
	data.Users[i] = updated
	if err := save(); err != nil {
		data.Users[i] = previous
		return err
	}
	return nil
}

func ChangeUserPassword(id, currentPassword, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	user, err := GetUserByID(id)
	if err != nil {
		return err
	}
	if !user.IsActive || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码处理失败: %w", err)
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	i := userIndexByID(id)
	if i < 0 {
		return ErrUserNotFound
	}
	previous := data.Users[i]
	// Recheck after hashing so a concurrent password reset or disable cannot be overwritten.
	if !previous.IsActive || previous.PasswordHash != user.PasswordHash {
		return ErrInvalidCredentials
	}
	data.Users[i].PasswordHash = string(hash)
	data.Users[i].AuthVersion++
	data.Users[i].UpdatedAt = NowString()
	if err := save(); err != nil {
		data.Users[i] = previous
		return err
	}
	return nil
}

func RevokeUserSessions(id string) error {
	dataLock.Lock()
	defer dataLock.Unlock()
	i := userIndexByID(id)
	if i < 0 {
		return ErrUserNotFound
	}
	previous := data.Users[i]
	data.Users[i].AuthVersion++
	data.Users[i].UpdatedAt = NowString()
	if err := save(); err != nil {
		data.Users[i] = previous
		return err
	}
	return nil
}

// credentialUser validates the effective owner while dataLock is held.
func credentialUser(id string, permissions []string) (*User, error) {
	if data == nil {
		return nil, ErrInvalidUser
	}
	if id == "" {
		for _, user := range data.Users {
			if user.Role == RoleOwner && user.IsActive {
				id = user.ID
				break
			}
		}
	}
	i := userIndexByID(id)
	if i < 0 || !data.Users[i].IsActive {
		return nil, fmt.Errorf("%w: 请选择有效成员", ErrInvalidUser)
	}
	user := data.Users[i]
	for _, permission := range permissions {
		if (permission != "read" && permission != "write" && permission != "delete") || !user.HasPermission(permission) {
			return nil, fmt.Errorf("%w: 凭证权限超出成员权限", ErrInvalidUser)
		}
	}
	return &user, nil
}
