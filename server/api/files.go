package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fileflow/server/service"
	"fileflow/server/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DownloadResult URL 下载结果
type DownloadResult struct {
	Body        io.ReadCloser
	Size        int64
	ContentType string
	Ext         string
}

// downloadFromURL 从 URL 下载文件
func downloadFromURL(rawURL string) (*DownloadResult, error) {
	// 验证 URL 格式
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("无效的 URL，必须以 http:// 或 https:// 开头")
	}

	// 创建 HTTP 客户端（120秒超时）
	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	// 发起 GET 请求
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("下载失败，HTTP 状态码: %d", resp.StatusCode)
	}

	// 获取 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 确定文件扩展名（优先 URL 路径）
	ext := getExtFromURL(rawURL)
	if ext == "" {
		ext = getExtFromContentType(contentType)
	}

	return &DownloadResult{
		Body:        resp.Body,
		Size:        resp.ContentLength,
		ContentType: contentType,
		Ext:         ext,
	}, nil
}

// getExtFromURL 从 URL 路径提取扩展名
func getExtFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return filepath.Ext(parsed.Path)
}

// getExtFromContentType 从 Content-Type 推断扩展名
func getExtFromContentType(contentType string) string {
	// 移除参数部分（如 charset）
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	mimeMap := map[string]string{
		"image/jpeg":              ".jpg",
		"image/png":               ".png",
		"image/gif":               ".gif",
		"image/webp":              ".webp",
		"image/svg+xml":           ".svg",
		"image/x-icon":            ".ico",
		"image/bmp":               ".bmp",
		"video/mp4":               ".mp4",
		"video/webm":              ".webm",
		"video/quicktime":         ".mov",
		"audio/mpeg":              ".mp3",
		"audio/wav":               ".wav",
		"audio/ogg":               ".ogg",
		"application/pdf":         ".pdf",
		"application/zip":         ".zip",
		"application/x-gzip":      ".gz",
		"application/x-tar":       ".tar",
		"text/plain":              ".txt",
		"text/html":               ".html",
		"text/css":                ".css",
		"application/javascript":  ".js",
		"application/json":        ".json",
		"application/xml":         ".xml",
		"application/octet-stream": "",
	}

	if ext, ok := mimeMap[contentType]; ok {
		return ext
	}
	return ""
}

// generateFilenameFromURL 从 URL 生成文件名
func generateFilenameFromURL(rawURL string, ext string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "download" + ext
	}

	// 尝试从路径获取文件名
	basename := filepath.Base(parsed.Path)
	if basename != "" && basename != "/" && basename != "." {
		return basename
	}

	// 使用域名作为文件名
	hostname := parsed.Hostname()
	if hostname != "" {
		return hostname + ext
	}

	return "download" + ext
}

