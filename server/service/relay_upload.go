package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

var uploadOperationLock sync.Mutex
var ErrUploadNotUploaded = errors.New("文件尚未成功上传")

type UploadActor struct {
	UserID      string
	AuthType    string
	TokenID     string
	AuthVersion int
}

type UploadRequest struct {
	Filename       string `json:"filename"`
	Size           *int64 `json:"size"`
	ContentType    string `json:"contentType"`
	Path           string `json:"path"`
	IDGroup        string `json:"idGroup"`
	ExpirationDays *int   `json:"expirationDays"`
	RandomFilename *bool  `json:"randomFilename"`
	RequestID      string `json:"requestId"`
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

// These requests are already charged through upload task reservations. SDK
// retries are disabled; each deliberate repeated PUT is separately authorized.
func getUploadS3Client(account *store.Account) *s3.Client {
	return s3.NewFromConfig(aws.Config{
		Region: "auto", Credentials: credentials.NewStaticCredentialsProvider(account.AccessKeyId, account.SecretAccessKey, ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		HTTPClient:                 &http.Client{Timeout: 5 * time.Minute},
	}, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(account.Endpoint)
		options.RetryMaxAttempts = 1
	})
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
	if actor.AuthType == "jwt" && actor.TokenID == "" {
		return nil
	}
	if actor.AuthType != "api_token" || actor.TokenID == "" {
		return store.ErrUploadSessionForbidden
	}
	for _, token := range store.GetTokens() {
		if token.ID == actor.TokenID && token.UserID == actor.UserID && token.HasPermission("write") {
			return nil
		}
	}
	return store.ErrUploadSessionForbidden
}

func chooseUploadAccount(actor UploadActor, id string, size int64) (*store.Account, error) {
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
		reservedB := store.GetUploadReadReservations(account.ID)
		if size >= 0 && account.Quota.MaxSizeBytes > 0 && account.Usage.SizeBytes <= account.Quota.MaxSizeBytes && size <= account.Quota.MaxSizeBytes-account.Usage.SizeBytes-reservedSize &&
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

func validateUploadRequest(input UploadRequest) (UploadRequest, int, bool, error) {
	var err error
	input.Filename, err = normalizedUploadFilename(input.Filename)
	if err != nil {
		return input, 0, false, err
	}
	input.Path, err = normalizedUploadPath(input.Path)
	if err != nil {
		return input, 0, false, err
	}
	if input.Size != nil && (*input.Size < -1 || *input.Size > store.MaxUploadFileSize) {
		return input, 0, false, fmt.Errorf("%w: 文件大小不能超过 5 GiB", store.ErrInvalidUploadSession)
	}
	input.ContentType = strings.TrimSpace(input.ContentType)
	if input.ContentType == "" {
		input.ContentType = "application/octet-stream"
	}
	if len(input.ContentType) > 256 || containsUploadControl(input.ContentType) {
		return input, 0, false, store.ErrInvalidUploadSession
	}
	if _, _, err := mime.ParseMediaType(input.ContentType); err != nil {
		return input, 0, false, fmt.Errorf("%w: 文件类型无效", store.ErrInvalidUploadSession)
	}
	days := store.GetSettings().DefaultExpirationDays
	if input.ExpirationDays != nil && *input.ExpirationDays != -1 {
		days = *input.ExpirationDays
	}
	if days < 0 || days > 3650 {
		return input, 0, false, fmt.Errorf("%w: 到期天数必须是 -1、0 或 1 至 3650", store.ErrInvalidUploadSession)
	}
	if len(input.RequestID) > 128 || containsUploadControl(input.RequestID) {
		return input, 0, false, store.ErrInvalidUploadSession
	}
	input.IDGroup = strings.TrimSpace(input.IDGroup)
	if len(input.IDGroup) > 128 || strings.Contains(input.IDGroup, ",") || containsUploadControl(input.IDGroup) {
		return input, 0, false, fmt.Errorf("%w: 上传只能指定一个存储账号", store.ErrInvalidUploadSession)
	}
	return input, days, input.RandomFilename == nil || *input.RandomFilename, nil
}

func findUploadRetry(actor UploadActor, requestID string) *store.UploadSession {
	if requestID == "" {
		return nil
	}
	for _, session := range store.ListUploadSessions() {
		if session.RequestID == requestID && sameUploadActor(&session, actor) {
			return &session
		}
	}
	return nil
}

// initializeUploadTask is internal: no browser upload URL or public completion
// capability is created. The server owns all byte transfer and task recovery.
func initializeUploadTask(ctx context.Context, actor UploadActor, input UploadRequest, contentSHA256 string) (*store.UploadSession, bool, error) {
	input, days, random, err := validateUploadRequest(input)
	if err != nil || input.Size == nil || *input.Size < 0 {
		if err == nil {
			err = store.ErrInvalidUploadSession
		}
		return nil, false, err
	}
	uploadOperationLock.Lock()
	defer uploadOperationLock.Unlock()
	if existing := findUploadRetry(actor, input.RequestID); existing != nil {
		if existing.Filename != input.Filename || existing.ExpectedSize != *input.Size || existing.ContentType != input.ContentType || existing.ExpirationDays != days || existing.RandomFilename != random || existing.RequestedPath != input.Path || (input.IDGroup != "" && existing.AccountID != input.IDGroup) || existing.ContentSHA256 == "" || existing.ContentSHA256 != contentSHA256 {
			return nil, false, fmt.Errorf("%w: requestId 已用于不同的文件内容或上传参数", store.ErrUploadSessionConflict)
		}
		if err := uploadActorAuthorized(existing, actor); err != nil {
			return nil, false, err
		}
		if existing.Status != store.UploadStatusCompleted {
			expires, _ := time.Parse(time.RFC3339, existing.ExpiresAt)
			if !expires.After(time.Now()) {
				return nil, false, store.ErrUploadSessionExpired
			}
		}
		return existing, true, nil
	}
	account, err := chooseUploadAccount(actor, input.IDGroup, *input.Size)
	if err != nil {
		return nil, false, err
	}
	key := uploadKey(input.Filename, input.Path, random)
	if len(key) > 1024 || IsPendingUploadKey(account.ID, key) {
		return nil, false, store.ErrUploadSessionConflict
	}
	if _, err := getS3Client(account).HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(key)}); err == nil {
		return nil, false, fmt.Errorf("%w: 同名文件已存在，未覆盖原文件", store.ErrUploadSessionConflict)
	} else if !isMissingR2Object(err) {
		return nil, false, fmt.Errorf("检查同名文件失败: %w", err)
	}
	now := time.Now().UTC()
	session := &store.UploadSession{ID: uuid.NewString(), UserID: actor.UserID, AuthType: actor.AuthType, TokenID: actor.TokenID, AuthVersion: actor.AuthVersion,
		AccountID: account.ID, FileKey: key, Filename: input.Filename, ExpectedSize: *input.Size, ContentType: input.ContentType, ContentSHA256: contentSHA256, ExpirationDays: days,
		RandomFilename: random, RequestID: input.RequestID, RequestedPath: input.Path, CreatedAt: now.Format(time.RFC3339), UploadExpiresAt: now.Add(time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}
	if err := store.CreateUploadSession(session); err != nil {
		return nil, false, err
	}
	return session, false, nil
}

func completeUploadTask(ctx context.Context, actor UploadActor, id string) (*UploadResult, bool, error) {
	uploadOperationLock.Lock()
	defer uploadOperationLock.Unlock()
	session, err := store.GetUploadSessionByID(id)
	if err != nil {
		return nil, false, err
	}
	if err := uploadActorAuthorized(session, actor); err != nil {
		return nil, false, err
	}
	account, err := store.GetAccountByID(session.AccountID)
	if err != nil || !account.IsActive || (actor.AuthType == "jwt" && !account.CanClientUpload()) || (actor.AuthType == "api_token" && !account.CanAPIUpload()) {
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
	head, err := getUploadS3Client(account).HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey)})
	if isMissingR2Object(err) {
		return nil, false, ErrUploadNotUploaded
	}
	if err != nil {
		return nil, false, fmt.Errorf("验证上传结果失败: %w", err)
	}
	if head.Metadata["fileflow-upload-id"] != session.ID || aws.ToInt64(head.ContentLength) != session.ExpectedSize || aws.ToString(head.ContentType) != session.ContentType || aws.ToString(head.ETag) == "" || (session.ContentSHA256 != "" && head.Metadata["fileflow-content-sha256"] != session.ContentSHA256) {
		return nil, false, fmt.Errorf("%w: 文件大小、类型、内容摘要或上传任务标记不匹配", store.ErrUploadSessionConflict)
	}
	if err := store.UpdateUploadSessionState(session.ID, store.UploadStatusCompleting, aws.ToString(head.ETag)); err != nil {
		return nil, false, err
	}
	if err := store.FinalizeUploadSession(session.ID); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

