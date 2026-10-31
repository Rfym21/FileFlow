package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

var (
	ErrMCPFileNotFound = errors.New("源文件不存在")
	ErrMCPFileExpired  = errors.New("源文件已到期，不能复制或移动")
)

type MCPFileOperationRequest struct {
	IDGroup        string `json:"idGroup"`
	Key            string `json:"key"`
	DestinationKey string `json:"destinationKey"`
}

// MCPFileOperationResult retains recovery information even if a remote request
// succeeded but confirmation, expiration persistence or source deletion failed.
type MCPFileOperationResult struct {
	Destination          *UploadResult `json:"destination"`
	DestinationConfirmed bool          `json:"destinationConfirmed"`
	SourceKey            string        `json:"sourceKey,omitempty"`
	SourceDeleted        bool          `json:"sourceDeleted"`
	Directory            bool          `json:"directory"`
	ExpiresAt            string        `json:"expiresAt,omitempty"`
}

func validateMCPFileKey(key string, directory bool) (string, error) {
	if key == "" || len(key) > 1024 || containsUploadControl(key) || strings.Contains(key, "\\") || strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("%w: 文件路径无效", store.ErrInvalidUploadSession)
	}
	if directory {
		key = strings.TrimSuffix(key, "/")
	} else if strings.HasSuffix(key, "/") {
		return "", fmt.Errorf("%w: 复制和移动仅支持单个文件，请逐页列出目录中的文件后分别操作", store.ErrInvalidUploadSession)
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("%w: 路径不能包含空目录、. 或 ..", store.ErrInvalidUploadSession)
		}
	}
	if directory {
		key += "/"
		if len(key) > 1024 {
			return "", store.ErrInvalidUploadSession
		}
	}
	return key, nil
}

func authorizeMCPFileActor(actor UploadActor, permissions ...string) error {
	user, err := store.GetUserByID(actor.UserID)
	if err != nil || !user.IsActive || user.AuthVersion != actor.AuthVersion {
		return store.ErrUploadSessionForbidden
	}
	for _, permission := range permissions {
		if !user.HasPermission(permission) {
			return store.ErrUploadSessionForbidden
		}
	}
	if actor.AuthType == "jwt" && actor.TokenID == "" {
		return nil
	}
	if actor.AuthType != "api_token" || actor.TokenID == "" {
		return store.ErrUploadSessionForbidden
	}
	for _, token := range store.GetTokens() {
		if token.ID != actor.TokenID || token.UserID != user.ID {
			continue
		}
		for _, permission := range permissions {
			if !token.HasPermission(permission) {
				return store.ErrUploadSessionForbidden
			}
		}
		return nil
	}
	return store.ErrUploadSessionForbidden
}

func mcpOperationAccount(actor UploadActor, id string) (*store.Account, error) {
	if id == "" || strings.Contains(id, ",") || len(id) > 128 || containsUploadControl(id) {
		return nil, store.ErrInvalidUploadSession
	}
	account, err := store.GetAccountByID(id)
	if err != nil || !account.IsActive ||
		(actor.AuthType == "jwt" && !account.CanClientUpload()) ||
		(actor.AuthType == "api_token" && !account.CanAPIUpload()) {
		return nil, store.ErrUploadSessionForbidden
	}
	return account, nil
}

func mcpOperationConflict(err error) error {
	var apiError smithy.APIError
	if errors.As(err, &apiError) && (apiError.ErrorCode() == "PreconditionFailed" || apiError.ErrorCode() == "ConditionalRequestConflict") {
		return fmt.Errorf("%w: 源文件已改变或目标文件已存在，未覆盖对象", store.ErrUploadSessionConflict)
	}
	return err
}

// reserveMCPObjectWrite is called under uploadOperationLock. Its one Class A write
// authorization is consumed atomically, so the following PUT uses the
// direct client without charging a second time. Confirmation HEAD remains a
// reserved Class B read through completeUploadTask.
func reserveMCPObjectWrite(actor UploadActor, account *store.Account, key string, size int64, contentType string, readReservations int64) (*store.UploadSession, error) {
	now := time.Now().UTC()
	session := &store.UploadSession{
		ID: uuid.NewString(), UserID: actor.UserID, AuthType: actor.AuthType, TokenID: actor.TokenID, AuthVersion: actor.AuthVersion,
		AccountID: account.ID, FileKey: key, Filename: path.Base(strings.TrimSuffix(key, "/")), ExpectedSize: size,
		ContentType: contentType, RequestedPath: path.Dir(key), ExpirationDays: 0,
		ReservedClassBOps: readReservations,
		CreatedAt:         now.Format(time.RFC3339), UploadExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
	}
	if err := store.CreateUploadSession(session); err != nil {
		return nil, err
	}
	return session, nil
}