// GetFiles 获取文件列表（懒加载+分页）
func GetFiles(c *gin.Context) {
	idGroupStr := c.Query("idGroup")
	prefix := c.Query("prefix")
	cursor := c.Query("cursor")
	limitStr := c.DefaultQuery("limit", "50")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// 解析 idGroup（逗号分隔）
	var idGroup []string
	if idGroupStr != "" {
		for _, id := range strings.Split(idGroupStr, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				idGroup = append(idGroup, id)
			}
		}
	}

	if len(idGroup) > 0 {
		// 获取指定账户组的文件
		result, err := service.ListAccountsFilesByIDs(c.Request.Context(), idGroup, prefix, cursor, int32(limit))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	} else {
		// 获取所有账户的文件
		result, err := service.ListAllAccountsFiles(c.Request.Context(), prefix, cursor, int32(limit))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// Upload 上传文件（可指定账户，不指定则智能选择）
// 支持两种方式：file 表单字段上传文件，或 url 参数从远程下载后上传
func Upload(c *gin.Context) {
	// 获取 url 参数
	urlParam := c.PostForm("url")
	// 检查是否有 file 表单字段
	file, header, fileErr := c.Request.FormFile("file")
	hasFile := fileErr == nil

	// 互斥校验：url 和 file 只能二选一
	if urlParam != "" && hasFile {
		file.Close()
		c.JSON(http.StatusBadRequest, gin.H{"error": "url 和 file 参数不能同时提供，请选择其一"})
		return
	}

	if !hasFile && urlParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 file 或 url 参数"})
		return
	}

	// 指定账户ID（可选，取第一个）
	idGroup := c.PostForm("idGroup")
	accountID := getFirstID(idGroup)

	// 解析到期天数（可选，默认使用系统设置）
	expirationDaysStr := c.PostForm("expirationDays")
	var expirationDays int
	if expirationDaysStr != "" {
		expirationDays, _ = strconv.Atoi(expirationDaysStr)
		// -1 表示使用默认设置，0 表示永久
		if expirationDays < -1 {
			expirationDays = -1
		}
	} else {
		expirationDays = -1 // 使用默认设置
	}

	// 解析实际到期天数（用于 ImgBB 判断）
	actualExpirationDays := expirationDays
	if actualExpirationDays == -1 {
		settings := store.GetSettings()
		actualExpirationDays = settings.DefaultExpirationDays
	}

	// 检查是否应该使用 ImgBB
	settings := store.GetSettings()
	useImgBB := false
	imgbbExpirationDays := actualExpirationDays
	var downloadResult *DownloadResult

	if settings.ImgBBEnabled && settings.ImgBBPriority {
		var fileContentType string
		var fileExt string

		if hasFile {
			// 直接文件上传
			fileContentType = header.Header.Get("Content-Type")
			fileExt = filepath.Ext(header.Filename)
		} else if urlParam != "" {
			// URL 上传：简单判断是否为图片 URL（从扩展名）
			fileExt = getExtFromURL(urlParam)
			// 如果扩展名是图片类型，直接使用 ImgBB URL 上传
			if isImageExtension(fileExt) {
				fileContentType = "image/" + strings.TrimPrefix(fileExt, ".")
			}
		}

		// 检查是否为图片类型
		isImage := strings.HasPrefix(fileContentType, "image/") ||
			(fileContentType == "" && isImageExtension(fileExt))

		if isImage {
			// 优先使用 ImgBB，检查是否支持该到期时间
			if closestDays, ok := service.FindClosestImgBBExpiration(actualExpirationDays); ok {
				useImgBB = true
				imgbbExpirationDays = closestDays // 使用最接近的 ImgBB 支持时间
			}
		}
	}

	// 如果使用 ImgBB
	if useImgBB {
		var imgbbResult *service.ImgBBUploadResult
		var err error
		var imgbbFileName string
		var imgbbFileSize int64

		if hasFile {
			// 直接文件上传
			defer file.Close()
			imgbbFileName = header.Filename
			imgbbFileSize = header.Size
			imgbbResult, err = service.UploadToImgBB(file, imgbbExpirationDays, 60*time.Second)
		} else if urlParam != "" {
			// URL 上传：让 ImgBB 直接从 URL 下载
			imgbbFileName = generateFilenameFromURL(urlParam, getExtFromURL(urlParam))
			imgbbFileSize = 0 // URL 上传暂时无法获取大小
			imgbbResult, err = service.UploadURLToImgBB(urlParam, imgbbExpirationDays, 60*time.Second)
		}

		if err != nil {
			// ImgBB 失败，回退到 R2
			fmt.Printf("[Upload] ImgBB 上传失败，回退到 R2: %v\n", err)
			if hasFile {
				// 需要重新获取文件（因为已被读取）
				file, header, fileErr = c.Request.FormFile("file")
				if fileErr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "ImgBB 失败且无法回退到 R2: " + fileErr.Error()})
					return
				}
				defer file.Close()
			}
			// URL 上传回退：继续到 R2 上传逻辑（不需要重新下载，后面会处理）
		} else {
			// ImgBB 上传成功，记录到数据库
			imgbbFile := store.ImgBBFile{
				ID:         uuid.New().String(),
				FileName:   imgbbFileName,
				URL:        imgbbResult.DirectURL,
				DeleteURL:  imgbbResult.DeleteURL,
				Size:       imgbbFileSize,
				UploadedAt: time.Now().Format(time.RFC3339),
			}
			if err := store.AddImgBBFile(imgbbFile); err != nil {
				fmt.Printf("[Upload] 保存 ImgBB 文件记录失败: %v\n", err)
			}

			c.JSON(http.StatusOK, gin.H{
				"id":        "imgbb",
				"accountId": "imgbb",
				"key":       imgbbResult.DirectURL,
				"url":       imgbbResult.DirectURL,
				"deleteUrl": imgbbResult.DeleteURL,
				"size":      imgbbFileSize,
				"provider":  "imgbb",
			})
			return
		}
	}

	// R2 上传逻辑
	var fileReader io.Reader
	var fileSize int64
	var contentType string
	var ext string

	if urlParam != "" {
		// 从 URL 下载文件（如果还没下载过）
		if downloadResult == nil {
			var err error
			downloadResult, err = downloadFromURL(urlParam)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		defer downloadResult.Body.Close()

		fileReader = downloadResult.Body
		fileSize = downloadResult.Size
		contentType = downloadResult.ContentType
		ext = downloadResult.Ext
	} else if hasFile {
		// file 表单处理逻辑
		if !useImgBB {
			defer file.Close()
		}

		fileReader = file
		fileSize = header.Size
		contentType = header.Header.Get("Content-Type")
		ext = filepath.Ext(header.Filename)
	}

	// 生成文件路径（始终使用 uuid+时间戳 重命名）
	customPath := c.PostForm("path")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	newFilename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().UnixMilli(), ext)

	var key string
	if customPath != "" {
		// 自定义路径 + 重命名后的文件名
		customPath = strings.TrimPrefix(customPath, "/")
		customPath = strings.TrimSuffix(customPath, "/")
		key = fmt.Sprintf("%s/%s", customPath, newFilename)
	} else {
		// 默认按日期组织
		key = fmt.Sprintf("%s/%s", time.Now().Format("2006/01/02"), newFilename)
	}

	var result *service.UploadResult
	var err error
	if accountID != "" {
		// 上传到指定账户（前端上传检查 client_upload 权限）
		result, err = service.UploadToAccountForClient(c.Request.Context(), accountID, key, fileReader, contentType)
	} else {
		// 智能上传（自动选择具有 client_upload 权限的账户）
		result, err = service.SmartUploadForClient(c.Request.Context(), key, fileReader, fileSize, contentType)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建文件到期记录
	if expirationDays == -1 {
		// 使用系统默认设置
		settings := store.GetSettings()
		expirationDays = settings.DefaultExpirationDays
	}
	if expirationDays > 0 {
		// expirationDays > 0 才创建到期记录，0 表示永久不过期
		if err := service.CreateFileExpirationRecord(result.ID, result.Key, expirationDays); err != nil {
			// 到期记录创建失败不影响上传结果，仅记录日志
			fmt.Printf("[Upload] 创建文件到期记录失败: %v\n", err)
		}
	}

	c.JSON(http.StatusOK, result)
}

