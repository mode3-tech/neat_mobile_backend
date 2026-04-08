package reporting

import (
	"context"
	"math"
)

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 100
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListSignedUsers(ctx context.Context, page, limit int) (*ListSignedUsersResponse, error) {
	if page < 1 {
		page = defaultPage
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := (page - 1) * limit

	rows, total, err := s.repo.ListSignedUsers(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	users := make([]SignedUserItem, 0, len(rows))
	for _, r := range rows {
		item := SignedUserItem{
			ID:             r.ID,
			FirstName:      r.FirstName,
			LastName:       r.LastName,
			MiddleName:     r.MiddleName,
			Email:          r.Email,
			Phone:          r.Phone,
			BVN:            r.BVN,
			CoreCustomerID: r.CoreCustomerID,
			CustomerStatus: r.CustomerStatus,
			Username:       r.Username,
			RegisteredAt:   r.CreatedAt,
			Verified: VerifiedFlags{
				BVN:   r.IsBVNVerified,
				NIN:   r.IsNINVerified,
				Phone: r.IsPhoneVerified,
			},
		}

		if r.LoanStatus != nil && r.LastLoanAppliedAt != nil {
			item.LatestLoan = &LatestLoanSummary{
				Status:    *r.LoanStatus,
				AppliedAt: *r.LastLoanAppliedAt,
			}
		}

		users = append(users, item)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return &ListSignedUsersResponse{
		Users:      users,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}
