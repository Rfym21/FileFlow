package webdav

import (
	"fileflow/server/store"
)

// User WebDAV 用户接口（兼容 OpenList model.User）
type User interface {
	// GetBasePath 获取用户的基础路径
	GetBasePath() string
	// CanWebdavRead 是否有 WebDAV 读取权限
	CanWebdavRead() bool
	// CanWebdavManage 是否有 WebDAV 管理权限
	CanWebdavManage() bool
	// CanWrite 是否有写入权限
	CanWrite() bool
	// CanMove 是否有移动权限
	CanMove() bool
	// CanRename 是否有重命名权限
	CanRename() bool
	// CanCopy 是否有复制权限
	CanCopy() bool
	// CanRemove 是否有删除权限
	CanRemove() bool
}

// WebDAVUser 用户权限包装器
type WebDAVUser struct {
	cred *store.WebDAVCredential
	acc  *store.Account
}

// NewWebDAVUser 创建 WebDAV 用户
func NewWebDAVUser(cred *store.WebDAVCredential, acc *store.Account) *WebDAVUser {
	return &WebDAVUser{
		cred: cred,
		acc:  acc,
	}
}

// GetBasePath 获取基础路径（WebDAV 根目录）
func (u *WebDAVUser) GetBasePath() string {
	return "/"
}

// CanWebdavRead 是否有读取权限
func (u *WebDAVUser) CanWebdavRead() bool {
	return u.cred.HasPermission("read")
}

// CanWebdavManage 是否有管理权限
func (u *WebDAVUser) CanWebdavManage() bool {
	return u.cred.HasPermission("write")
}

// CanWrite 是否有写入权限
func (u *WebDAVUser) CanWrite() bool {
	return u.cred.HasPermission("write")
}

// CanMove 是否有移动权限
func (u *WebDAVUser) CanMove() bool {
	return u.cred.HasPermission("write")
}

// CanRename 是否有重命名权限
func (u *WebDAVUser) CanRename() bool {
	return u.cred.HasPermission("write")
}

// CanCopy 是否有复制权限
func (u *WebDAVUser) CanCopy() bool {
	return u.cred.HasPermission("write")
}

// CanRemove 是否有删除权限
func (u *WebDAVUser) CanRemove() bool {
	return u.cred.HasPermission("delete")
}

// GetCredential 获取原始凭证
func (u *WebDAVUser) GetCredential() *store.WebDAVCredential {
	return u.cred
}

// GetAccount 获取关联账户
func (u *WebDAVUser) GetAccount() *store.Account {
	return u.acc
}
