package vas

type AirtimePayload struct {
	UniqueCode  string `json:"unique_code" validate:"required"`
	PhoneNumber string `json:"phone_number" validate:"required"`
	Amount      int64  `json:"amount" validate:"amount,gt=0"`
}

type DataPayload struct {
	UniqueCode  string `json:"unique_code" validate:"required"`
	PhoneNumber string `json:"phone_number" validate:"required"`
	Amount      int64  `json:"amount" validate:"amount,gt=0"`
}

type ElectricityValidationPayload struct {
	UniqueCode    string      `json:"unique_code" validate:"required"`
	AccountNumber string      `json:"account_number" validate:"required"`
	AccountType   AccountType `json:"account_type" validate:"required"`
}

type PayElectricityPayload struct {
	UniqueCode    string      `json:"unique_code" validate:"required"`
	AccountNumber string      `json:"account_number" validate:"required"`
	AccountType   AccountType `json:"account_type" validate:"required"`
	Amount        int64       `json:"amount" validate:"required,gt=0"`
	Name          string      `json:"name" validate:"required"`
	Address       string      `json:"address" validate:"required"`
	PhoneNumber   string      `json:"phone_number" validate:"required"`
}

type ValidateCablePayload struct {
	UniqueCode    string `json:"unique_code" validate:"required"`
	AccountNumber string `json:"account_number" validate:"required"`
	NoOfMonth     int    `json:"no_of_month" validate:"required,gt=0"`
}

type PayCablePayload struct {
	UniqueCode    string `json:"unique_code" validate:"required"`
	AccountNumber string `json:"account_number" validate:"required"`
	AccountType   string `json:"account_type" validate:"required"`
	Name          string `json:"name" validate:"required"`
	PhoneNumber   string `json:"phone_number" validate:"required"`
	NoOfMonth     int    `json:"no_of_month" validate:"required,gt=0"`
	Amount        int64  `json:"amount" validate:"required,gt=0"`
}
