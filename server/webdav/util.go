package webdav

import (
	"path"
	"strings"
)

// slashClean is equivalent to but slightly more efficient than
// path.Clean("/" + name).
func slashClean(name string) string {
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return path.Clean(name)
}

// Context keys
type ctxKey string

const (
	userAgentKey ctxKey = "webdav_user_agent"
	storageKey   ctxKey = "webdav_storage"
	userKey      ctxKey = "webdav_user"
)

// stripPrefix removes the prefix from the path
func stripPrefix(p, prefix string) (string, error) {
	if prefix == "" {
		return p, nil
	}
	if !strings.HasPrefix(p, prefix) {
		return "", errPrefixMismatch
	}
	return strings.TrimPrefix(p, prefix), nil
}
