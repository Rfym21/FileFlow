package webdav

import (
	"encoding/xml"
	"net/http"
	"time"
)

// WebDAV XML 命名空间
const (
	nsDAV = "DAV:"
)

/**
 * WebDAV Multistatus 响应结构
 * RFC 4918: 207 Multi-Status
 */
type Multistatus struct {
	XMLName   xml.Name   `xml:"DAV: multistatus"`
	Responses []Response `xml:"response"`
}

/**
 * WebDAV Response 元素
 */
type Response struct {
	Href     string     `xml:"href"`
	Propstat []Propstat `xml:"propstat"`
}

/**
 * WebDAV Propstat 元素
 */
type Propstat struct {
	Prop   Prop   `xml:"prop"`
	Status string `xml:"status"`
}

/**
 * WebDAV Prop 元素 - 文件/目录属性
 */
type Prop struct {
	ResourceType    *ResourceType `xml:"resourcetype,omitempty"`
	DisplayName     string        `xml:"displayname,omitempty"`
	GetContentType  string        `xml:"getcontenttype,omitempty"`
	GetContentLength int64        `xml:"getcontentlength,omitempty"`
	GetLastModified string        `xml:"getlastmodified,omitempty"`
	CreationDate    string        `xml:"creationdate,omitempty"`
	GetETag         string        `xml:"getetag,omitempty"`
}

/**
 * WebDAV ResourceType 元素
 */
type ResourceType struct {
	Collection *struct{} `xml:"collection,omitempty"`
}

/**
 * WebDAV Propfind 请求结构
 */
type Propfind struct {
	XMLName  xml.Name `xml:"propfind"`
	Prop     *Prop    `xml:"prop,omitempty"`
	AllProp  *struct{} `xml:"allprop,omitempty"`
	PropName *struct{} `xml:"propname,omitempty"`
}

/**
 * 写入 WebDAV XML 响应
 */
func WriteXML(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(status)

	output, err := xml.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	w.Write([]byte(xml.Header))
	w.Write(output)
	return nil
}

/**
 * 创建文件属性响应
 */
func NewFileProp(name string, size int64, modTime time.Time) Prop {
	return Prop{
		DisplayName:      name,
		GetContentType:   getContentType(name),
		GetContentLength: size,
		GetLastModified:  modTime.UTC().Format(http.TimeFormat),
		CreationDate:     modTime.UTC().Format(time.RFC3339),
		GetETag:          generateETag(name, size, modTime),
	}
}

/**
 * 创建目录属性响应
 */
func NewDirProp(name string, modTime time.Time) Prop {
	return Prop{
		ResourceType:    &ResourceType{Collection: &struct{}{}},
		DisplayName:     name,
		GetLastModified: modTime.UTC().Format(http.TimeFormat),
		CreationDate:    modTime.UTC().Format(time.RFC3339),
	}
}

/**
 * 根据文件名获取 Content-Type
 */
func getContentType(name string) string {
	// 简化版本，可以根据需要扩展
	contentTypes := map[string]string{
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".pdf":  "application/pdf",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
	}

	for ext, ct := range contentTypes {
		if len(name) >= len(ext) && name[len(name)-len(ext):] == ext {
			return ct
		}
	}

	return "application/octet-stream"
}

/**
 * 生成 ETag
 */
func generateETag(name string, size int64, modTime time.Time) string {
	return `"` + name + `-` + modTime.UTC().Format("20060102150405") + `"`
}
