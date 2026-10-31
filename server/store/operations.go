package store

import (
	"math"
	"time"
)

// AccountOperationUsage preserves authorized monthly counts when a delayed R2
// usage sync reports older values. Billing counters remain a conservative floor.
type AccountOperationUsage struct {
	Month     string `json:"month"`
	ClassAOps int64  `json:"classAOps"`
	ClassBOps int64  `json:"classBOps"`
}

// GetAccountOperationView returns current-month usage without changing persisted
// data, so candidate selection does not reject accounts on last month's counts.
func GetAccountOperationView(accountID string) (*Account, error) {
	return GetAccountByID(accountID)
}

func operationUsageView(account Account, now time.Time) Account {
	month := now.UTC().Format("2006-01")
	if usage, exists := data.OperationUsage[account.ID]; exists {
		if usage.Month == month {
			account.Usage.ClassAOps = max(account.Usage.ClassAOps, usage.ClassAOps)
			account.Usage.ClassBOps = max(account.Usage.ClassBOps, usage.ClassBOps)
		} else {
			// A new-month sync can precede the first authorization. Its current
			// counts are a valid floor even while the saved ledger is still old.
			synced, err := time.Parse(time.RFC3339, account.Usage.LastSyncAt)
			if err != nil || synced.UTC().Format("2006-01") != month {
				account.Usage.ClassAOps, account.Usage.ClassBOps = 0, 0
			}
		}
	} else if synced, err := time.Parse(time.RFC3339, account.Usage.LastSyncAt); err == nil && synced.UTC().Format("2006-01") != month {
		account.Usage.ClassAOps, account.Usage.ClassBOps = 0, 0
	}
	return account
}

func recordOperationUsage(account Account, now time.Time) {
	updated := make(map[string]AccountOperationUsage, len(data.OperationUsage)+1)
	for id, usage := range data.OperationUsage {
		updated[id] = usage
	}
	updated[account.ID] = AccountOperationUsage{Month: now.UTC().Format("2006-01"), ClassAOps: account.Usage.ClassAOps, ClassBOps: account.Usage.ClassBOps}
	data.OperationUsage = updated
}

func operationCapacityAvailable(account Account, classA, classB int64) bool {
	if classA < 0 || classB < 0 || account.Quota.MaxClassAOps < 0 || account.Quota.MaxClassBOps < 0 || account.Usage.ClassAOps < 0 || account.Usage.ClassBOps < 0 {
		return false
	}
	if classA > 0 && (account.Usage.ClassAOps > account.Quota.MaxClassAOps || classA > account.Quota.MaxClassAOps-account.Usage.ClassAOps) {
		return false
	}
	if account.Quota.MaxClassBOps > 0 && (account.Usage.ClassBOps > account.Quota.MaxClassBOps || classB > account.Quota.MaxClassBOps-account.Usage.ClassBOps) {
		return false
	}
	return true
}

// ReserveAccountOperations authorizes and records attempts before issuing R2
// calls. Uncompleted uploads retain their future operation slots. A failed R2
// attempt remains charged; a rejected authorization or failed database save does not.
func ReserveAccountOperations(accountID string, classA, classB int64) error {
	if classA < 0 || classB < 0 {
		return ErrUploadQuotaExceeded
	}
	dataLock.Lock()
	defer dataLock.Unlock()
	if data == nil {
		return ErrUploadQuotaExceeded
	}
	for i := range data.Accounts {
		account := &data.Accounts[i]
		if account.ID != accountID {
			continue
		}
		if !account.IsActive {
			return ErrUploadSessionForbidden
		}
		now := time.Now().UTC()
		view := operationUsageView(*account, now)
		_, reservedA, _ := uploadReservationsLocked(accountID, now)
		reservedB := uploadReadReservationsLocked(accountID, now)
		// Compare by subtraction so an untrusted huge count cannot overflow.
		if !operationCapacityAvailable(view, reservedA, reservedB) ||
			(classA > 0 && classA > account.Quota.MaxClassAOps-view.Usage.ClassAOps-reservedA) ||
			(account.Quota.MaxClassBOps > 0 && classB > account.Quota.MaxClassBOps-view.Usage.ClassBOps-reservedB) {
			return ErrUploadQuotaExceeded
		}
		if classA == 0 && classB == 0 {
			return nil
		}
		if classA > math.MaxInt64-view.Usage.ClassAOps || classB > math.MaxInt64-view.Usage.ClassBOps {
			return ErrUploadQuotaExceeded
		}
		previous, previousLedger := *account, data.OperationUsage
		account.Usage.ClassAOps, account.Usage.ClassBOps = view.Usage.ClassAOps+classA, view.Usage.ClassBOps+classB
		account.UpdatedAt = now.Format(time.RFC3339)
		recordOperationUsage(*account, now)
		if err := save(); err != nil {
			*account = previous
			data.OperationUsage = previousLedger
			return err
		}
		return nil
	}
	return ErrUploadSessionForbidden
}

func ConsumeUploadSessionRead(id string) error {
	return consumeUploadSessionRead(id, false)
}

// ConsumeUploadSessionCleanupRead is reserved for backend cleanup, not a user
// completion endpoint. Expired/revoked users cannot strand object reservations.
func ConsumeUploadSessionCleanupRead(id string) error {
	return consumeUploadSessionRead(id, true)
}

func consumeUploadSessionRead(id string, cleanup bool) error {
	dataLock.Lock()
	defer dataLock.Unlock()
	i := uploadSessionIndex(id)
	if i < 0 {
		return ErrUploadSessionNotFound
	}
	session := &data.UploadSessions[i]
	now := time.Now().UTC()
	if (!cleanup && !uploadSessionPendingAt(*session, now)) || !uploadSessionUncompleted(*session) {
		return ErrUploadSessionExpired
	}
	var account *Account
	if cleanup {
		for j := range data.Accounts {
			if data.Accounts[j].ID == session.AccountID && data.Accounts[j].IsActive {
				account = &data.Accounts[j]
				break
			}
		}
		if account == nil {
			return ErrUploadSessionForbidden
		}
	} else {
		var err error
		account, err = uploadSessionAuthority(*session)
		if err != nil {
			return err
		}
	}
	view := operationUsageView(*account, now)
	_, reservedA, _ := uploadReservationsLocked(account.ID, now)
	reservedB := uploadReadReservationsLocked(account.ID, now)
	extra := int64(0)
	if session.ConsumedClassBOps >= session.ReservedClassBOps {
		extra = 1
	}
	if !operationCapacityAvailable(view, reservedA, reservedB+extra) {
		return ErrUploadQuotaExceeded
	}
	previousSession, previousAccount, previousLedger := *session, *account, data.OperationUsage
	session.ConsumedClassBOps++
	account.Usage.ClassAOps, account.Usage.ClassBOps = view.Usage.ClassAOps, view.Usage.ClassBOps+1
	account.UpdatedAt = now.Format(time.RFC3339)
	recordOperationUsage(*account, now)
	if err := save(); err != nil {
		*session = previousSession
		*account = previousAccount
		data.OperationUsage = previousLedger
		return err
	}
	return nil
}
