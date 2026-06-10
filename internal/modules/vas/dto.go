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

type BillingsByCategoryIDPayload struct {
	CategoryID int `json:"category_id" validate:"required,gt=0"`
}

type FetchProductsQuery struct {
	CategoryID int `form:"category_id" binding:"required,gt=0"`
	BillerID   int `form:"biller_id" binding:"required,gt=0"`
	Size       int `form:"size" binding:"required,gt=0"`
	Page       int `form:"page" binding:"required,gt=0"`
}

type FetchBillersByCategoryIDQuery struct {
	CategoryID int `form:"category_id" binding:"required,gt=0"`
	Size       int `form:"size" binding:"required,gt=0"`
	Page       int `form:"page" binding:"required,gt=0"`
}

type BillingsByCategoryIDResponse []Biller

type Biller struct {
	ID           int              `json:"id"`
	Name         string           `json:"name"`
	BillerCode   string           `json:"biller_code"`
	Description  string           `json:"description"`
	CategoryDTOs []BillerCategory `json:"category_dtos"`
	Image        string           `json:"image"`
}

type BillerCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type FetchProductsByCategoryIDAndBillerIDPayload struct {
	CategoryID int `json:"category_id" validate:"required,gt=0"`
	BillerID   int `json:"biller_id" validate:"required,gt=0"`
}

type ProductsResponse []Product

type Product struct {
	Name        string  `json:"name"`
	UniqueCode  string  `json:"unique_code"`
	LookUp      bool    `json:"look_up"`
	FixedAmount bool    `json:"fixed_amount"`
	Amount      float32 `json:"amount"`
	MinAmount   float32 `json:"min_amount"`
	MaxAmount   float32 `json:"max_amount"`
	ImageURL    string  `json:"image_url"`
	BillerName  string  `json:"biller_name"`
	CategoryDTO struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"category_dto"`
}
