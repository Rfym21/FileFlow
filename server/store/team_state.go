package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const teamStateKey = "team_state"

// The team extension lives beside legacy data so old database tables need no ALTER.
// SQL backends write this value in the same transaction as existing resources.
type teamState struct {
	Version         int                              `json:"version"`
	Users           []User                           `json:"users"`
	AuditEntries    []AuditEntry                     `json:"auditEntries"`
	UploadSessions  []UploadSession                  `json:"uploadSessions,omitempty"`
	TokenOwners     map[string]string                `json:"tokenOwners"`
	WebDAVOwners    map[string]string                `json:"webdavOwners"`
	ReadQuotaLimits map[string]int64                 `json:"readQuotaLimits,omitempty"`
	OperationUsage  map[string]AccountOperationUsage `json:"operationUsage,omitempty"`
	UsageRevisions  map[string]uint64                `json:"usageRevisions,omitempty"`
}

func marshalTeamState(data *Data) (string, error) {
	state := teamState{Version: 1, Users: data.Users, AuditEntries: data.AuditEntries, UploadSessions: data.UploadSessions, TokenOwners: map[string]string{}, WebDAVOwners: map[string]string{}, ReadQuotaLimits: map[string]int64{}, OperationUsage: data.OperationUsage, UsageRevisions: map[string]uint64{}}
	for _, account := range data.Accounts {
		state.ReadQuotaLimits[account.ID] = account.Quota.MaxClassBOps
		state.UsageRevisions[account.ID] = account.UsageRevision
	}
	for _, token := range data.Tokens {
		state.TokenOwners[token.ID] = token.UserID
	}
	for _, credential := range data.WebDAVCredentials {
		state.WebDAVOwners[credential.ID] = credential.UserID
	}
	encoded, err := json.Marshal(state)
	return string(encoded), err
}

func applyTeamState(data *Data, encoded string) error {
	if encoded == "" {
		return nil
	}
	var state teamState
	if err := json.Unmarshal([]byte(encoded), &state); err != nil {
		return fmt.Errorf("读取团队信息失败: %w", err)
	}
	if state.Version != 1 {
		return fmt.Errorf("不支持的团队信息版本: %d", state.Version)
	}
	seenIDs, seenNames := map[string]bool{}, map[string]bool{}
	for _, user := range state.Users {
		name := strings.ToLower(strings.TrimSpace(user.Username))
		if user.ID == "" || name == "" || seenIDs[user.ID] || seenNames[name] || len(PermissionsForRole(user.Role)) == 0 || user.PasswordHash == "" {
			return fmt.Errorf("读取团队信息失败: 成员信息无效")
		}
		seenIDs[user.ID], seenNames[name] = true, true
	}
	data.Users = state.Users
	seenSessions := map[string]bool{}
	for _, session := range state.UploadSessions {
		if seenSessions[session.ID] || validateStoredUploadSession(session) != nil {
			return fmt.Errorf("读取团队信息失败: 上传任务信息无效")
		}
		seenSessions[session.ID] = true
	}
	data.UploadSessions = state.UploadSessions
	for _, limit := range state.ReadQuotaLimits {
		if limit < 0 {
			return fmt.Errorf("读取团队信息失败: 读取配额无效")
		}
	}
	for i := range data.Accounts {
		data.Accounts[i].UsageRevision = state.UsageRevisions[data.Accounts[i].ID]
		if limit, exists := state.ReadQuotaLimits[data.Accounts[i].ID]; exists {
			data.Accounts[i].Quota.MaxClassBOps = limit
		}
	}
	for _, usage := range state.OperationUsage {
		if _, err := time.Parse("2006-01", usage.Month); err != nil || usage.ClassAOps < 0 || usage.ClassBOps < 0 {
			return fmt.Errorf("读取团队信息失败: 操作计数无效")
		}
	}
	data.OperationUsage = state.OperationUsage
	data.AuditEntries = state.AuditEntries
	if len(data.AuditEntries) > maxAuditEntries {
		data.AuditEntries = data.AuditEntries[len(data.AuditEntries)-maxAuditEntries:]
	}
	for i := range data.Tokens {
		data.Tokens[i].UserID = state.TokenOwners[data.Tokens[i].ID]
	}
	for i := range data.WebDAVCredentials {
		data.WebDAVCredentials[i].UserID = state.WebDAVOwners[data.WebDAVCredentials[i].ID]
	}
	return nil
}

func createSQLTeamState(db *sql.DB, mysql bool) error {
	valueType := "TEXT"
	if mysql {
		valueType = "LONGTEXT"
	}
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS team_state (id INTEGER PRIMARY KEY, value " + valueType + " NOT NULL)")
	return err
}

func loadSQLTeamState(db *sql.DB, data *Data) error {
	var encoded string
	err := db.QueryRow("SELECT value FROM team_state WHERE id = 1").Scan(&encoded)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取团队信息失败: %w", err)
	}
	return applyTeamState(data, encoded)
}

func saveSQLTeamState(tx *sql.Tx, data *Data, postgres bool) error {
	encoded, err := marshalTeamState(data)
	if err != nil {
		return fmt.Errorf("保存团队信息失败: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM team_state WHERE id = 1"); err != nil {
		return err
	}
	query := "INSERT INTO team_state (id, value) VALUES (1, ?)"
	if postgres {
		query = "INSERT INTO team_state (id, value) VALUES (1, $1)"
	}
	_, err = tx.Exec(query, encoded)
	return err
}
