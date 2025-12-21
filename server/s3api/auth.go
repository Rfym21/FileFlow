package s3api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"fileflow/server/store"

	"github.com/gin-gonic/gin"
)

const (
	// S3 签名算法
	signatureAlgorithm = "AWS4-HMAC-SHA256"
	// S3 服务名称
	s3Service = "s3"
	// 请求类型
	aws4Request = "aws4_request"
	// 时间格式
	iso8601Format     = "20060102T150405Z"
	iso8601DateFormat = "20060102"
)

// SignatureV4Info 解析后的签名信息
type SignatureV4Info struct {
	AccessKeyID   string
	Date          time.Time
	Region        string
	Service       string
	SignedHeaders []string
	Signature     string
	Credential    string
}

// ContextKeys
const (
	ContextKeyS3Credential = "s3_credential"
	ContextKeyS3Account    = "s3_account"
)

// S3AuthMiddleware AWS Signature v4 认证中间件
func S3AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			WriteS3Error(c, ErrAccessDenied)
			c.Abort()
			return
		}

		// 解析 Authorization 头
		sigInfo, err := parseAuthorizationHeader(authHeader)
		if err != nil {
			WriteS3ErrorWithMessage(c, ErrInvalidRequest, err.Error())
			c.Abort()
			return
		}

		// 获取凭证
		cred, err := store.GetS3CredentialByAccessKey(sigInfo.AccessKeyID)
		if err != nil {
			WriteS3Error(c, ErrInvalidAccessKeyId)
			c.Abort()
			return
		}

		if !cred.IsActive {
			WriteS3Error(c, ErrAccessDenied)
			c.Abort()
			return
		}

		// 验证签名
		if err := verifySignatureV4(c.Request, sigInfo, cred.SecretAccessKey); err != nil {
			WriteS3Error(c, ErrSignatureDoesNotMatch)
			c.Abort()
			return
		}

		// 获取关联的账户
		acc, err := store.GetAccountByID(cred.AccountID)
		if err != nil {
			WriteS3Error(c, ErrInternalError)
			c.Abort()
			return
		}

		// 将凭证和账户信息存入上下文
		c.Set(ContextKeyS3Credential, cred)
		c.Set(ContextKeyS3Account, acc)

		// 更新最后使用时间
		go store.UpdateS3CredentialLastUsed(cred.ID)

		c.Next()
	}
}

// parseAuthorizationHeader 解析 Authorization 头
// 格式: AWS4-HMAC-SHA256 Credential=AKID/20231221/us-east-1/s3/aws4_request,SignedHeaders=host;x-amz-content-sha256;x-amz-date,Signature=xxx
func parseAuthorizationHeader(header string) (*SignatureV4Info, error) {
	if !strings.HasPrefix(header, signatureAlgorithm) {
		return nil, fmt.Errorf("unsupported signature algorithm")
	}

	header = strings.TrimPrefix(header, signatureAlgorithm)
	header = strings.TrimSpace(header)

	info := &SignatureV4Info{}

	// 解析各部分
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.HasPrefix(part, "Credential=") {
			info.Credential = strings.TrimPrefix(part, "Credential=")
			// 解析 Credential: AKID/20231221/us-east-1/s3/aws4_request
			credParts := strings.Split(info.Credential, "/")
			if len(credParts) != 5 {
				return nil, fmt.Errorf("invalid credential format")
			}
			info.AccessKeyID = credParts[0]
			date, err := time.Parse(iso8601DateFormat, credParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid date in credential")
			}
			info.Date = date
			info.Region = credParts[2]
			info.Service = credParts[3]
		} else if strings.HasPrefix(part, "SignedHeaders=") {
			signedHeaders := strings.TrimPrefix(part, "SignedHeaders=")
			info.SignedHeaders = strings.Split(signedHeaders, ";")
		} else if strings.HasPrefix(part, "Signature=") {
			info.Signature = strings.TrimPrefix(part, "Signature=")
		}
	}

	if info.AccessKeyID == "" || info.Signature == "" || len(info.SignedHeaders) == 0 {
		return nil, fmt.Errorf("missing required fields in authorization header")
	}

	return info, nil
}

