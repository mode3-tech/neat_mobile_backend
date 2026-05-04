package response

type APIResponse[T any] struct {
	Status  string    `json:"status"`
	Message string    `json:"message"`
	Data    *T        `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
