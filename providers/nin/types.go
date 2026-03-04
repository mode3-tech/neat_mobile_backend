package nin

type PremblyNINValidationSuccessResponse struct {
	Status             bool                             `json:"status"`
	ResponseCode       string                           `json:"response_code"`
	Message            string                           `json:"message"`
	Detail             string                           `json:"detail"`
	Data               PremblyNINValidationData         `json:"data"`
	Meta               map[string]any                   `json:"meta"`
	BillingInfo        PremblyNINValidationBillingInfo  `json:"billing_info"`
	Verification       PremblyNINValidationVerification `json:"verification"`
	ReferenceID        string                           `json:"reference_id"`
	TransactionID      string                           `json:"transaction_id"`
	VerificationStatus string                           `json:"verification_status"`
	NINData            PremblyNINValidationData         `json:"nin_data"`
}

type PremblyNINValidationData struct {
	NIN              string `json:"nin"`
	FirstName        string `json:"firstname"`
	Surname          string `json:"surname"`
	MiddleName       string `json:"middlename"`
	TelephoneNo      string `json:"telephoneno"`
	Gender           string `json:"gender"`
	Photo            string `json:"photo"`
	BirthDate        string `json:"birthdate"`
	ResidenceAddress string `json:"residence_address"`
	BirthCountry     string `json:"birthcountry"`
	SelfOriginLGA    string `json:"self_origin_lga"`
	SelfOriginState  string `json:"self_origin_state"`
	Signature        string `json:"signature"`
	ResidenceLGA     string `json:"residence_lga"`
	ResidenceState   string `json:"residence_state"`
	ResidenceTown    string `json:"residence_town"`
	NOKAddress1      string `json:"nok_address1"`
	NOKAddress2      string `json:"nok_address2"`
	NOKFirstName     string `json:"nok_firstname"`
	NOKLGA           string `json:"nok_lga"`
	NOKMiddleName    string `json:"nok_middlename"`
	NOKPostalCode    string `json:"nok_postalcode"`
	NOKState         string `json:"nok_state"`
	NOKSurname       string `json:"nok_surname"`
	NOKTown          string `json:"nok_town"`
	PMiddleName      string `json:"pmiddlename"`
	SelfOriginPlace  string `json:"self_origin_place"`
	PSurname         string `json:"psurname"`
	BirthState       string `json:"birthstate"`
	TrackingID       string `json:"trackingId"`
	ResidenceStatus  string `json:"residencestatus"`
	EducationalLevel string `json:"educationallevel"`
	VNIN             string `json:"vnin"`
	Email            string `json:"email"`
	OSpokenLang      string `json:"ospokenlang"`
	Heigth           string `json:"heigth"`
	PFirstName       string `json:"pfirstname"`
	UserID           string `json:"userid"`
	SpokenLanguage   string `json:"spoken_language"`
	Profession       string `json:"profession"`
	Religion         string `json:"religion"`
	MaritalStatus    string `json:"maritalstatus"`
	CentralID        string `json:"centralID"`
	Title            string `json:"title"`
	BirthLGA         string `json:"birthlga"`
	EmploymentStatus string `json:"employmentstatus"`
}

type PremblyNINValidationBillingInfo struct {
	WasCharged bool   `json:"was_charged"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
	Note       string `json:"note"`
}

type PremblyNINValidationVerification struct {
	Status         string `json:"status"`
	Reference      string `json:"reference"`
	VerificationID string `json:"verification_id"`
}
