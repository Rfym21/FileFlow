package store

import "time"

// AccountPermissions 账户权限配置
type AccountPermissions struct {
	S3           bool `json:"s3"`           // 是否允许 S3 API 访问
	WebDAV       bool `json:"webdav"`       // 是否允许 WebDAV 访问
	AutoUpload   bool `json:"autoUpload"`   // 是否允许作为自动上传目标（SmartUpload）
	APIUpload    bool `json:"apiUpload"`    // 是否允许通过 API 上传
	ClientUpload bool `json:"clientUpload"` // 是否允许前端客户端上传
}

// DefaultAccountPermissions 返回默认权限配置（全部启用）
func DefaultAccountPermissions() AccountPermissions {
	return AccountPermissions{
		S3:           true,
		WebDAV:       true,
		AutoUpload:   true,
		APIUpload:    true,
		ClientUpload: true,
	}
}

// Account R2 账户
type Account struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	IsActive        bool               `json:"isActive"`
	Description     string             `json:"description"`
	AccountID       string             `json:"accountId"`       // Cloudflare Account ID
	AccessKeyId     string             `json:"accessKeyId"`     // R2 Access Key ID
	SecretAccessKey string             `json:"secretAccessKey"` // R2 Secret Access Key
	BucketName      string             `json:"bucketName"`
	Endpoint        string             `json:"endpoint"`     // R2 Endpoint URL
	PublicDomain    string             `json:"publicDomain"` // 公开访问域名
	APIToken        string             `json:"apiToken"`     // Cloudflare API Token (用于 GraphQL 查询)
	Quota           Quota              `json:"quota"`
	Usage           Usage              `json:"usage"`
	Permissions     AccountPermissions `json:"permissions"` // 账户权限配置
	CreatedAt       string             `json:"createdAt"`
	UpdatedAt       string             `json:"updatedAt"`
}

// Quota 账户配额限制（用户手动配置）
type Quota struct {
	MaxSizeBytes  int64 `json:"maxSizeBytes"`  // 最大存储容量（字节）
	MaxClassAOps  int64 `json:"maxClassAOps"`  // 最大 Class A 操作数
}

// Usage 账户使用量（通过 R2 API 动态获取）
type Usage struct {
	SizeBytes  int64  `json:"sizeBytes"`  // 当前已用容量
	ClassAOps  int64  `json:"classAOps"`  // 当前 Class A 操作数（写入操作）
	ClassBOps  int64  `json:"classBOps"`  // 当前 Class B 操作数（读取操作）
	LastSyncAt string `json:"lastSyncAt"` // 上次同步时间
}

// Token API 访问令牌
type Token struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Token       string   `json:"token"`
	Permissions []string `json:"permissions"` // read, write, delete
	CreatedAt   string   `json:"createdAt"`
}

// S3Credential S3 兼容 API 访问凭证
type S3Credential struct {
	ID              string   `json:"id"`
	AccessKeyID     string   `json:"accessKeyId"`     // 20 字符，如 FFLWXXXXXXXXXXXX
	SecretAccessKey string   `json:"secretAccessKey"` // 40 字符
	AccountID       string   `json:"accountId"`       // 关联的账户 ID
	Description     string   `json:"description"`
	Permissions     []string `json:"permissions"` // read, write, delete
	IsActive        bool     `json:"isActive"`
	CreatedAt       string   `json:"createdAt"`
	LastUsedAt      string   `json:"lastUsedAt"`
}

