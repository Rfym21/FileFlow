package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	ImgBBHomeURL = "https://imgbb.com/"
	ImgBBJSONURL = "https://imgbb.com/json"
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
)

// ImgBB 到期时间映射（天数 -> ISO 8601 duration）
var ImgBBExpirations = map[int]string{
	0:  "",     // 永久
	1:  "P1D",  // 1天
	2:  "P2D",  // 2天
	3:  "P3D",  // 3天
	4:  "P4D",  // 4天
	5:  "P5D",  // 5天
	6:  "P6D",  // 6天
	7:  "P1W",  // 1周
	14: "P2W",  // 2周
	21: "P3W",  // 3周
	30: "P1M",  // 1月
	60: "P2M",  // 2月
	90: "P3M",  // 3月
	120: "P4M", // 4月
	150: "P5M", // 5月
	180: "P6M", // 6月
}

// ImgBBGuestAuth ImgBB 访客认证信息
type ImgBBGuestAuth struct {
	AuthToken string
	PHPSESSID string
}

// ImgBBUploadResult ImgBB 上传结果
type ImgBBUploadResult struct {
	DeleteURL string `json:"deleteUrl"` // 删除链接
	DirectURL string `json:"directUrl"` // 直接访问链接
}

// GetImgBBGuestAuth 获取 ImgBB 访客认证令牌
func GetImgBBGuestAuth(timeout time.Duration) (*ImgBBGuestAuth, error) {
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequest("GET", ImgBBHomeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取 HTML
	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	html := string(htmlBytes)

	// 提取 auth_token
	re := regexp.MustCompile(`PF\.obj\.config\.auth_token\s*=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("无法从 ImgBB 主页提取 auth_token")
	}

	// 提取 PHPSESSID
	var phpsessid string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "PHPSESSID" {
			phpsessid = cookie.Value
			break
		}
	}

	return &ImgBBGuestAuth{
		AuthToken: matches[1],
		PHPSESSID: phpsessid,
	}, nil
}

// UploadToImgBB 上传文件到 ImgBB
func UploadToImgBB(fileReader io.Reader, expirationDays int, timeout time.Duration) (*ImgBBUploadResult, error) {
	// 获取访客认证
	auth, err := GetImgBBGuestAuth(timeout)
	if err != nil {
		return nil, err
	}

	// 确定到期时间
	expiration, ok := ImgBBExpirations[expirationDays]
	if !ok {
		// 不支持的天数，返回错误
		return nil, fmt.Errorf("ImgBB 不支持 %d 天的到期时间，请使用 R2 存储", expirationDays)
	}

	// 读取文件内容到内存
	fileData, err := io.ReadAll(fileReader)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 构造 multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加表单字段
	writer.WriteField("type", "file")
	writer.WriteField("action", "upload")
	writer.WriteField("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	writer.WriteField("auth_token", auth.AuthToken)
	if expiration != "" {
		writer.WriteField("expiration", expiration)
	}

	// 添加文件
	part, err := writer.CreateFormFile("source", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("创建文件字段失败: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("写入文件数据失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 writer 失败: %w", err)
	}

	// 创建上传请求
	req, err := http.NewRequest("POST", ImgBBJSONURL, body)
	if err != nil {
		return nil, fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://imgbb.com")
	req.Header.Set("Referer", ImgBBHomeURL)
	if auth.PHPSESSID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "PHPSESSID",
			Value: auth.PHPSESSID,
		})
	}

	// 发送请求
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取上传响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ImgBB 上传失败，HTTP 状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	// 检查状态码
	statusCode, _ := result["status_code"].(float64)
	if statusCode != 200 {
		errorMsg := "未知错误"
		if errorData, ok := result["error"].(map[string]interface{}); ok {
			if msg, ok := errorData["message"].(string); ok {
				errorMsg = msg
			}
		}
		return nil, fmt.Errorf("ImgBB 上传失败: %s", errorMsg)
	}

	// 提取链接
	imageData, ok := result["image"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("ImgBB 响应中缺少 image 数据")
	}

	deleteURL, _ := imageData["delete_url"].(string)
	directURL, _ := imageData["url"].(string)

	if deleteURL == "" || directURL == "" {
		return nil, fmt.Errorf("ImgBB 响应中缺少必要的链接信息")
	}

	return &ImgBBUploadResult{
		DeleteURL: deleteURL,
		DirectURL: directURL,
	}, nil
}

// UploadURLToImgBB 从 URL 上传到 ImgBB
func UploadURLToImgBB(imageURL string, expirationDays int, timeout time.Duration) (*ImgBBUploadResult, error) {
	// 获取访客认证
	auth, err := GetImgBBGuestAuth(timeout)
	if err != nil {
		return nil, err
	}

	// 确定到期时间
	expiration, ok := ImgBBExpirations[expirationDays]
	if !ok {
		return nil, fmt.Errorf("ImgBB 不支持 %d 天的到期时间，请使用 R2 存储", expirationDays)
	}

	// 构造表单数据
	formData := url.Values{}
	formData.Set("type", "url")
	formData.Set("action", "upload")
	formData.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	formData.Set("auth_token", auth.AuthToken)
	formData.Set("source", imageURL)
	if expiration != "" {
		formData.Set("expiration", expiration)
	}

	// 创建上传请求
	req, err := http.NewRequest("POST", ImgBBJSONURL, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://imgbb.com")
	req.Header.Set("Referer", ImgBBHomeURL)
	if auth.PHPSESSID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "PHPSESSID",
			Value: auth.PHPSESSID,
		})
	}

	// 发送请求
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取上传响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ImgBB 上传失败，HTTP 状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	// 检查状态码
	statusCode, _ := result["status_code"].(float64)
	if statusCode != 200 {
		errorMsg := "未知错误"
		if errorData, ok := result["error"].(map[string]interface{}); ok {
			if msg, ok := errorData["message"].(string); ok {
				errorMsg = msg
			}
		}
		return nil, fmt.Errorf("ImgBB 上传失败: %s", errorMsg)
	}

	// 提取链接
	imageData, ok := result["image"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("ImgBB 响应中缺少 image 数据")
	}

	deleteURL, _ := imageData["delete_url"].(string)
	directURL, _ := imageData["url"].(string)

	if deleteURL == "" || directURL == "" {
		return nil, fmt.Errorf("ImgBB 响应中缺少必要的链接信息")
	}

	return &ImgBBUploadResult{
		DeleteURL: deleteURL,
		DirectURL: directURL,
	}, nil
}

// FindClosestImgBBExpiration 查找最接近的 ImgBB 到期天数
func FindClosestImgBBExpiration(days int) (int, bool) {
	if days == 0 {
		return 0, true // 永久
	}

	// 精确匹配
	if _, ok := ImgBBExpirations[days]; ok {
		return days, true
	}

	// 查找最接近的较小值
	closest := -1
	for supportedDays := range ImgBBExpirations {
		if supportedDays <= days && supportedDays > closest {
			closest = supportedDays
		}
	}

	if closest >= 0 {
		return closest, true
	}

	return 0, false
}

// DeleteImgBBFile 通过 deleteUrl 删除 ImgBB 文件
func DeleteImgBBFile(deleteURL string, timeout time.Duration) error {
	if deleteURL == "" {
		return fmt.Errorf("deleteURL 为空")
	}

	// 验证是否为 ImgBB 删除链接
	if !strings.Contains(deleteURL, "ibb.co") {
		return fmt.Errorf("无效的 ImgBB 删除链接")
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("创建删除请求失败: %w", err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("删除请求失败: %w", err)
	}
	defer resp.Body.Close()

	// ImgBB 删除链接访问后即删除，任何 HTTP 状态都视为成功
	// （有些情况下返回 302 重定向，有些返回 200）
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}

	return fmt.Errorf("ImgBB 删除失败，HTTP 状态码: %d", resp.StatusCode)
}
