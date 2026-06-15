package wallet

import (
	"neat_mobile_app_backend/models"
	"testing"
	"time"
)

func TestCheckActivationCap(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	withinCap := &models.User{
		ActivationCapAmount:    200000,
		ActivationCapExpiresAt: &future,
	}
	exceededCap := &models.User{
		ActivationCapAmount:    200000,
		ActivationCapExpiresAt: &future,
	}
	expired := &models.User{
		ActivationCapAmount:    200000,
		ActivationCapExpiresAt: &past,
	}
	noCap := &models.User{ActivationCapExpiresAt: nil}

	tests := []struct {
		name         string
		user         *models.User
		amount       int64
		alreadySpent int64
		wantErr      bool
	}{
		{"within cap", withinCap, 50000, 100000, false},
		{"exactly at cap", withinCap, 0, 200000, false},
		{"one kobo over cap", withinCap, 1, 200000, true},
		{"well over cap", exceededCap, 150000, 200000, true},
		{"cap expired", expired, 500000, 0, false},
		{"no cap set", noCap, 500000, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkActivationCap(now, tt.user, tt.amount, tt.alreadySpent)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkActivationCap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
