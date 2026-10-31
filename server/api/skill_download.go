package api

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Only installable skill sources are distributed. Runtime state, local
// configuration and script tests must never enter this public archive.
var skillBundleFiles = []string{
	"SKILL.md",
	"agents/openai.yaml",
	"references/operations.md",
	"scripts/fileflow.py",
}

// SetupSkillRouter publishes a static, credential-free skill bundle. source is
// rooted at skills/fileflow; a missing required file fails setup rather than
// delivering an incomplete installation. No store or R2 operation is performed.
func SetupSkillRouter(router *gin.Engine, source fs.FS) error {
	bundle, err := buildSkillBundle(source)
	if err != nil {
		return err
	}
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(bundle))
	router.GET("/api/skill/download", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("ETag", etag)
		c.Header("X-Content-Type-Options", "nosniff")
		for _, candidate := range strings.Split(c.GetHeader("If-None-Match"), ",") {
			candidate = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(candidate), "W/"))
			if candidate == etag || candidate == "*" {
				c.Status(http.StatusNotModified)
				return
			}
		}
		c.Header("Content-Disposition", `attachment; filename="fileflow-skill.zip"`)
		c.Header("Content-Length", strconv.Itoa(len(bundle)))
		c.Data(http.StatusOK, "application/zip", bundle)
	})
	return nil
}

func buildSkillBundle(source fs.FS) ([]byte, error) {
	if source == nil {
		return nil, fmt.Errorf("Skill 来源不能为空")
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range skillBundleFiles {
		info, err := fs.Stat(source, name)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("读取 Skill 文件 %s 失败: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			_ = writer.Close()
			return nil, fmt.Errorf("Skill 文件 %s 必须为普通文件", name)
		}
		contents, err := fs.ReadFile(source, name)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("读取 Skill 文件 %s 失败: %w", name, err)
		}
		if len(contents) == 0 {
			_ = writer.Close()
			return nil, fmt.Errorf("Skill 文件 %s 不能为空", name)
		}
		header := &zip.FileHeader{Name: "fileflow/" + name, Method: zip.Deflate}
		header.SetMode(0644)
		if name == "scripts/fileflow.py" {
			header.SetMode(0755)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("创建 Skill 压缩包失败: %w", err)
		}
		if _, err := entry.Write(contents); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("写入 Skill 压缩包失败: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("完成 Skill 压缩包失败: %w", err)
	}
	return buffer.Bytes(), nil
}
