package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

var directUploadLock sync.Mutex
var ErrUploadNotUploaded = errors.New("文件尚未上传到 R2")

type UploadActor struct {
	UserID      string
	AuthType    string
	TokenID     string
	AuthVersion int
}

type DirectUploadRequest struct {
	Filename       string `json:"filename"`
	Size           *int64 `json:"size"`
	ContentType    string `json:"contentType"`
	Path           string `json:"path"`
	IDGroup        string `json:"idGroup"`
	ExpirationDays *int   `json:"expirationDays"`
	RandomFilename *bool  `json:"randomFilename"`
	RequestID      string `json:"requestId"`
}

type DirectUploadInit struct {
	UploadID  string            `json:"uploadId"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt string            `json:"expiresAt"`
	AccountID string            `json:"accountId"`
	Key       string            `json:"key"`
	Size      int64             `json:"size"`
}

func containsUploadControl(value string) bool {
	return !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0
}

func normalizedUploadPath(value string) (string, error) {
	value = strings.Trim(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
	if len(value) > 800 || containsUploadControl(value) {
		return "", fmt.Errorf("%w: 上传目录无效或过长", store.ErrInvalidUploadSession)
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." {
			return "", fmt.Errorf("%w: 目录不能包含 . 或 ..", store.ErrInvalidUploadSession)
		}
	}
	return value, nil
}

func normalizedUploadFilename(value string) (string, error) {
	value = path.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if value == "" || value == "." || value == ".." || value == "/" || len(value) > 255 || containsUploadControl(value) {
		return "", fmt.Errorf("%w: 文件名无效或过长", store.ErrInvalidUploadSession)
	}
	return value, nil
}

func uploadKey(filename, prefix string, random bool) string {
	if random {
		filename = fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().UnixMilli(), path.Ext(filename))
	}
	if prefix == "" {
		prefix = time.Now().Format("2006/01/02")
	}
	return prefix + "/" + filename
}

func newDirectS3Client(account *store.Account) *s3.Client {
	return s3.NewFromConfig(aws.Config{
		Region: "auto", Credentials: credentials.NewStaticCredentialsProvider(account.AccessKeyId, account.SecretAccessKey, ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
	}, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(account.Endpoint)
		options.RetryMaxAttempts = 1
	})
}

type uploadPresigner struct {
	contentType string
	size        int64
}

func (p uploadPresigner) PresignHTTP(ctx context.Context, creds aws.Credentials, request *http.Request,
	hash, serviceName, region string, signingTime time.Time, options ...func(*v4.SignerOptions)) (string, http.Header, error) {
	// The SDK normally drops content-type for body-less presigning. Restore the
	// contract and bind the auto-generated browser Content-Length to the file size.
	request.Header.Set("Content-Type", p.contentType)
	request.Header.Set("If-None-Match", "*")
	request.Header.Set("Content-Length", strconv.FormatInt(p.size, 10))
	request.ContentLength = p.size
	options = append(options, func(opt *v4.SignerOptions) { opt.DisableHeaderHoisting = true; opt.DisableURIPathEscaping = true })
	return v4.NewSigner().PresignHTTP(ctx, creds, request, hash, serviceName, region, signingTime, options...)
}

func signDirectUpload(ctx context.Context, account *store.Account, session *store.UploadSession) (*DirectUploadInit, error) {
	expires, _ := time.Parse(time.RFC3339, session.UploadExpiresAt)
	duration := time.Until(expires)
	if duration < time.Second {
		return nil, store.ErrUploadSessionExpired
	}
	presigner := s3.NewPresignClient(newDirectS3Client(account), func(options *s3.PresignOptions) {
		options.Expires = duration
		options.Presigner = uploadPresigner{contentType: session.ContentType, size: session.ExpectedSize}
	})
	signed, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey),
		ContentLength: aws.Int64(session.ExpectedSize), ContentType: aws.String(session.ContentType), IfNoneMatch: aws.String("*"),
		Metadata:   map[string]string{"fileflow-upload-id": session.ID},
		ContentMD5: emptyUploadMD5(session.ExpectedSize),
	})
	if err != nil {
		return nil, fmt.Errorf("生成直传链接失败: %w", err)
	}
	headers := map[string]string{}
	for name, values := range signed.SignedHeader {
		if strings.EqualFold(name, "Host") || strings.EqualFold(name, "Content-Length") {
			continue
		}
		headers[name] = strings.Join(values, ",")
	}
	return &DirectUploadInit{session.ID, signed.URL, signed.Method, headers, session.UploadExpiresAt, account.ID, session.FileKey, session.ExpectedSize}, nil
}

func sameUploadActor(session *store.UploadSession, actor UploadActor) bool {
	return session.UserID == actor.UserID && session.AuthType == actor.AuthType && session.TokenID == actor.TokenID && session.AuthVersion == actor.AuthVersion
}

func uploadActorAuthorized(session *store.UploadSession, actor UploadActor) error {
	if !sameUploadActor(session, actor) {
		return store.ErrUploadSessionForbidden
	}
	user, err := store.GetUserByID(actor.UserID)
	if err != nil || !user.IsActive || !user.HasPermission("write") || user.AuthVersion != actor.AuthVersion {
		return store.ErrUploadSessionForbidden
	}
	return nil
}

func chooseDirectAccount(actor UploadActor, id string, size int64) (*store.Account, error) {
	accounts := store.GetAccounts()
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].GetUsagePercent() < accounts[j].GetUsagePercent() })
	for i := range accounts {
		account := &accounts[i]
		if id != "" && account.ID != id {
			continue
		}
		channel := (actor.AuthType == "jwt" && account.CanClientUpload()) || (actor.AuthType == "api_token" && account.CanAPIUpload())
		if !account.IsActive || !channel || (id == "" && !account.CanAutoUpload()) {
			continue
		}
		reservedSize, reservedA, _ := store.GetUploadReservations(account.ID)
		reservedB := int64(0)
		for _, session := range store.ListUploadSessions() {
			if session.AccountID == account.ID && session.Status != store.UploadStatusCompleted {
				reservedB += max(session.ReservedClassBOps-session.ConsumedClassBOps, 0)
			}
		}
		if account.Quota.MaxSizeBytes > 0 && account.Usage.SizeBytes <= account.Quota.MaxSizeBytes && size <= account.Quota.MaxSizeBytes-account.Usage.SizeBytes-reservedSize &&
			account.Quota.MaxClassAOps > 0 && account.Usage.ClassAOps+reservedA+1 <= account.Quota.MaxClassAOps &&
			(account.Quota.MaxClassBOps == 0 || (account.Usage.ClassBOps <= account.Quota.MaxClassBOps && reservedB+2 <= account.Quota.MaxClassBOps-account.Usage.ClassBOps)) {
			return account, nil
		}
	}
	return nil, store.ErrUploadQuotaExceeded
}

func isMissingR2Object(err error) bool {
	var apiError smithy.APIError
	return errors.As(err, &apiError) && (apiError.ErrorCode() == "NoSuchKey" || apiError.ErrorCode() == "NotFound" || apiError.ErrorCode() == "404")
}

func InitDirectUpload(ctx context.Context, actor UploadActor, input DirectUploadRequest) (*DirectUploadInit, error) {
	filename, err := normalizedUploadFilename(input.Filename)
	if err != nil {
		return nil, err
	}
	prefix, err := normalizedUploadPath(input.Path)
	if err != nil {
		return nil, err
	}
	if input.Size == nil || *input.Size < 0 || *input.Size > store.MaxDirectUploadFileSize {
		return nil, fmt.Errorf("%w: 文件大小必须在 0 至 5 GiB 之间", store.ErrInvalidUploadSession)
	}
	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if len(contentType) > 256 || containsUploadControl(contentType) {
		return nil, store.ErrInvalidUploadSession
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		return nil, fmt.Errorf("%w: 文件类型无效", store.ErrInvalidUploadSession)
	}
	days := store.GetSettings().DefaultExpirationDays
	if input.ExpirationDays != nil && *input.ExpirationDays != -1 {
		days = *input.ExpirationDays
	}
	if days < 0 || days > 3650 {
		return nil, fmt.Errorf("%w: 到期天数必须是 -1、0 或 1 至 3650", store.ErrInvalidUploadSession)
	}
	random := input.RandomFilename == nil || *input.RandomFilename
	if len(input.RequestID) > 128 || containsUploadControl(input.RequestID) {
		return nil, store.ErrInvalidUploadSession
	}
	id := strings.TrimSpace(input.IDGroup)
	if strings.Contains(id, ",") {
		return nil, fmt.Errorf("%w: 直传只能指定一个存储账号", store.ErrInvalidUploadSession)
	}
	directUploadLock.Lock()
	defer directUploadLock.Unlock()
	if input.RequestID != "" {
		for _, existing := range store.ListUploadSessions() {
			if existing.RequestID != input.RequestID || !sameUploadActor(&existing, actor) {
				continue
			}
			if existing.Filename != filename || existing.ExpectedSize != *input.Size || existing.ContentType != contentType || existing.ExpirationDays != days || existing.RandomFilename != random || existing.RequestedPath != prefix || (id != "" && existing.AccountID != id) {
				return nil, store.ErrUploadSessionConflict
			}
			if existing.Status != store.UploadStatusPending {
				return nil, store.ErrUploadSessionConflict
			}
			if err := uploadActorAuthorized(&existing, actor); err != nil {
				return nil, err
			}
			account, err := store.GetAccountByID(existing.AccountID)
			if err != nil {
				return nil, store.ErrUploadSessionForbidden
			}
			return signDirectUpload(ctx, account, &existing)
		}
	}
	account, err := chooseDirectAccount(actor, id, *input.Size)
	if err != nil {
		return nil, err
	}
	key := uploadKey(filename, prefix, random)
	if len(key) > 1024 || IsPendingUploadKey(account.ID, key) {
		return nil, store.ErrUploadSessionConflict
	}
	if err := ChargeAccountOperations(account.ID, 0, 1); err != nil {
		return nil, err
	}
	_, headErr := newDirectS3Client(account).HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(key)})
	if headErr == nil {
		return nil, fmt.Errorf("%w: 同名文件已存在，未覆盖原文件", store.ErrUploadSessionConflict)
	}
	if !isMissingR2Object(headErr) {
		return nil, fmt.Errorf("检查同名文件失败: %w", headErr)
	}
	now := time.Now().UTC()
	session := &store.UploadSession{ID: uuid.NewString(), UserID: actor.UserID, AuthType: actor.AuthType, TokenID: actor.TokenID, AuthVersion: actor.AuthVersion,
		AccountID: account.ID, FileKey: key, Filename: filename, ExpectedSize: *input.Size, ContentType: contentType, ExpirationDays: days,
		RandomFilename: random, RequestID: input.RequestID, RequestedPath: prefix, CreatedAt: now.Format(time.RFC3339), UploadExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}
	response, err := signDirectUpload(ctx, account, session)
	if err != nil {
		return nil, err
	}
	if err := store.CreateUploadSession(session); err != nil {
		return nil, err
	}
	return response, nil
}

func CompleteDirectUpload(ctx context.Context, actor UploadActor, id string) (*UploadResult, bool, error) {
	directUploadLock.Lock()
	defer directUploadLock.Unlock()
	session, err := store.GetUploadSessionByID(id)
	if err != nil {
		return nil, false, err
	}
	if err := uploadActorAuthorized(session, actor); err != nil {
		return nil, false, err
	}
	account, err := store.GetAccountByID(session.AccountID)
	if err != nil || !account.IsActive {
		return nil, false, store.ErrUploadSessionForbidden
	}
	if (actor.AuthType == "jwt" && !account.CanClientUpload()) || (actor.AuthType == "api_token" && !account.CanAPIUpload()) {
		return nil, false, store.ErrUploadSessionForbidden
	}
	result := &UploadResult{ID: account.ID, AccountName: account.Name, Key: session.FileKey, Size: session.ExpectedSize, URL: buildPublicURL(account.PublicDomain, session.FileKey)}
	if session.Status == store.UploadStatusCompleted {
		return result, false, nil
	}
	expires, _ := time.Parse(time.RFC3339, session.ExpiresAt)
	if !expires.After(time.Now()) {
		return nil, false, store.ErrUploadSessionExpired
	}
	if err := store.ConsumeUploadSessionRead(session.ID); err != nil {
		return nil, false, err
	}
	head, err := newDirectS3Client(account).HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey)})
	if isMissingR2Object(err) {
		return nil, false, ErrUploadNotUploaded
	}
	if err != nil {
		return nil, false, fmt.Errorf("验证 R2 上传结果失败: %w", err)
	}
	if head.Metadata["fileflow-upload-id"] != session.ID || aws.ToInt64(head.ContentLength) != session.ExpectedSize || aws.ToString(head.ContentType) != session.ContentType || aws.ToString(head.ETag) == "" {
		return nil, false, fmt.Errorf("%w: R2 文件大小、类型或上传任务标记不匹配", store.ErrUploadSessionConflict)
	}
	if err := store.UpdateUploadSessionState(session.ID, store.UploadStatusCompleting, aws.ToString(head.ETag)); err != nil {
		return nil, false, err
	}
	if err := store.FinalizeUploadSession(session.ID); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

// UploadRelay preserves /api/upload's multipart protocol while sharing direct
// upload naming, reservations, conditional writes and atomic confirmation.
func UploadRelay(ctx context.Context, actor UploadActor, input DirectUploadRequest, body io.Reader) (*UploadResult, error) {
	if input.Size != nil && *input.Size > store.MaxDirectUploadFileSize {
		return nil, store.ErrInvalidUploadSession
	}
	readLimit := store.MaxDirectUploadFileSize
	if input.Size != nil && *input.Size >= 0 {
		if _, err := chooseDirectAccount(actor, input.IDGroup, *input.Size); err != nil {
			return nil, err
		}
		readLimit = *input.Size
	} else {
		readLimit = 0
		for _, account := range store.GetAccounts() {
			if input.IDGroup != "" && input.IDGroup != account.ID {
				continue
			}
			allowed := (actor.AuthType == "jwt" && account.CanClientUpload()) || (actor.AuthType == "api_token" && account.CanAPIUpload())
			if !account.IsActive || !allowed || (input.IDGroup == "" && !account.CanAutoUpload()) {
				continue
			}
			reserved, _, _ := store.GetUploadReservations(account.ID)
			remaining := account.Quota.MaxSizeBytes - account.Usage.SizeBytes - reserved
			if remaining > readLimit {
				readLimit = remaining
			}
		}
		if readLimit <= 0 {
			return nil, store.ErrUploadQuotaExceeded
		}
		readLimit = min(readLimit, store.MaxDirectUploadFileSize)
	}
	content, err := io.ReadAll(io.LimitReader(body, readLimit+1))
	if err != nil {
		return nil, fmt.Errorf("读取上传内容失败: %w", err)
	}
	size := int64(len(content))
	if size > readLimit {
		return nil, fmt.Errorf("%w: 文件超过允许的大小或剩余存储容量", store.ErrUploadQuotaExceeded)
	}
	if input.Size != nil && *input.Size >= 0 && *input.Size != size {
		return nil, fmt.Errorf("%w: 文件大小与请求不匹配", store.ErrInvalidUploadSession)
	}
	input.Size = &size
	init, err := InitDirectUpload(ctx, actor, input)
	if err != nil {
		return nil, err
	}
	session, err := store.GetUploadSessionByID(init.UploadID)
	if err != nil {
		return nil, err
	}
	account, err := store.GetAccountByID(session.AccountID)
	if err != nil {
		return nil, err
	}
	_, err = newDirectS3Client(account).PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey),
		Body: bytes.NewReader(content), ContentLength: aws.Int64(size), ContentType: aws.String(session.ContentType), IfNoneMatch: aws.String("*"),
		ContentMD5: emptyUploadMD5(size),
		Metadata:   map[string]string{"fileflow-upload-id": session.ID}})
	if err != nil {
		return nil, fmt.Errorf("服务器中转上传失败: %w", err)
	}
	result, _, err := CompleteDirectUpload(ctx, actor, session.ID)
	return result, err
}

func emptyUploadMD5(size int64) *string {
	if size == 0 {
		return aws.String("1B2M2Y8AsgTpgAmY7PhCfg==")
	}
	return nil
}

// CleanupDirectUploads removes only expired objects created by their own task.
// A conflicting pre-existing filename is never deleted by this cleanup.
func CleanupDirectUploads(ctx context.Context) {
	directUploadLock.Lock()
	defer directUploadLock.Unlock()
	now := time.Now()
	for _, session := range store.ListUploadSessions() {
		if session.Status == store.UploadStatusCompleted {
			completed, _ := time.Parse(time.RFC3339, session.CompletedAt)
			if now.After(completed.Add(24 * time.Hour)) {
				_ = store.DeleteUploadSession(session.ID)
			}
			continue
		}
		expires, _ := time.Parse(time.RFC3339, session.ExpiresAt)
		if expires.After(now) {
			continue
		}
		account, err := store.GetAccountByID(session.AccountID)
		if err != nil {
			continue
		}
		if err := store.ConsumeUploadSessionCleanupRead(session.ID); err != nil {
			continue
		}
		client := newDirectS3Client(account)
		head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey)})
		if isMissingR2Object(err) {
			_ = store.DeleteUploadSession(session.ID)
			continue
		}
		if err != nil {
			continue
		}
		if head.Metadata["fileflow-upload-id"] == session.ID {
			_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey), IfMatch: head.ETag})
			if err != nil {
				continue
			}
		}
		_ = store.DeleteUploadSession(session.ID)
	}
}