type relayRequestLock struct {
	mutex sync.Mutex
	refs  int
}

var relayRequestLocks = struct {
	sync.Mutex
	items map[string]*relayRequestLock
}{items: make(map[string]*relayRequestLock)}

func lockRelayRequest(actor UploadActor, requestID string) func() {
	if requestID == "" {
		return func() {}
	}
	key := strings.Join([]string{actor.UserID, actor.AuthType, actor.TokenID, requestID}, "\x00")
	relayRequestLocks.Lock()
	item := relayRequestLocks.items[key]
	if item == nil {
		item = &relayRequestLock{}
		relayRequestLocks.items[key] = item
	}
	item.refs++
	relayRequestLocks.Unlock()
	item.mutex.Lock()
	return func() {
		item.mutex.Unlock()
		relayRequestLocks.Lock()
		item.refs--
		if item.refs == 0 {
			delete(relayRequestLocks.items, key)
		}
		relayRequestLocks.Unlock()
	}
}

func relayReadLimit(actor UploadActor, input UploadRequest) (int64, error) {
	// Matching requests must still read and hash bytes to verify idempotency, even
	// if the original upload filled the remaining capacity or operation allowance.
	if findUploadRetry(actor, input.RequestID) != nil {
		if input.Size != nil && *input.Size >= 0 {
			return *input.Size, nil
		}
		return store.MaxUploadFileSize, nil
	}
	if input.Size != nil && *input.Size >= 0 {
		if _, err := chooseUploadAccount(actor, input.IDGroup, *input.Size); err != nil {
			return 0, err
		}
		return *input.Size, nil
	}
	var readLimit int64
	for _, account := range store.GetAccounts() {
		if input.IDGroup != "" && input.IDGroup != account.ID {
			continue
		}
		allowed := (actor.AuthType == "jwt" && account.CanClientUpload()) || (actor.AuthType == "api_token" && account.CanAPIUpload())
		if !account.IsActive || !allowed || (input.IDGroup == "" && !account.CanAutoUpload()) {
			continue
		}
		reserved, _, _ := store.GetUploadReservations(account.ID)
		readLimit = max(readLimit, account.Quota.MaxSizeBytes-account.Usage.SizeBytes-reserved)
	}
	if readLimit <= 0 {
		return 0, store.ErrUploadQuotaExceeded
	}
	return min(readLimit, store.MaxUploadFileSize), nil
}