// verifySignatureV4 验证 AWS Signature v4
func verifySignatureV4(r *http.Request, sigInfo *SignatureV4Info, secretKey string) error {
	// 获取请求时间
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		amzDate = r.Header.Get("Date")
	}

	// 1. 构建规范请求
	canonicalRequest := buildCanonicalRequest(r, sigInfo.SignedHeaders)

	// 2. 构建待签名字符串
	stringToSign := buildStringToSign(sigInfo, canonicalRequest, amzDate)

	// 3. 计算签名密钥
	signingKey := deriveSigningKey(secretKey, sigInfo.Date, sigInfo.Region, sigInfo.Service)

	// 4. 计算签名
	expectedSignature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// 5. 比较签名
	if !hmac.Equal([]byte(expectedSignature), []byte(sigInfo.Signature)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// buildCanonicalRequest 构建规范请求
func buildCanonicalRequest(r *http.Request, signedHeaders []string) string {
	var sb strings.Builder

	// HTTP 方法
	sb.WriteString(r.Method)
	sb.WriteByte('\n')

	// URI 编码的路径
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	sb.WriteString(uriEncode(path, false))
	sb.WriteByte('\n')

	// 规范化查询字符串
	sb.WriteString(canonicalQueryString(r.URL.Query()))
	sb.WriteByte('\n')

	// 规范化头部
	sort.Strings(signedHeaders)
	for _, h := range signedHeaders {
		sb.WriteString(strings.ToLower(h))
		sb.WriteByte(':')

		var value string
		// 特殊处理 Host 头部
		if strings.ToLower(h) == "host" {
			value = r.Host
		} else {
			value = strings.TrimSpace(r.Header.Get(h))
		}

		// 压缩连续空格
		value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
		sb.WriteString(value)
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')

	// 已签名头部列表
	sb.WriteString(strings.Join(signedHeaders, ";"))
	sb.WriteByte('\n')

	// Payload 哈希
	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
	}
	sb.WriteString(payloadHash)

	return sb.String()
}

// buildStringToSign 构建待签名字符串
func buildStringToSign(sigInfo *SignatureV4Info, canonicalRequest string, amzDate string) string {
	var sb strings.Builder

	// 算法
	sb.WriteString(signatureAlgorithm)
	sb.WriteByte('\n')

	// 时间戳
	sb.WriteString(amzDate)
	sb.WriteByte('\n')

	// 范围
	scope := fmt.Sprintf("%s/%s/%s/%s",
		sigInfo.Date.Format(iso8601DateFormat),
		sigInfo.Region,
		sigInfo.Service,
		aws4Request,
	)
	sb.WriteString(scope)
	sb.WriteByte('\n')

	// 规范请求的哈希
	hash := sha256.Sum256([]byte(canonicalRequest))
	sb.WriteString(hex.EncodeToString(hash[:]))

	return sb.String()
}

// deriveSigningKey 派生签名密钥
func deriveSigningKey(secretKey string, date time.Time, region, service string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secretKey), []byte(date.Format(iso8601DateFormat)))
	regionKey := hmacSHA256(dateKey, []byte(region))
	serviceKey := hmacSHA256(regionKey, []byte(service))
	signingKey := hmacSHA256(serviceKey, []byte(aws4Request))
	return signingKey
}

// hmacSHA256 计算 HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// uriEncode URI 编码
func uriEncode(s string, encodeSlash bool) string {
	var result strings.Builder
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '~' || c == '.' {
			result.WriteRune(c)
		} else if c == '/' && !encodeSlash {
			result.WriteRune(c)
		} else {
			result.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return result.String()
}

// canonicalQueryString 规范化查询字符串
func canonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	// 获取所有键并排序
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建查询字符串
	var parts []string
	for _, k := range keys {
		vs := values[k]
		sort.Strings(vs)
		for _, v := range vs {
			parts = append(parts, uriEncode(k, true)+"="+uriEncode(v, true))
		}
	}

	return strings.Join(parts, "&")
}

// GetS3CredentialFromContext 从上下文获取 S3 凭证
func GetS3CredentialFromContext(c *gin.Context) *store.S3Credential {
	if cred, exists := c.Get(ContextKeyS3Credential); exists {
		return cred.(*store.S3Credential)
	}
	return nil
}

// GetS3AccountFromContext 从上下文获取账户
func GetS3AccountFromContext(c *gin.Context) *store.Account {
	if acc, exists := c.Get(ContextKeyS3Account); exists {
		return acc.(*store.Account)
	}
	return nil
}

// HasPermission 检查是否有权限
func HasPermission(c *gin.Context, perm string) bool {
	cred := GetS3CredentialFromContext(c)
	if cred == nil {
		return false
	}
	for _, p := range cred.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}
