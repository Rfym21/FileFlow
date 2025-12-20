package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"fileflow/server/store"
)

const cloudflareGraphQLEndpoint = "https://api.cloudflare.com/client/v4/graphql"

// GraphQL 请求和响应结构
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type graphQLResponse struct {
	Data   *graphQLData   `json:"data"`
	Errors []graphQLError `json:"errors,omitempty"`
}

type graphQLData struct {
	Viewer struct {
		Accounts []struct {
			R2OperationsAdaptiveGroups []struct {
				Sum struct {
					Requests int64 `json:"requests"`
				} `json:"sum"`
				Dimensions struct {
					ActionType   string `json:"actionType"`
					ActionStatus string `json:"actionStatus"`
				} `json:"dimensions"`
			} `json:"r2OperationsAdaptiveGroups"`
		} `json:"accounts"`
	} `json:"viewer"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// Class A 操作类型列表（写入操作）
var classAOperations = map[string]bool{
	"ListBuckets":         true,
	"PutBucket":           true,
	"ListObjects":         true,
	"PutObject":           true,
	"CopyObject":          true,
	"CompleteMultipart":   true,
	"CreateMultipart":     true,
	"UploadPart":          true,
	"UploadPartCopy":      true,
	"PutBucketEncryption": true,
	"PutBucketCors":       true,
	"PutBucketLifecycle":  true,
	"ListMultipartUploads": true,
}

// SyncAccountUsage 同步单个账户的使用量
func SyncAccountUsage(ctx context.Context, acc *store.Account) error {
	// 获取存储容量
	sizeBytes, err := GetAccountStorageSize(ctx, acc)
	if err != nil {
		return fmt.Errorf("获取存储容量失败: %w", err)
	}

	// 获取操作次数
	classAOps, classBOps, err := getAccountOps(ctx, acc)
	if err != nil {
		log.Printf("获取账户 %s 操作次数失败: %v，使用默认值 0", acc.Name, err)
		classAOps = 0
		classBOps = 0
	}

	// 更新使用量
	usage := store.Usage{
		SizeBytes: sizeBytes,
		ClassAOps: classAOps,
		ClassBOps: classBOps,
	}

	if err := store.UpdateAccountUsage(acc.ID, usage); err != nil {
		return fmt.Errorf("更新使用量失败: %w", err)
	}

	log.Printf("[Sync] 账户 %s 同步完成: 容量 %.2f MB, 写入操作 %d 次, 读取操作 %d 次",
		acc.Name, float64(sizeBytes)/1024/1024, classAOps, classBOps)

	return nil
}

// SyncAllAccountsUsage 同步所有账户的使用量
func SyncAllAccountsUsage(ctx context.Context) {
	accounts := store.GetAccounts()

	for _, acc := range accounts {
		if !acc.IsActive {
			continue
		}

		if err := SyncAccountUsage(ctx, &acc); err != nil {
			log.Printf("[Sync] 账户 %s 同步失败: %v", acc.Name, err)
		}
	}

	// 同步后检查是否需要 GC
	RunGCForAllAccounts(ctx)
}

// getAccountOps 获取账户当月操作次数（Class A 和 Class B）
func getAccountOps(ctx context.Context, acc *store.Account) (classA int64, classB int64, err error) {
	if acc.APIToken == "" {
		return 0, 0, fmt.Errorf("账户未配置 API Token")
	}

	// 构建查询：当月的操作统计
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	query := `
		query R2Operations($accountTag: String!, $filter: R2OperationsAdaptiveGroupsFilter_InputType) {
			viewer {
				accounts(filter: { accountTag: $accountTag }) {
					r2OperationsAdaptiveGroups(
						limit: 1000,
						filter: $filter
					) {
						sum {
							requests
						}
						dimensions {
							actionType
							actionStatus
						}
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"accountTag": acc.AccountID,
		"filter": map[string]interface{}{
			"datetime_geq": startOfMonth.Format(time.RFC3339),
			"datetime_lt":  now.Format(time.RFC3339),
			"bucketName":   acc.BucketName,
		},
	}

	reqBody := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return 0, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cloudflareGraphQLEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+acc.APIToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return 0, 0, err
	}

	if len(gqlResp.Errors) > 0 {
		return 0, 0, fmt.Errorf("GraphQL 错误: %s", gqlResp.Errors[0].Message)
	}

	if gqlResp.Data == nil || len(gqlResp.Data.Viewer.Accounts) == 0 {
		return 0, 0, nil
	}

	// 统计操作次数
	var totalClassAOps, totalClassBOps int64
	for _, group := range gqlResp.Data.Viewer.Accounts[0].R2OperationsAdaptiveGroups {
		if classAOperations[group.Dimensions.ActionType] {
			totalClassAOps += group.Sum.Requests
		} else {
			// 非 Class A 的都算 Class B（读取操作）
			totalClassBOps += group.Sum.Requests
		}
	}

	return totalClassAOps, totalClassBOps, nil
}
