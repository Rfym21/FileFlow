package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"
	"time"

	"fileflow/server/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const maxPreviewTextBytes = 256 * 1024

var (
	ErrInvalidFilePreview     = errors.New("预览参数无效")
	ErrPreviewAccountNotFound = errors.New("预览账户不存在")
)

type FilePreview struct {
	URL          string            `json:"url"`
	DownloadURL  string            `json:"downloadUrl"`
	ContentType  string            `json:"contentType"`
	Kind         string            `json:"kind"`
	Text         *string           `json:"text,omitempty"`
	Truncated    bool              `json:"truncated,omitempty"`
	Entries      []ZIPPreviewEntry `json:"entries,omitempty"`
	EntryCount   int               `json:"entryCount,omitempty"`
	ArchiveError string            `json:"archiveError,omitempty"`
}

type previewFormat struct {
	kind        string
	contentType string
}

var previewMediaFormats = map[string]previewFormat{
	".jpg": {"image", "image/jpeg"}, ".jpeg": {"image", "image/jpeg"},
	".png": {"image", "image/png"}, ".apng": {"image", "image/apng"},
	".gif": {"image", "image/gif"}, ".webp": {"image", "image/webp"},
	".avif": {"image", "image/avif"}, ".bmp": {"image", "image/bmp"},
	".ico": {"image", "image/x-icon"},
	".mp4": {"video", "video/mp4"}, ".m4v": {"video", "video/mp4"},
	".webm": {"video", "video/webm"}, ".mov": {"video", "video/quicktime"},
	".ogv": {"video", "video/ogg"},
	".mp3": {"audio", "audio/mpeg"}, ".wav": {"audio", "audio/wav"},
	".ogg": {"audio", "audio/ogg"}, ".oga": {"audio", "audio/ogg"},
	".m4a": {"audio", "audio/mp4"}, ".aac": {"audio", "audio/aac"},
	".flac": {"audio", "audio/flac"}, ".opus": {"audio", "audio/ogg"},
	".weba": {"audio", "audio/webm"},
	".pdf":  {"pdf", "application/pdf"},
	".zip":  {"zip", "application/zip"},
}

var previewTextExtensions = map[string]struct{}{
	".txt": {}, ".log": {}, ".md": {}, ".markdown": {}, ".json": {}, ".jsonl": {},
	".csv": {}, ".tsv": {}, ".xml": {}, ".html": {}, ".htm": {}, ".svg": {},
	".yaml": {}, ".yml": {}, ".toml": {}, ".ini": {}, ".conf": {}, ".cfg": {},
	".env": {}, ".gitignore": {}, ".go": {}, ".js": {}, ".jsx": {}, ".ts": {},
	".tsx": {}, ".css": {}, ".scss": {}, ".less": {}, ".py": {}, ".rb": {},
	".rs": {}, ".java": {}, ".c": {}, ".h": {}, ".cpp": {}, ".hpp": {},
	".cs": {}, ".php": {}, ".sh": {}, ".bash": {}, ".ps1": {}, ".sql": {},
	".r": {}, ".vue": {}, ".svelte": {},
}

func classifyPreview(key string) previewFormat {
	extension := strings.ToLower(path.Ext(key))
	if format, ok := previewMediaFormats[extension]; ok {
		return format
	}
	_, textExtension := previewTextExtensions[extension]
	base := strings.ToLower(path.Base(key))
	if textExtension || base == "dockerfile" || base == "makefile" || base == "readme" || base == "license" {
		// HTML、SVG 和源码均按纯文本返回，避免浏览器执行文件内容。
		contentType := "text/plain; charset=utf-8"
		if extension == ".json" {
			contentType = "application/json"
		} else if extension == ".csv" {
			contentType = "text/csv; charset=utf-8"
		} else if extension == ".tsv" {
			contentType = "text/tab-separated-values; charset=utf-8"
		}
		return previewFormat{"text", contentType}
	}
	return previewFormat{"unsupported", "application/octet-stream"}
}

