package auditlog

type ResourceType string

const (
	ResourceTypeWallet          ResourceType = "wallet"
	ResourceTypeUser            ResourceType = "user"
	ResourceTypeUserAccount     ResourceType = "user_account"
	ResourceTypeTransaction     ResourceType = "transaction"
	ResourceTypeLoan            ResourceType = "loan"
	ResourceTypeLoanApplication ResourceType = "loan_application"
	ResourceTypeTierUpgrade     ResourceType = "tier_upgrade"
	ResourceTypeDevice          ResourceType = "device"
	ResourceTypeSession         ResourceType = "session"
	ResourceTypeNIN             ResourceType = "nin"
	ResourceTypeBVN             ResourceType = "bvn"
	ResourceTypeBeneficiary     ResourceType = "beneficiary"
	ResourceTypeCard            ResourceType = "card"
	ResourceTypeSavingsGoal     ResourceType = "savings_goal"
)

type LogStatus string

const (
	StatusSuccess LogStatus = "success"
	StatusFailure LogStatus = "failure"
)
