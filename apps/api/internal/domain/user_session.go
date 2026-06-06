package domain

import "time"

// UserSession is a server-side session. Only a hash of the opaque session token is persisted;
// the raw token lives solely in the user's secure cookie.
type UserSession struct {
	Identifier       int64
	UserIdentifier   int64
	SessionTokenHash string
	ExpiresAt        time.Time
	CreatedAt        time.Time
	LastSeenAt       time.Time
	UserAgent        string
	IPAddress        string
}