func classifyPreviewMIME(contentType string) previewFormat {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return previewFormat{"unsupported", "application/octet-stream"}
	}
	for _, format := range previewMediaFormats {
		if format.contentType == mediaType {
			return format
		}
	}
	switch mediaType {
	case "image/vnd.microsoft.icon":
		return previewFormat{"image", "image/x-icon"}
	case "audio/x-wav":
		return previewFormat{"audio", "audio/wav"}
	case "audio/x-flac":
		return previewFormat{"audio", "audio/flac"}
	case "application/x-zip-compressed":
		return previewFormat{"zip", "application/zip"}
	case "application/json", "application/ld+json":
		return previewFormat{"text", "application/json"}
	case "text/csv":
		return previewFormat{"text", "text/csv; charset=utf-8"}
	case "text/tab-separated-values":
		return previewFormat{"text", "text/tab-separated-values; charset=utf-8"}
	case "text/plain", "text/markdown", "text/html", "image/svg+xml", "text/xml", "application/xml",
		"text/javascript", "application/javascript", "application/ecmascript", "text/css",
		"application/yaml", "application/x-yaml", "text/yaml", "text/x-python", "text/x-go", "text/x-c":
		// 只允许明确列出的文本类型；HTML/SVG 即使来自元数据也按纯文本展示。
		return previewFormat{"text", "text/plain; charset=utf-8"}
	}
	return previewFormat{"unsupported", "application/octet-stream"}
}

// GetFilePreview 使用账户凭据生成短期 R2 链接；文本内容仅从该账户读取。
func GetFilePreview(ctx context.Context, accountID, key string) (*FilePreview, error) {
	if IsPendingUploadKey(accountID, key) {
		return nil, fmt.Errorf("%w: 文件上传尚未确认", ErrInvalidFilePreview)
	}
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(key) == "" || strings.HasSuffix(key, "/") {
		return nil, fmt.Errorf("%w: 请选择一个文件并提供账户 ID", ErrInvalidFilePreview)
	}
	account, err := store.GetAccountByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPreviewAccountNotFound, accountID)
	}
	format := classifyPreview(key)
	client := getS3Client(account)
	var metadata *s3.HeadObjectOutput
	if format.kind == "unsupported" {
		metadata, err = client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(key)})
		if err != nil {
			return nil, fmt.Errorf("读取文件类型失败: %w", err)
		}
		format = classifyPreviewMIME(aws.ToString(metadata.ContentType))
	}
	presigner := s3.NewPresignClient(client)
	if err := ChargeAccountOperations(account.ID, 0, 2); err != nil {
		return nil, err
	}
	input := &s3.GetObjectInput{
		Bucket:                     aws.String(account.BucketName),
		Key:                        aws.String(key),
		ResponseContentType:        aws.String(format.contentType),
		ResponseContentDisposition: aws.String(mime.FormatMediaType("inline", map[string]string{"filename": path.Base(key)})),
	}
	signed, err := presigner.PresignGetObject(ctx, input, func(options *s3.PresignOptions) {
		options.Expires = 15 * time.Minute
	})
	if err != nil {
		return nil, fmt.Errorf("生成预览链接失败: %w", err)
	}
	input.ResponseContentDisposition = aws.String(mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(key)}))
	download, err := presigner.PresignGetObject(ctx, input, func(options *s3.PresignOptions) {
		options.Expires = 15 * time.Minute
	})
	if err != nil {
		return nil, fmt.Errorf("生成下载链接失败: %w", err)
	}
	result := &FilePreview{URL: signed.URL, DownloadURL: download.URL, ContentType: format.contentType, Kind: format.kind}
	if format.kind == "zip" {
		if metadata == nil {
			metadata, err = client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(account.BucketName), Key: aws.String(key)})
		}
		if err == nil {
			var directory zipPreviewDirectory
			directory, err = readZIPPreview(ctx, client, account.BucketName, key, metadata)
			result.Entries, result.EntryCount, result.Truncated = directory.entries, directory.entryCount, directory.truncated
		}
		if err != nil {
			result.ArchiveError = "ZIP 目录预览失败: " + err.Error()
		}
		return result, nil
	}
	if format.kind != "text" {
		return result, nil
	}

	// 多取一个字节判断是否截断；即使存储忽略 Range，也限制读取量。
	objectInput := &s3.GetObjectInput{Bucket: input.Bucket, Key: input.Key, Range: aws.String(fmt.Sprintf("bytes=0-%d", maxPreviewTextBytes))}
	object, err := client.GetObject(ctx, objectInput)
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) && apiError.ErrorCode() == "InvalidRange" {
			// 空文本对象可能拒绝 Range；回退请求仍通过 LimitReader 限制读取。
			objectInput.Range = nil
			object, err = client.GetObject(ctx, objectInput)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("读取文件预览失败: %w", err)
	}
	if object.Body == nil {
		return nil, fmt.Errorf("读取文件预览失败: 存储未返回文件内容")
	}
	defer object.Body.Close()
	content, err := io.ReadAll(io.LimitReader(object.Body, maxPreviewTextBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取文件预览失败: %w", err)
	}
	result.Truncated = len(content) > maxPreviewTextBytes
	if result.Truncated {
		content = content[:maxPreviewTextBytes]
	}
	text := string(content)
	result.Text = &text
	return result, nil
}
