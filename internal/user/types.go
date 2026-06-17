package user

type UserWithAddress struct {
	ID             string  `json:"id"`
	FullName       string  `json:"full_name"`        // first_name || ' ' || last_name
	Phone          string  `json:"phone"`
	Address        *string `json:"address"`           // from wallet_users
	BVNHomeAddress string  `json:"bvn_home_address"`  // from wallet_bvn_records.full_home_address
}
