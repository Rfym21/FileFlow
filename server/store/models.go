package store

import "time"

// Account R2 账户
type Account struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	IsActive        bool   `json:"isActive"`
	Description     string `json:"description"`
	AccountID       string `json:"accountId"`       // Cloudflare Account ID
	AccessKeyId     string `json:"accessKeyId"`     // R2 Access Key ID
	SecretAccessKey string `json:"secretAccessKey"` // R2 Secret Access Key
	BucketName      string `json:"bucketName"`
	Endpoint        string `json:"endpoint"`     // R2 Endpoint URL
	PublicDomain    string `json:"publicDomain"` // 公开访问域名
	APIToken        string `json:"apiToken"`     // Cloudflare API Token (用于 GraphQL 查询)
	Quota           Quota  `json:"quota"`
	Usage           Usage  `json:"usage"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
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

// Data 存储的完整数据结构
type Data struct {
	Accounts []Account `json:"accounts"`
	Tokens   []Token   `json:"tokens"`
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
