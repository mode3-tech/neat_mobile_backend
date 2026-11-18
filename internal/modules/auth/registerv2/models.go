package registerv2

import "time"

// ProviderPreference is a singleton row (id is always 1) recording which BaaS
// provider new registrations should use. Switching it only affects new
// registrations going forward - existing wallets stay on whichever provider
// created them.
type ProviderPreference struct {
	ID        int16     `gorm:"column:id;primaryKey"`
	Provider  string    `gorm:"column:provider;type:text;not null"`
	UpdatedBy *string   `gorm:"column:updated_by;type:text"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (ProviderPreference) TableName() string { return "wallet_provider_preference" }

// Pending Wema registration states.
const (
	PendingWemaStatusPending    = "pending"    // waiting for the ALAT account
	PendingWemaStatusProcessing = "processing" // a worker is completing it right now
	PendingWemaStatusCompleted  = "completed"  // user and wallet saved
	PendingWemaStatusFailed     = "failed"     // cannot complete (see FailureReason)
	PendingWemaStatusExpired    = "expired"    // the ALAT account never appeared in time
)

// PendingWemaRegistration is a /register call that passed every check and
// asked ALAT for a tier 2 account, but the account wasn't ready before the
// request had to answer. It holds what's needed to finish the registration
// once the account exists: ALAT's Account Creation webhook, the sweep, or the
// client's status poll completes it, and the client claims its session with
// ClaimToken. Secrets are stored hashed (password, PIN, claim token) or
// encrypted (BVN, NIN); the face image is not stored.
//
// Only one active (pending/processing) row may exist per phone.
type PendingWemaRegistration struct {
	ID               string `gorm:"column:id;type:text;primaryKey"`
	MobileUserID     string `gorm:"column:mobile_user_id;type:text;not null"`
	InternalWalletID string `gorm:"column:internal_wallet_id;type:text;not null"`
	Phone            string `gorm:"column:phone;type:text;not null;index;uniqueIndex:idx_pending_wema_active_phone,where:status = 'pending' OR status = 'processing'"`
	Email            string `gorm:"column:email;type:text;not null"`
	FirstName        string `gorm:"column:first_name;type:text;not null"`
	LastName         string `gorm:"column:last_name;type:text;not null"`
	DOB              string `gorm:"column:dob;type:text;not null"` // YYYY-MM-DD, as validated
	Address          string `gorm:"column:address;type:text;not null"`
	BVNEncrypted     string `gorm:"column:bvn_encrypted;type:text;not null"`
	NINEncrypted     string `gorm:"column:nin_encrypted;type:text;not null"`
	PasswordHash     string `gorm:"column:password_hash;type:text;not null"`
	PinHash          string `gorm:"column:pin_hash;type:text;not null"`
	Biometrics       bool   `gorm:"column:biometrics;not null"`
	ReferralCode     string `gorm:"column:referral_code;type:text"`

	DeviceID          string `gorm:"column:device_id;type:text;not null"`
	DevicePublicKey   string `gorm:"column:device_public_key;type:text;not null"`
	DeviceName        string `gorm:"column:device_name;type:text;not null"`
	DeviceModel       string `gorm:"column:device_model;type:text;not null"`
	DeviceOS          string `gorm:"column:device_os;type:text;not null"`
	DeviceOSVersion   string `gorm:"column:device_os_version;type:text;not null"`
	DeviceAppVersion  string `gorm:"column:device_app_version;type:text;not null"`
	IP                string `gorm:"column:ip;type:text"`
	PhoneVerification string `gorm:"column:phone_verification_id;type:text;not null"`
	EmailVerification string `gorm:"column:email_verification_id;type:text;not null"`
	BVNVerification   string `gorm:"column:bvn_verification_id;type:text;not null"`

	TrackingID     string `gorm:"column:tracking_id;type:text"`
	ClaimTokenHash string `gorm:"column:claim_token_hash;type:text;not null"`

	Status        string     `gorm:"column:status;type:text;not null;index"`
	FailureReason *string    `gorm:"column:failure_reason;type:text"`
	AccountNumber *string    `gorm:"column:account_number;type:text"`
	Attempts      int        `gorm:"column:attempts;not null;default:0"`
	LastAttemptAt *time.Time `gorm:"column:last_attempt_at;type:timestamptz"`
	ExpiresAt     time.Time  `gorm:"column:expires_at;type:timestamptz;not null"`
	CompletedAt   *time.Time `gorm:"column:completed_at;type:timestamptz"`
	ClaimedAt     *time.Time `gorm:"column:claimed_at;type:timestamptz"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (PendingWemaRegistration) TableName() string { return "wallet_pending_wema_registrations" }
