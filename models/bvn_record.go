package models

import "time"

type BVNRecord struct {
	ID                       string     `gorm:"column:id;type:text;primaryKey"`
	UserID                   string     `gorm:"column:user_id;type:text;index;not null"`
	FirstName                string     `gorm:"column:first_name;type:text;not null"`
	MiddleName               string     `gorm:"column:middle_name;type:text;not null"`
	LastName                 string     `gorm:"column:last_name;type:text;not null"`
	Gender                   string     `gorm:"column:gender;type:text;not null"`
	Nationality              string     `gorm:"column:nationality;type:text;not null"`
	StateOfOrigin            string     `gorm:"column:state_of_origin;type:text;not null"`
	DateOfBirth              time.Time  `gorm:"column:date_of_birth;not null"`
	PlaceOfBirth             string     `gorm:"column:place_of_birth;type:text;not null"`
	Occupation               string     `gorm:"column:occupation;type:text;not null"`
	MaritalStatus            string     `gorm:"column:marital_status;type:text;not null"`
	Education                string     `gorm:"column:education;type:text;not null"`
	Religion                 string     `gorm:"column:religion;type:text;not null"`
	EmailAddress             string     `gorm:"column:email_address;type:text;index;not null"`
	PassportOnBVN            string     `gorm:"column:passport_on_bvn;type:text;not null"`
	Passport                 *string    `gorm:"column:passport;type:text"`
	FullHomeAddress          string     `gorm:"column:full_home_address;type:text;not null"`
	TypeOfHouse              *string    `gorm:"column:type_of_house;type:text"`
	City                     *string    `gorm:"column:city;type:text"`
	Landmark                 *string    `gorm:"column:landmark;type:text"`
	LivingSince              *time.Time `gorm:"column:living_since"`
	MobilePhone              string     `gorm:"column:mobile_phone;type:text;index;not null"`
	AlternativeMobilePhone   *string    `gorm:"column:alternative_mobile_phone;type:text"`
	IDType                   *string    `gorm:"column:id_type;type:text"`
	IDNumber                 *string    `gorm:"column:id_number;type:text"`
	BankName                 string     `gorm:"column:bank_name;type:text;not null"`
	AccountNumber            *string    `gorm:"column:account_number;type:text;index"`
	BVN                      string     `gorm:"column:bvn;type:text;uniqueIndex;not null"`
	NextOfKinFirstName       *string    `gorm:"column:next_of_kin_first_name;type:text"`
	NextOfKinMiddleName      *string    `gorm:"column:next_of_kin_middle_name;type:text"`
	NextOfKinLastName        *string    `gorm:"column:next_of_kin_last_name;type:text"`
	NextOfKinLandmark        *string    `gorm:"column:next_of_kin_landmark;type:text"`
	NextOfKinPhoneNumber     *string    `gorm:"column:next_of_kin_phone_number;type:text"`
	NextOfKinAddress         *string    `gorm:"column:next_of_kin_address;type:text"`
	NextOfKinRelationship    *string    `gorm:"column:next_of_kin_relationship;type:text"`
	NextOfKinPassport        *string    `gorm:"column:next_of_kin_passport;type:text"`
	ContactPerson            *string    `gorm:"column:contact_person;type:text"`
	ContactPersonPhoneNumber *string    `gorm:"column:contact_person_phone_number;type:text"`
	CustomerSignature        *string    `gorm:"column:customer_signature;type:text"`
}

func (BVNRecord) TableName() string {
	return "wallet_bvn_records"
}
