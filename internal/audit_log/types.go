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
)
