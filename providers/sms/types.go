package sms

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

type smsLiveErrorResponse struct {
	Code    int64          `json:"code"`
	Message string         `json:"message"`
	Errors  []smsLiveError `json:"errors"`
}

type smsLiveError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
