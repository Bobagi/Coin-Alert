package domain

import "time"

// User is an authenticated account that owns its own credentials, settings, and trades.
type User struct {
	Identifier    int64
	Email         string
	PasswordHash  string // empty for accounts created via Google that have not set a password
	GoogleSubject string // OIDC `sub` of the linked Google account; empty when not linked
	DisplayName   string
	IsActive      bool
	IsAdmin       bool // admins access the B3 tab and get unlimited trading robots
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// HasPassword reports whether the account can sign in with email + password.
func (user *User) HasPassword() bool { return user.PasswordHash != "" }

// HasGoogleLinked reports whether a Google identity is connected to the account.
func (user *User) HasGoogleLinked() bool { return user.GoogleSubject != "" }

// UserTradingSettings holds the per-user configuration that used to be process-global.
type UserTradingSettings struct {
	UserIdentifier               int64
	TradingPairSymbol            string
	CapitalThreshold             float64
	TargetProfitPercent          float64
	StopLossPercent              *float64 // nil means no stop-loss configured
	AutomaticSellIntervalMinutes int
	DailyPurchaseHourUTC         int
	DailyPurchaseEnabled         bool // explicit on/off switch for the daily DCA buy
	SellOrderValidityDays        int  // 0 = no expiry (GTC); N = cancel the take-profit after N days
	LiveTradingEnabled           bool
	ActiveBinanceEnvironment     string
	BinanceEnvironment           string // the environment this per-environment settings row belongs to
}
