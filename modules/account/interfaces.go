package account

import (
	"context"
	"io"
	"neat_mobile_app_backend/modules/loanproduct"
	"time"
)

type LoanProvider interface {
	GetAllLoans(ctx context.Context, userID, deviceID string) (*loanproduct.AllLoansResponse, error)
}

type UploadService interface {
	UploadDocument(ctx context.Context, key string, body io.ReadSeeker, contentType string) error
	UploadProfilePicture(ctx context.Context, key string, body io.ReadSeeker, contentType string) error
	PresignURL(ctx context.Context, filePath string, ttl time.Duration) (string, error)
	FileURL(key string) string
}
