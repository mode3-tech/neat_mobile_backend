package termii

type TermiiSuccessResponse struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	MessageID    string `json:"message_id"`
	User         string `json:"user"`
	MessageIDStr string `json:"message_id_str"`
}

type TermiiErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
	Link    string `json:"link"`
}
