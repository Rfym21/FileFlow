package webdav

import "errors"

// WebDAV HTTP status codes
const (
	StatusMulti            = 207
	StatusUnprocessable    = 422
	StatusLocked           = 423
	StatusFailedDependency = 424
	StatusInsufficientStorage = 507
)

// Common errors
var (
	errInvalidTimeout      = errors.New("webdav: invalid timeout")
	errInvalidLockInfo     = errors.New("webdav: invalid lock info")
	errUnsupportedLockInfo = errors.New("webdav: unsupported lock info")
	errInvalidPropfind     = errors.New("webdav: invalid propfind")
	errInvalidProppatch    = errors.New("webdav: invalid proppatch")
	errInvalidResponse     = errors.New("webdav: invalid response")
	errDestinationEqualsSource = errors.New("webdav: destination equals source")
	errNoFileSystem        = errors.New("webdav: no file system")
	errPrefixMismatch      = errors.New("webdav: prefix mismatch")
	errRecursionTooDeep    = errors.New("webdav: recursion too deep")
)
