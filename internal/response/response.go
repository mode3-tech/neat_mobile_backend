package response

import "time"

type APIResponse[T any] struct {
	Status     string     `json:"status"`
	Message    string     `json:"message,omitempty"`
	Data       *T         `json:"data,omitempty"`
	Error      *APIError  `json:"error,omitempty"`
	NextCursor *time.Time `json:"next_cursor,omitempty"`
	Page       *int       `json:"page,omitempty"`
	Size       *int       `json:"size,omitempty"`
	Limit      *int       `json:"limit,omitempty"`
	TotalCount *int       `json:"total_count,omitempty"`
	TotalPages *int       `json:"total_pages,omitempty"`
	HasNext    *bool      `json:"has_next,omitempty"`
	HasPrev    *bool      `json:"has_prev,omitempty"`
	Total      *int64     `json:"total,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
