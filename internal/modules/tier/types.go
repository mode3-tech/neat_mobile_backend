package tier

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type TierRequirement string

const (
	TierRequirementNIN                 TierRequirement = "nin"
	TierRequirementPassportPhotograph  TierRequirement = "passport_photograph"
	TierRequirementUtilityBill         TierRequirement = "utility_bill"
	TierRequirementAddressVerification TierRequirement = "address_verification"
)

type TierSubmissionStatus string

const (
	TierSubmissionStatusPending  TierSubmissionStatus = "pending"
	TierSubmissionStatusApproved TierSubmissionStatus = "approved"
	TierSubmissionStatusRejected TierSubmissionStatus = "rejected"
)

type RequirementStatus string

const (
	RequirementStatusNotSubmitted RequirementStatus = "not_submitted"
	RequirementStatusPending      RequirementStatus = "pending"
	RequirementStatusApproved     RequirementStatus = "approved"
	RequirementStatusRejected     RequirementStatus = "rejected"
)

type RequirementCheck struct {
	Requirement TierRequirement   `json:"requirement"`
	Status      RequirementStatus `json:"status"`
}

type WalletTier struct {
	ID                   string
	Tier                 string
	DailyLimit           int64
	MaxCumulativeBalance int64
	Requirements         StringList `gorm:"type:jsonb"`
	CurrentUserTier      int
}

type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}

	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *StringList) Scan(src interface{}) error {
	if src == nil {
		*s = nil
		return nil
	}

	var b []byte
	switch v := src.(type) {
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		return fmt.Errorf("StringList: unsupported type %T", v)
	}
	return json.Unmarshal(b, s)
}

type MissingRequirements struct {
	Requirement string `json:"requirement"`
	Value       string `json:"value" binding:"omitempty"`
	HasValue    bool   `json:"has_value"`
}
