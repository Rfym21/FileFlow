package webdav

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/**
 * PROPFIND 处理函数
 * RFC 4918: 列出文件和目录的属性
 */
func handlePropfind(w http.ResponseWriter, r *http.Request) {
	cred, ok := GetCredentialFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !HasPermission(cred, "read") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	acc, ok := GetAccountFromContext(r.Context())
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 解析路径
	urlPath := strings.TrimPrefix(r.URL.Path, "/webdav")
	urlPath = strings.TrimPrefix(urlPath, "/")
	if urlPath == "" {
		urlPath = "/"
	}

	// 解析 PROPFIND 请求体
	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "infinity"
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var propfindReq Propfind
	if len(body) > 0 {
		if err := xml.Unmarshal(body, &propfindReq); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}

	// 创建 S3 客户端
	client, err := createS3Client(acc)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 构建响应
	multistatus := Multistatus{
		Responses: []Response{},
	}

	// 检查是目录还是文件
	isDir := strings.HasSuffix(urlPath, "/") || urlPath == "/"

	if isDir {
		// 列出目录内容
		prefix := strings.TrimPrefix(urlPath, "/")
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}

		delimiter := ""
		if depth == "1" {
			delimiter = "/"
		}

		listResp, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket:    aws.String(acc.BucketName),
			Prefix:    aws.String(prefix),
			Delimiter: aws.String(delimiter),
		})

		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// 获取目录时间（如果目录为空，使用当前时间）
		dirTime := time.Now()
		if len(listResp.Contents) > 0 && listResp.Contents[0].LastModified != nil {
			dirTime = *listResp.Contents[0].LastModified
		}

		// 添加当前目录
		multistatus.Responses = append(multistatus.Responses, Response{
			Href: "/webdav/" + urlPath,
			Propstat: []Propstat{
				{
					Prop:   NewDirProp(path.Base(urlPath), dirTime),
					Status: "HTTP/1.1 200 OK",
				},
			},
		})

		// 添加子目录
		for _, commonPrefix := range listResp.CommonPrefixes {
			dirName := strings.TrimSuffix(strings.TrimPrefix(*commonPrefix.Prefix, prefix), "/")
			multistatus.Responses = append(multistatus.Responses, Response{
				Href: "/webdav/" + *commonPrefix.Prefix,
				Propstat: []Propstat{
					{
						Prop:   NewDirProp(dirName, dirTime),
						Status: "HTTP/1.1 200 OK",
					},
				},
			})
		}

		// 添加文件
		for _, obj := range listResp.Contents {
			if *obj.Key == prefix {
				continue // 跳过目录本身
			}
			fileName := path.Base(*obj.Key)
			multistatus.Responses = append(multistatus.Responses, Response{
				Href: "/webdav/" + *obj.Key,
				Propstat: []Propstat{
					{
						Prop:   NewFileProp(fileName, *obj.Size, *obj.LastModified),
						Status: "HTTP/1.1 200 OK",
					},
				},
			})
		}
	} else {
		// 获取单个文件信息
		key := strings.TrimPrefix(urlPath, "/")
		headResp, err := client.HeadObject(context.Background(), &s3.HeadObjectInput{
			Bucket: aws.String(acc.BucketName),
			Key:    aws.String(key),
		})

		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		fileName := path.Base(key)
		multistatus.Responses = append(multistatus.Responses, Response{
			Href: "/webdav/" + urlPath,
			Propstat: []Propstat{
				{
					Prop:   NewFileProp(fileName, *headResp.ContentLength, *headResp.LastModified),
					Status: "HTTP/1.1 200 OK",
				},
			},
		})
	}

	// 返回 207 Multi-Status
	WriteXML(w, http.StatusMultiStatus, multistatus)
}
