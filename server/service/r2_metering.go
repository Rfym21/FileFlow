package service

import (
	"net/http"
	"strings"
	"time"

	"fileflow/server/store"
)

// ChargeAccountOperations records FileFlow-authorized R2 operations before they
// execute. Requests made outside FileFlow cannot be measured by this ledger.
func ChargeAccountOperations(accountID string, classA, classB int64) error {
	return store.ReserveAccountOperations(accountID, classA, classB)
}

type meteredR2Transport struct {
	accountID string
	base      http.RoundTripper
}

func (t meteredR2Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	var classA, classB int64
	switch request.Method {
	case http.MethodGet:
		if request.URL.Query().Has("list-type") || request.URL.Query().Has("uploads") || request.URL.Query().Has("uploadId") || request.URL.Query().Has("prefix") {
			classA = 1
		} else {
			classB = 1
		}
	case http.MethodHead:
		classB = 1
	case http.MethodPut, http.MethodPost:
		if !(request.Method == http.MethodPost && request.URL.Query().Has("delete")) {
			classA = 1
		}
	case http.MethodDelete:
		// R2 DeleteObject, DeleteObjects and AbortMultipartUpload are free.
	default:
		classA = 1
	}
	if classA != 0 || classB != 0 {
		if err := ChargeAccountOperations(t.accountID, classA, classB); err != nil {
			return nil, err
		}
	}
	return t.base.RoundTrip(request)
}

// NewMeteredR2HTTPClient meters actual SDK requests, including retries. S3
// presigning uses its own no-op HTTP client and does not invoke this transport.
func NewMeteredR2HTTPClient(accountID string) *http.Client {
	return &http.Client{
		Transport: meteredR2Transport{accountID: accountID, base: http.DefaultTransport},
		Timeout:   5 * time.Minute,
	}
}

// IsPendingUploadKey hides objects that have not passed the completion contract.
func IsPendingUploadKey(accountID, key string) bool {
	key = strings.TrimPrefix(key, "/")
	for _, session := range store.ListUploadSessions() {
		if session.AccountID == accountID && session.FileKey == key && session.Status != store.UploadStatusCompleted {
			return true
		}
	}
	return false
}
