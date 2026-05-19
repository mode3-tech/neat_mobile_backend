package response

import "time"

type APIResponse[T any] struct {
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	Data       *T        `json:"data,omitempty"`
	Error      *APIError `json:"error,omitempty"`
	NextCursor time.Time `json:"next_cursor,omitempty"`
	Page       *int      `json:"page,omitempty"`
	Limit      *int      `json:"limit,omitempty"`
	Total      *int64    `json:"total,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
