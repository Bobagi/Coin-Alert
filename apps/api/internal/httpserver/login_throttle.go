package httpserver

import (
	"strings"
	"sync"
	"time"
)

// Login throttling parameters. nginx already rate-limits /auth/login per source IP; this adds a
// per-account ceiling so a distributed (many-IP) credential-stuffing attack against a single
// account is also slowed, without ever locking a legitimate user out for long.
const (
	maxFailedLoginAttempts = 5
	loginFailureWindow     = 15 * time.Minute
	loginLockoutDuration   = 15 * time.Minute
)

// loginThrottle tracks recent failed sign-in attempts per account key (the lowercased email) in
// memory. It is intentionally process-local: it resets on restart and does not need a database
// table. Under the single-instance API deployment this is sufficient; if the API is ever scaled
// out, move this to a shared store.
type loginThrottle struct {
	mutex    sync.Mutex
	attempts map[string]*loginAttemptRecord
}

type loginAttemptRecord struct {
	failureCount   int
	windowStart    time.Time
	lockedUntilUTC time.Time
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{attempts: make(map[string]*loginAttemptRecord)}
}

func throttleKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsLocked reports whether the account is currently locked out due to too many recent failures.
func (throttle *loginThrottle) IsLocked(email string) bool {
	key := throttleKey(email)
	if key == "" {
		return false
	}
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()

	record := throttle.attempts[key]
	if record == nil {
		return false
	}
	return time.Now().Before(record.lockedUntilUTC)
}

// RegisterFailure records a failed attempt and locks the account once the threshold is crossed.
func (throttle *loginThrottle) RegisterFailure(email string) {
	key := throttleKey(email)
	if key == "" {
		return
	}
	now := time.Now()
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()

	record := throttle.attempts[key]
	if record == nil || now.Sub(record.windowStart) > loginFailureWindow {
		record = &loginAttemptRecord{windowStart: now}
		throttle.attempts[key] = record
	}
	record.failureCount++
	if record.failureCount >= maxFailedLoginAttempts {
		record.lockedUntilUTC = now.Add(loginLockoutDuration)
	}
}

// RegisterSuccess clears any tracked failures for the account after a successful sign-in.
func (throttle *loginThrottle) RegisterSuccess(email string) {
	key := throttleKey(email)
	if key == "" {
		return
	}
	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	delete(throttle.attempts, key)
}
