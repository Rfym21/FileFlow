package webdav

import (
	"encoding/xml"
	"net/http"
	"time"
)

/**
 * LOCK 处理函数
 * WebDAV 锁定机制（简化实现）
 * 注意：此实现为兼容性实现，不提供真正的锁定功能
 */
func handleLock(w http.ResponseWriter, r *http.Request) {
	cred, ok := GetCredentialFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !HasPermission(cred, "write") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 返回一个虚拟的锁定响应
	// 这是为了兼容某些 WebDAV 客户端，实际上不实现真正的锁定
	lockToken := "opaquelocktoken:" + generateLockToken()

	lockResponse := &LockDiscovery{
		XMLName: xml.Name{Space: nsDAV, Local: "prop"},
		ActiveLock: ActiveLock{
			LockType:   LockType{Write: &struct{}{}},
			LockScope:  LockScope{Exclusive: &struct{}{}},
			Depth:      "0",
			Owner:      Owner{Href: cred.Username},
			Timeout:    "Second-3600",
			LockToken:  LockToken{Href: lockToken},
			LockRoot:   LockRoot{Href: r.URL.Path},
		},
	}

	w.Header().Set("Lock-Token", "<"+lockToken+">")
	WriteXML(w, http.StatusOK, lockResponse)
}

/**
 * UNLOCK 处理函数
 * WebDAV 解锁机制（简化实现）
 */
func handleUnlock(w http.ResponseWriter, r *http.Request) {
	cred, ok := GetCredentialFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !HasPermission(cred, "write") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 简化实现：直接返回成功
	w.WriteHeader(http.StatusNoContent)
}

/**
 * 锁定相关的 XML 结构
 */
type LockDiscovery struct {
	XMLName    xml.Name   `xml:"DAV: prop"`
	ActiveLock ActiveLock `xml:"lockdiscovery>activelock"`
}

type ActiveLock struct {
	LockType  LockType  `xml:"locktype"`
	LockScope LockScope `xml:"lockscope"`
	Depth     string    `xml:"depth"`
	Owner     Owner     `xml:"owner"`
	Timeout   string    `xml:"timeout"`
	LockToken LockToken `xml:"locktoken"`
	LockRoot  LockRoot  `xml:"lockroot"`
}

type LockType struct {
	Write *struct{} `xml:"write,omitempty"`
}

type LockScope struct {
	Exclusive *struct{} `xml:"exclusive,omitempty"`
	Shared    *struct{} `xml:"shared,omitempty"`
}

type Owner struct {
	Href string `xml:"href"`
}

type LockToken struct {
	Href string `xml:"href"`
}

type LockRoot struct {
	Href string `xml:"href"`
}

/**
 * 生成锁定令牌
 */
func generateLockToken() string {
	return time.Now().Format("20060102150405")
}