// UploadRelay transfers every file through the server, with bounded temporary
// disk storage and a streaming digest instead of reading up to 5 GiB into RAM.
// requestId binds retries to one actor, file digest and upload options.
func UploadRelay(ctx context.Context, actor UploadActor, input UploadRequest, body io.Reader) (*UploadResult, bool, error) {
	input, _, _, err := validateUploadRequest(input)
	if err != nil {
		return nil, false, err
	}
	if err := uploadActorAuthorized(&store.UploadSession{UserID: actor.UserID, AuthType: actor.AuthType, TokenID: actor.TokenID, AuthVersion: actor.AuthVersion}, actor); err != nil {
		return nil, false, err
	}
	readLimit, err := relayReadLimit(actor, input)
	if err != nil {
		return nil, false, err
	}
	spool, err := os.CreateTemp("", "fileflow-upload-*")
	if err != nil {
		return nil, false, fmt.Errorf("创建上传临时文件失败: %w", err)
	}
	defer func() { _ = spool.Close(); _ = os.Remove(spool.Name()) }()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(spool, hash), io.LimitReader(body, readLimit+1))
	if err != nil {
		return nil, false, fmt.Errorf("读取上传内容失败: %w", err)
	}
	if size > readLimit {
		return nil, false, fmt.Errorf("%w: 文件超过允许的大小或剩余存储容量", store.ErrUploadQuotaExceeded)
	}
	if input.Size != nil && *input.Size >= 0 && *input.Size != size {
		return nil, false, fmt.Errorf("%w: 文件大小与请求不匹配", store.ErrInvalidUploadSession)
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return nil, false, err
	}
	input.Size = &size
	unlock := lockRelayRequest(actor, input.RequestID)
	defer unlock()
	session, reused, err := initializeUploadTask(ctx, actor, input, hex.EncodeToString(hash.Sum(nil)))
	if err != nil {
		return nil, false, err
	}
	if reused {
		result, changed, err := completeUploadTask(ctx, actor, session.ID)
		if err == nil || !errors.Is(err, ErrUploadNotUploaded) {
			return result, changed, err
		}
	}
	account, err := store.GetAccountByID(session.AccountID)
	if err != nil || !account.IsActive || (actor.AuthType == "jwt" && !account.CanClientUpload()) || (actor.AuthType == "api_token" && !account.CanAPIUpload()) {
		return nil, false, store.ErrUploadSessionForbidden
	}
	if err := uploadActorAuthorized(session, actor); err != nil {
		return nil, false, err
	}
	if reused {
		if err := ChargeAccountOperations(account.ID, 1, 0); err != nil {
			return nil, false, err
		}
	}
	_, err = getUploadS3Client(account).PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey),
		Body: spool, ContentLength: aws.Int64(size), ContentType: aws.String(session.ContentType), IfNoneMatch: aws.String("*"), ContentMD5: emptyUploadMD5(size),
		Metadata: map[string]string{"fileflow-upload-id": session.ID, "fileflow-content-sha256": session.ContentSHA256}})
	if err != nil {
		return nil, false, fmt.Errorf("服务器上传失败: %w", mcpOperationConflict(err))
	}
	return completeUploadTask(ctx, actor, session.ID)
}

func emptyUploadMD5(size int64) *string {
	if size == 0 {
		return aws.String("1B2M2Y8AsgTpgAmY7PhCfg==")
	}
	return nil
}

// cleanupUploadTasks also handles persisted tasks issued by older versions.
// Unknown objects with the same key are never removed by this cleanup.
func cleanupUploadTasks(ctx context.Context) {
	uploadOperationLock.Lock()
	defer uploadOperationLock.Unlock()
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
		client := getUploadS3Client(account)
		head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey)})
		if isMissingR2Object(err) {
			_ = store.DeleteUploadSession(session.ID)
			continue
		}
		if err != nil {
			continue
		}
		if head.Metadata["fileflow-upload-id"] == session.ID {
			if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey)}); err != nil {
				continue
			}
		}
		_ = store.DeleteUploadSession(session.ID)
	}
}
