package auditlog

type GetAuditLogsQuery struct {
	ActorType    string       `form:"actor_type"`
	ActorID      string       `form:"actor_id"`
	Action       string       `form:"action"`
	ResourceType ResourceType `form:"resource_type"`
	ResourceID   string       `form:"resource_id"`
	Status       LogStatus    `form:"status"`
	RequestID    string       `form:"request_id"`
	IPAddress    string       `form:"ip_address"`
	Page         int          `form:"page"`
	Limit        int          `form:"limit"`
}
