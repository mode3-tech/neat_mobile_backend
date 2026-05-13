package bvn

import "time"

type TendarBVNValidationSuccessResponse struct {
	VerificationID string                         `json:"verification_id"`
	Data           TendarBVNValidationSuccessData `json:"data"`
	Error          bool                           `json:"error"`
	Message        string                         `json:"message"`
}

type TendarBVNValidationSuccessData struct {
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	ID              string           `json:"id"`
	User            any              `json:"user"`
	UserID          string           `json:"user_id"`
	Identity        string           `json:"identity"`
	Verified        bool             `json:"verified"`
	VerifiedBy      []string         `json:"verified_by"`
	CanBeVerifiedBy []string         `json:"can_be_verified_by"`
	FaceData        any              `json:"face_data"`
	Details         TendarBVNDetails `json:"details"`
	Email           string           `json:"email"`
	PhoneNumber     string           `json:"phone_number"`
	ExpiresAt       time.Time        `json:"expires_at"`
}

type TendarBVNDetails struct {
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	MiddleName         string `json:"middle_name"`
	DateOfBirth        string `json:"date_of_birth"`
	RegistrationDate   string `json:"registration_date"`
	EnrollmentBank     string `json:"enrollment_bank"`
	EnrollmentBranch   string `json:"enrollment_branch"`
	Email              string `json:"email"`
	Gender             string `json:"gender"`
	LevelOfAccount     string `json:"level_of_account"`
	LGAOfOrigin        string `json:"lga_of_origin"`
	LGAOfResidence     string `json:"lga_of_residence"`
	MaritalStatus      string `json:"marital_status"`
	NameOnCard         string `json:"name_on_card"`
	Nationality        string `json:"nationality"`
	PhoneNumber        string `json:"phone_number"`
	PhoneNumber2       string `json:"phone_number2"`
	ResidentialAddress string `json:"residential_address"`
	StateOfOrigin      string `json:"state_of_origin"`
	StateOfResidence   string `json:"state_of_residence"`
	Title              string `json:"title"`
	WatchListed        string `json:"watch_listed"`
	Image              string `json:"image"`
}

type PremblyBVNValidationSuccessResponse struct {
	VerificationID     string                           `json:"verification_id"`
	Status             bool                             `json:"status"`
	ResponseCode       string                           `json:"response_code"`
	Message            string                           `json:"message"`
	Detail             string                           `json:"detail"`
	Data               PremblyBVNValidationSuccessData  `json:"data"`
	BVNData            PremblyBVNValidationSuccessData  `json:"bvn_data"`
	Meta               map[string]any                   `json:"meta"`
	BillingInfo        PremblyBVNValidationBillingInfo  `json:"billing_info"`
	Verification       PremblyBVNValidationVerification `json:"verification"`
	ReferenceID        string                           `json:"reference_id"`
	TransactionID      string                           `json:"transaction_id"`
	IsSandbox          bool                             `json:"is_sandbox"`
	AccountVerified    bool                             `json:"account_verified"`
	VerificationStatus string                           `json:"verification_status"`
}

type PremblyBVNValidationSuccessData struct {
	BVN                string  `json:"bvn"`
	NIN                string  `json:"nin"`
	FirstName          string  `json:"firstName"`
	LastName           string  `json:"lastName"`
	MiddleName         string  `json:"middleName"`
	DateOfBirth        string  `json:"dateOfBirth"`
	PhoneNumber1       string  `json:"phoneNumber1"`
	PhoneNumber2       string  `json:"phoneNumber2"`
	RegistrationDate   string  `json:"registrationDate"`
	EnrollmentBank     string  `json:"enrollmentBank"`
	EnrollmentBranch   string  `json:"enrollmentBranch"`
	Email              string  `json:"email"`
	Gender             string  `json:"gender"`
	StateOfOrigin      string  `json:"stateOfOrigin"`
	StateOfResidence   string  `json:"stateOfResidence"`
	LGAOfOrigin        string  `json:"lgaOfOrigin"`
	LGAOfResidence     string  `json:"lgaOfResidence"`
	ResidentialAddress string  `json:"residentialAddress"`
	Nationality        string  `json:"nationality"`
	MaritalStatus      string  `json:"maritalStatus"`
	LevelOfAccount     string  `json:"levelOfAccount"`
	WatchListed        string  `json:"watchListed"`
	Title              string  `json:"title"`
	NameOnCard         string  `json:"nameOnCard"`
	Image              *string `json:"base64Image"`
}

type PremblyBVNValidationBillingInfo struct {
	WasCharged bool   `json:"was_charged"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
	Note       string `json:"note"`
}

type PremblyBVNValidationVerification struct {
	Status         string `json:"status"`
	Reference      string `json:"reference"`
	VerificationID string `json:"verification_id"`
}