func mcpSourceExpiration(accountID, key string) (string, error) {
	for _, expiration := range store.GetFileExpirations() {
		if expiration.AccountID != accountID || expiration.FileKey != key {
			continue
		}
		deadline, err := time.Parse(time.RFC3339, expiration.ExpiresAt)
		if err != nil || !deadline.After(time.Now()) {
			return "", ErrMCPFileExpired
		}
		return expiration.ExpiresAt, nil
	}
	return "", nil
}

func prepareMCPCopy(ctx context.Context, actor UploadActor, request MCPFileOperationRequest) (*store.Account, *store.UploadSession, *s3.HeadObjectOutput, error) {
	uploadOperationLock.Lock()
	defer uploadOperationLock.Unlock()
	account, err := mcpOperationAccount(actor, request.IDGroup)
	if err != nil {
		return nil, nil, nil, err
	}
	if IsPendingUploadKey(account.ID, request.Key) || IsPendingUploadKey(account.ID, request.DestinationKey) {
		return nil, nil, nil, store.ErrUploadSessionConflict
	}
	client := getS3Client(account)
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(request.Key)})
	if isMissingR2Object(err) {
		return nil, nil, nil, ErrMCPFileNotFound
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("读取源文件属性失败: %w", err)
	}
	size, contentType := aws.ToInt64(head.ContentLength), aws.ToString(head.ContentType)
	if size < 0 || size > store.MaxUploadFileSize || aws.ToString(head.ETag) == "" {
		return nil, nil, nil, fmt.Errorf("%w: 复制文件不能超过 5 GiB，且必须具有有效 ETag", store.ErrInvalidUploadSession)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, err := chooseUploadAccount(actor, account.ID, size); err != nil {
		return nil, nil, nil, err
	}
	if _, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(request.DestinationKey)}); err == nil {
		return nil, nil, nil, fmt.Errorf("%w: 目标文件已存在，未覆盖原文件", store.ErrUploadSessionConflict)
	} else if !isMissingR2Object(err) {
		return nil, nil, nil, fmt.Errorf("检查目标文件失败: %w", err)
	}
	session, err := reserveMCPObjectWrite(actor, account, request.DestinationKey, size, contentType, 2)
	return account, session, head, err
}

func CopyMCPFile(ctx context.Context, actor UploadActor, request MCPFileOperationRequest, move bool) (*MCPFileOperationResult, error) {
	permissions := []string{"read", "write"}
	if move {
		permissions = append(permissions, "delete")
	}
	if err := authorizeMCPFileActor(actor, permissions...); err != nil {
		return nil, err
	}
	var err error
	request.Key, err = validateMCPFileKey(request.Key, false)
	if err != nil {
		return nil, err
	}
	request.DestinationKey, err = validateMCPFileKey(request.DestinationKey, false)
	if err != nil {
		return nil, err
	}
	if request.Key == request.DestinationKey {
		return nil, fmt.Errorf("%w: 源路径和目标路径不能相同", store.ErrInvalidUploadSession)
	}
	// Keep scheduled cleanup and deadline edits from racing the source transfer.
	expirationOperationLock.Lock()
	defer expirationOperationLock.Unlock()
	expiresAt, err := mcpSourceExpiration(request.IDGroup, request.Key)
	if err != nil {
		return nil, err
	}
	account, session, source, err := prepareMCPCopy(ctx, actor, request)
	if err != nil {
		return nil, err
	}
	result := &MCPFileOperationResult{SourceKey: request.Key, ExpiresAt: expiresAt,
		Destination: &UploadResult{ID: account.ID, AccountName: account.Name, Key: session.FileKey, Size: session.ExpectedSize, URL: buildPublicURL(account.PublicDomain, session.FileKey)}}
	// R2 does not guarantee destination conditions on CopyObject. Read the
	// unchanged source into a bounded temporary spool, then use the same proven
	// conditional PutObject contract as ordinary uploads to refuse overwrite.
	spool, err := os.CreateTemp("", "fileflow-copy-*")
	if err != nil {
		return result, fmt.Errorf("创建复制临时文件失败: %w", err)
	}
	defer func() {
		_ = spool.Close()
		_ = os.Remove(spool.Name())
	}()
	if err := store.ConsumeUploadSessionRead(session.ID); err != nil {
		return result, err
	}
	response, err := getUploadS3Client(account).GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(request.Key), IfMatch: source.ETag})
	if err != nil {
		return result, fmt.Errorf("读取源文件失败: %w", mcpOperationConflict(err))
	}
	if aws.ToInt64(response.ContentLength) != session.ExpectedSize || aws.ToString(response.ETag) != aws.ToString(source.ETag) {
		_ = response.Body.Close()
		return result, fmt.Errorf("%w: 源文件大小或 ETag 已改变", store.ErrUploadSessionConflict)
	}
	written, readErr := io.Copy(spool, io.LimitReader(response.Body, session.ExpectedSize+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		return result, fmt.Errorf("缓存源文件失败: %w", errors.Join(readErr, closeErr))
	}
	if written != session.ExpectedSize {
		return result, fmt.Errorf("%w: 源文件内容大小与声明不符", store.ErrUploadSessionConflict)
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		return result, fmt.Errorf("读取复制临时文件失败: %w", err)
	}
	if err := authorizeMCPFileActor(actor, permissions...); err != nil {
		return result, err
	}
	_, err = getUploadS3Client(account).PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(account.BucketName), Key: aws.String(session.FileKey), Body: spool,
		ContentLength: aws.Int64(session.ExpectedSize), ContentType: aws.String(session.ContentType), ContentMD5: emptyUploadMD5(session.ExpectedSize),
		IfNoneMatch: aws.String("*"), Metadata: map[string]string{"fileflow-upload-id": session.ID},
	})
	if err != nil {
		return result, fmt.Errorf("写入目标文件失败: %w", mcpOperationConflict(err))
	}
	completed, _, err := completeUploadTask(ctx, actor, session.ID)
	if err != nil {
		return result, fmt.Errorf("目标文件确认失败，文件任务将由系统检查清理；源文件未删除: %w", err)
	}
	result.Destination = completed
	result.DestinationConfirmed = true
	if err := store.SetCopiedFileExpiration(account.ID, request.Key, session.FileKey); err != nil {
		return result, fmt.Errorf("目标文件已复制，但保留到期时间失败；源文件未删除: %w", err)
	}
	if !move {
		return result, nil
	}
	if err := authorizeMCPFileActor(actor, permissions...); err != nil {
		return result, fmt.Errorf("目标文件已复制，但当前成员已无删除权限；源文件未删除: %w", err)
	}
	uploadOperationLock.Lock()
	defer uploadOperationLock.Unlock()
	currentSource, err := getS3Client(account).HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(request.Key)})
	if err != nil || aws.ToString(currentSource.ETag) != aws.ToString(source.ETag) {
		if err == nil {
			err = store.ErrUploadSessionConflict
		}
		return result, fmt.Errorf("目标文件已复制，但源文件已改变或无法确认；源文件未删除: %w", err)
	}
	// R2 lacks conditional deletion. Recheck immediately before DELETE while the
	// operation lock is held; an independent R2 client can still race this window.
	if _, err := getS3Client(account).DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(request.Key)}); err != nil {
		return result, fmt.Errorf("目标文件已复制，删除源文件失败；两份文件均需检查: %w", mcpOperationConflict(err))
	}
	result.SourceDeleted = true
	if err := store.DeleteFileExpiration(account.ID, request.Key); err != nil {
		return result, fmt.Errorf("源文件已删除，但删除旧计划失败: %w", err)
	}
	return result, nil
}