// HasPermission 检查 S3 凭证是否有指定权限
func (c *S3Credential) HasPermission(perm string) bool {
	for _, p := range c.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// WebDAVCredential WebDAV 访问凭证
type WebDAVCredential struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`    // WebDAV 用户名
	Password    string   `json:"password"`    // WebDAV 密码
	AccountID   string   `json:"accountId"`   // 关联的账户 ID
	Description string   `json:"description"`
	Permissions []string `json:"permissions"` // read, write, delete
	IsActive    bool     `json:"isActive"`
	CreatedAt   string   `json:"createdAt"`
	LastUsedAt  string   `json:"lastUsedAt"`
}

// HasPermission 检查 WebDAV 凭证是否有指定权限
func (c *WebDAVCredential) HasPermission(perm string) bool {
	for _, p := range c.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// FileExpiration 文件到期记录
type FileExpiration struct {
	ID        string `json:"id"`        // 记录ID
	AccountID string `json:"accountId"` // 所属账户ID
	FileKey   string `json:"fileKey"`   // S3中的文件路径
	ExpiresAt string `json:"expiresAt"` // 到期时间 (ISO 8601)
	CreatedAt string `json:"createdAt"` // 创建时间
}

// Settings 系统设置
type Settings struct {
	SyncInterval           int    `json:"syncInterval"`           // 同步间隔（分钟），默认 5
	EndpointProxy          bool   `json:"endpointProxy"`          // 启用 URL 代理
	EndpointProxyURL       string `json:"endpointProxyUrl"`       // 反代 URL
	DefaultExpirationDays  int    `json:"defaultExpirationDays"`  // 默认文件到期天数，默认 30，0 表示永久
	ExpirationCheckMinutes int    `json:"expirationCheckMinutes"` // 到期检查间隔（分钟），默认 720（12小时）
}

// Data 存储的完整数据结构
type Data struct {
	Accounts          []Account          `json:"accounts"`
	Tokens            []Token            `json:"tokens"`
	S3Credentials     []S3Credential     `json:"s3Credentials"`
	WebDAVCredentials []WebDAVCredential `json:"webdavCredentials"`
	FileExpirations   []FileExpiration   `json:"fileExpirations"`
	Settings          Settings           `json:"settings"`
}

// HasPermission 检查 Token 是否有指定权限
func (t *Token) HasPermission(perm string) bool {
	for _, p := range t.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// IsOverQuota 检查账户是否超过配额
func (a *Account) IsOverQuota() bool {
	return a.Usage.SizeBytes >= a.Quota.MaxSizeBytes
}

// IsOverOps 检查账户是否超过操作次数限制
func (a *Account) IsOverOps() bool {
	return a.Usage.ClassAOps >= a.Quota.MaxClassAOps
}

// IsAvailable 检查账户是否可用于上传
func (a *Account) IsAvailable() bool {
	return a.IsActive && !a.IsOverQuota() && !a.IsOverOps()
}

// CanS3 检查账户是否允许 S3 API 访问
func (a *Account) CanS3() bool {
	return a.Permissions.S3
}

// CanWebDAV 检查账户是否允许 WebDAV 访问
func (a *Account) CanWebDAV() bool {
	return a.Permissions.WebDAV
}

// CanAutoUpload 检查账户是否允许作为自动上传目标
func (a *Account) CanAutoUpload() bool {
	return a.Permissions.AutoUpload
}

// CanAPIUpload 检查账户是否允许通过 API 上传
func (a *Account) CanAPIUpload() bool {
	return a.Permissions.APIUpload
}

// CanClientUpload 检查账户是否允许前端客户端上传
func (a *Account) CanClientUpload() bool {
	return a.Permissions.ClientUpload
}

// IsAvailableForAutoUpload 检查账户是否可用于自动上传
func (a *Account) IsAvailableForAutoUpload() bool {
	return a.IsAvailable() && a.CanAutoUpload() && a.CanAPIUpload()
}

// IsAvailableForAPIUpload 检查账户是否可用于 API 上传
func (a *Account) IsAvailableForAPIUpload() bool {
	return a.IsAvailable() && a.CanAPIUpload()
}

// IsAvailableForClientUpload 检查账户是否可用于前端上传
func (a *Account) IsAvailableForClientUpload() bool {
	return a.IsAvailable() && a.CanClientUpload()
}

// GetUsagePercent 获取容量使用百分比
func (a *Account) GetUsagePercent() float64 {
	if a.Quota.MaxSizeBytes == 0 {
		return 0
	}
	return float64(a.Usage.SizeBytes) / float64(a.Quota.MaxSizeBytes) * 100
}

// NowString 获取当前时间的 ISO 字符串
func NowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}