// DeleteFile 删除文件
func DeleteFile(c *gin.Context) {
	idGroup := c.Query("idGroup")
	accountID := getFirstID(idGroup)
	key := c.Query("key")

	if accountID == "" || key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 idGroup 或 key 参数"})
		return
	}

	if err := service.DeleteFile(c.Request.Context(), accountID, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 删除对应的到期记录（如果存在）
	service.DeleteFileExpirationRecord(accountID, key)

	// 如果是 ImgBB 文件，删除数据库记录
	if accountID == "imgbb" {
		// 通过 deleteUrl 查找并删除记录
		files := store.GetImgBBFiles()
		for _, f := range files {
			if f.DeleteURL == key {
				if err := store.DeleteImgBBFile(f.ID); err != nil {
					fmt.Printf("[DeleteFile] 删除 ImgBB 文件记录失败: %v\n", err)
				}
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetLink 获取文件直链
func GetLink(c *gin.Context) {
	idGroup := c.Query("idGroup")
	accountID := getFirstID(idGroup)
	key := c.Query("key")

	if accountID == "" || key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 idGroup 或 key 参数"})
		return
	}

	url, err := service.GetFileLink(accountID, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// GetImgBBFiles 获取 ImgBB 文件列表
func GetImgBBFiles(c *gin.Context) {
	files := store.GetImgBBFiles()
	c.JSON(http.StatusOK, files)
}

// getFirstID 从逗号分隔的 ID 列表中获取第一个 ID
func getFirstID(idGroup string) string {
	if idGroup == "" {
		return ""
	}
	ids := strings.Split(idGroup, ",")
	if len(ids) > 0 {
		return strings.TrimSpace(ids[0])
	}
	return ""
}

// isImageExtension 检查文件扩展名是否为图片类型
func isImageExtension(ext string) bool {
	ext = strings.ToLower(ext)
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".ico"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// ClearBucket 清空账户的存储桶
func ClearBucket(c *gin.Context) {
	accountID := c.Param("id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少账户 ID"})
		return
	}

	if err := service.ClearBucket(c.Request.Context(), accountID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "清空成功"})
}

// DeleteOldFilesRequest 删除旧文件请求
type DeleteOldFilesRequest struct {
	AccountIDs []string `json:"accountIds" binding:"required"`
	BeforeDate string   `json:"beforeDate" binding:"required"`
}

// DeleteOldFiles 批量删除指定账户中早于指定时间的文件
func DeleteOldFiles(c *gin.Context) {
	var req DeleteOldFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if len(req.AccountIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择至少一个账户"})
		return
	}

	// 解析日期
	beforeDate, err := time.Parse("2006-01-02", req.BeforeDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期格式错误，请使用 YYYY-MM-DD 格式"})
		return
	}

	// 设置为当天结束时间（23:59:59）
	beforeDate = beforeDate.Add(24*time.Hour - time.Second)

	results := service.DeleteOldFilesMultiple(c.Request.Context(), req.AccountIDs, beforeDate)

	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

// FileExpirationResponse 文件到期记录响应（包含账户名）
type FileExpirationResponse struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	FileKey     string `json:"fileKey"`
	ExpiresAt   string `json:"expiresAt"`
	CreatedAt   string `json:"createdAt"`
}

// GetFileExpirations 获取文件到期列表
func GetFileExpirations(c *gin.Context) {
	expirations := store.GetFileExpirations()

	// 构建账户 ID -> 名称映射
	accounts := store.GetAccounts()
	accountMap := make(map[string]string)
	for _, acc := range accounts {
		accountMap[acc.ID] = acc.Name
	}

	// 转换为响应格式
	result := make([]FileExpirationResponse, 0, len(expirations))
	for _, exp := range expirations {
		accountName := accountMap[exp.AccountID]
		if accountName == "" {
			accountName = "未知账户"
		}
		result = append(result, FileExpirationResponse{
			ID:          exp.ID,
			AccountID:   exp.AccountID,
			AccountName: accountName,
			FileKey:     exp.FileKey,
			ExpiresAt:   exp.ExpiresAt,
			CreatedAt:   exp.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"expirations": result,
		"total":       len(result),
	})
}

// DeleteFileExpirationByID 删除单个到期记录（同时删除文件）
func DeleteFileExpirationByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少记录 ID"})
		return
	}

	// 查找到期记录
	expirations := store.GetFileExpirations()
	var target *store.FileExpiration
	for _, exp := range expirations {
		if exp.ID == id {
			target = &exp
			break
		}
	}

	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}

	// 删除 S3 文件
	if err := service.DeleteFile(c.Request.Context(), target.AccountID, target.FileKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文件失败: " + err.Error()})
		return
	}

	// 删除到期记录
	if err := store.DeleteFileExpirationByID(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除记录失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