func CreateMCPDirectory(ctx context.Context, actor UploadActor, request MCPFileOperationRequest) (*MCPFileOperationResult, error) {
	if err := authorizeMCPFileActor(actor, "write"); err != nil {
		return nil, err
	}
	if request.DestinationKey != "" {
		return nil, store.ErrInvalidUploadSession
	}
	key, err := validateMCPFileKey(request.Key, true)
	if err != nil {
		return nil, err
	}
	account, session, err := func() (*store.Account, *store.UploadSession, error) {
		uploadOperationLock.Lock()
		defer uploadOperationLock.Unlock()
		account, err := mcpOperationAccount(actor, request.IDGroup)
		if err != nil {
			return nil, nil, err
		}
		if _, err := chooseUploadAccount(actor, account.ID, 0); err != nil {
			return nil, nil, err
		}
		if IsPendingUploadKey(account.ID, key) {
			return nil, nil, store.ErrUploadSessionConflict
		}
		if _, err := getS3Client(account).HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(key)}); err == nil {
			return nil, nil, fmt.Errorf("%w: 目录标记已存在", store.ErrUploadSessionConflict)
		} else if !isMissingR2Object(err) {
			return nil, nil, err
		}
		session, err := reserveMCPObjectWrite(actor, account, key, 0, "application/x-directory", 1)
		return account, session, err
	}()
	if err != nil {
		return nil, err
	}
	result := &MCPFileOperationResult{Directory: true,
		Destination: &UploadResult{ID: account.ID, AccountName: account.Name, Key: key, Size: 0, URL: buildPublicURL(account.PublicDomain, key)}}
	_, err = getUploadS3Client(account).PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(account.BucketName), Key: aws.String(key), Body: bytes.NewReader(nil),
		ContentLength: aws.Int64(0), ContentType: aws.String(session.ContentType), ContentMD5: emptyUploadMD5(0),
		IfNoneMatch: aws.String("*"), Metadata: map[string]string{"fileflow-upload-id": session.ID},
	})
	if err != nil {
		return result, fmt.Errorf("创建目录失败: %w", mcpOperationConflict(err))
	}
	completed, _, err := completeUploadTask(ctx, actor, session.ID)
	if err != nil {
		return result, fmt.Errorf("目录标记确认失败，文件任务将由系统检查清理: %w", err)
	}
	result.Destination = completed
	result.DestinationConfirmed = true
	return result, nil
}
