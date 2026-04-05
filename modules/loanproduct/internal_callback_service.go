package loanproduct

import (
	"context"
	"errors"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"git.sr.ht/~shulhan/pakakeh.go/lib/ascii"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InternalService struct {
	repo *InternalRepository
}

func NewInternalService(repo *InternalRepository) *InternalService {
	return &InternalService{repo: repo}
}

var (
	ErrBadRequest                = errors.New("bad request")
	ErrInvalidStatus             = errors.New("invalid status")
	ErrInvalidTransition         = errors.New("invalid loan status transition")
	ErrApplicationNotFound       = errors.New("loan application not found")
	ErrInvalidMobileUserID       = errors.New("invalid mobile user id")
	ErrCustomerRecordNotFound    = errors.New("customer record not found")
	ErrInvalidCustomerID         = errors.New("invalid customer id")
	ErrCustomerNotFound          = errors.New("customer not found")
	ErrInvalidCustomerTransition = errors.New("invalid customer status transition")
)

func (s *InternalService) GetLoanApplicationsForCBA(ctx context.Context, mobileUserID string) (*GetLoanApplicationsForCBAResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, ErrInvalidMobileUserID
	}

	row, err := s.repo.GetMostRecentEmbryoLoanApplicationForCBA(ctx, mobileUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &GetLoanApplicationsForCBAResponse{
			Count:        0,
			Applications: []CBAListLoanApplicationItem{},
		}, nil
	}
	if err != nil {
		return nil, err
	}

	resp := &GetLoanApplicationsForCBAResponse{
		Count:        1,
		Applications: []CBAListLoanApplicationItem{mapCBAApplicationItem(row)},
	}

	return resp, nil
}

func (s *InternalService) GetLoanApplicationForCBA(ctx context.Context, applicationRef string) (*GetLoanApplicationForCBAResponse, error) {
	applicationRef = strings.TrimSpace(applicationRef)
	if applicationRef == "" {
		return nil, ErrBadRequest
	}

	row, err := s.repo.GetLoanApplicationForCBAByRef(ctx, applicationRef)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApplicationNotFound
	}
	if err != nil {
		return nil, err
	}

	return &GetLoanApplicationForCBAResponse{
		Application: mapCBAApplicationItem(row),
	}, nil
}

func (s *InternalService) GetEmbryoLoanApplicationsForCBA(ctx context.Context, page, limit int) (*GetEmbryoLoanApplicationsForCBAResponse, error) {
	page, limit, offset := normalizeEmbryoLoanApplicationsPagination(page, limit)

	rows, total, err := s.repo.ListEmbryoLoanApplicationSummariesForCBA(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	resp := &GetEmbryoLoanApplicationsForCBAResponse{
		Count:        len(rows),
		Page:         page,
		Limit:        limit,
		Total:        total,
		Applications: make([]CBAEmbryoLoanApplicationItem, 0, len(rows)),
	}

	for _, row := range rows {
		resp.Applications = append(resp.Applications, CBAEmbryoLoanApplicationItem{
			ApplicationRef: strings.TrimSpace(row.ApplicationRef),
			MobileUserID:   strings.TrimSpace(row.MobileUserID),
			Name:           buildDisplayName(row.FirstName, row.MiddleName, row.LastName),
			Gender:         valueOrEmpty(row.Gender),
			PhoneNumber:    strings.TrimSpace(row.PhoneNumber),
			LoanStatus:     strings.TrimSpace(row.LoanStatus),
			CustomerStatus: normalizeCustomerStatusString(row.CustomerStatus),
		})
	}

	return resp, nil
}

func (s *InternalService) GetLoanApplicationBVNRecordForCBA(ctx context.Context, mobileUserID string) (*GetLoanApplicationBVNRecordForCBAResponse, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)

	if mobileUserID == "" {
		return nil, ErrInvalidMobileUserID
	}

	row, err := s.repo.GetLoanApplicationBVNRecordForCBA(ctx, mobileUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCustomerRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	return &GetLoanApplicationBVNRecordForCBAResponse{
		Record: CBABVNRecordReadDTO{
			ApplicationRef:         strings.TrimSpace(row.ApplicationRef),
			BVN:                    valueOrEmpty(row.BVN),
			FirstName:              valueOrEmpty(row.FirstName),
			MiddleName:             valueOrEmpty(row.MiddleName),
			LastName:               valueOrEmpty(row.LastName),
			Gender:                 valueOrEmpty(row.Gender),
			Nationality:            valueOrEmpty(row.Nationality),
			StateOfOrigin:          valueOrEmpty(row.StateOfOrigin),
			DateOfBirth:            formatDatePtr(row.DateOfBirth),
			EmailAddress:           valueOrEmpty(row.EmailAddress),
			MobilePhone:            valueOrEmpty(row.MobilePhone),
			AlternativeMobilePhone: trimmedPtr(valueOrEmpty(row.AlternativeMobilePhone)),
			BankName:               valueOrEmpty(row.BankName),
			FullHomeAddress:        valueOrEmpty(row.FullHomeAddress),
			PassportOnBVN:          valueOrEmpty(row.PassportOnBVN),
			City:                   trimmedPtr(valueOrEmpty(row.City)),
			Landmark:               trimmedPtr(valueOrEmpty(row.Landmark)),
			WalletBankName:         trimmedPtr(valueOrEmpty(row.WalletBankName)),
			WalletAccountNumber:    trimmedPtr(valueOrEmpty(row.WalletAccountNumber)),
			WalletBankCode:         trimmedPtr(valueOrEmpty(row.WalletBankCode)),
		},
	}, nil
}

func (s *InternalService) ApplyCBACustomerUpdate(ctx context.Context, customerID string, req UpdateCustomerRequest, rawPayload []byte) error {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" || strings.TrimSpace(req.EventID) == "" {
		return ErrBadRequest
	}
	if !ascii.IsDigits([]byte(customerID)) {
		return ErrInvalidCustomerID
	}

	status, ok := parseCustomerStatusValue(req.Status)
	if !ok {
		return ErrInvalidStatus
	}

	now := time.Now().UTC()

	return s.repo.WithTx(ctx, func(repo *InternalRepository) error {
		user, err := repo.GetUserByCoreCustomerIDForUpdate(ctx, customerID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCustomerNotFound
		}
		if err != nil {
			return err
		}

		if normalizedCustomerStatus(user.CustomerStatus) != status && !canTransitionCustomerStatus(user.CustomerStatus, status) {
			return ErrInvalidCustomerTransition
		}

		created, err := repo.InsertCustomerEvent(ctx, &CustomerEvent{
			ID:             uuid.NewString(),
			EventID:        strings.TrimSpace(req.EventID),
			CoreCustomerID: customerID,
			Status:         status,
			Username:       req.Username,
			RawPayload:     string(rawPayload),
			ProcessedAt:    now,
		})
		if err != nil || !created {
			return err
		}

		if sameCustomerStatusPtr(user.CustomerStatus, status) {
			return nil
		}

		return repo.UpdateUserCustomer(ctx, customerID, req.Username, status)
	})
}

func (s *InternalService) ApplyCBAStatusUpdate(ctx context.Context, applicationRef string, req UpdateLoanApplicationStatusRequest, rawPayload []byte) error {
	applicationRef = strings.TrimSpace(applicationRef)
	if applicationRef == "" || strings.TrimSpace(req.EventID) == "" {
		return ErrBadRequest
	}

	status := LoanStatus(strings.TrimSpace(req.Status))
	if !isAllowedCallbackStatus(status) {
		return ErrInvalidStatus
	}

	coreLoanID := trimmedPtr(req.CoreLoanID)
	if status == LoanStatusActive && coreLoanID == nil {
		return errors.New("core_loan_id is required when status is converted_to_loan")
	}

	now := time.Now().UTC()

	return s.repo.WithTx(ctx, func(repo *InternalRepository) error {
		app, err := repo.GetApplicationByRefForUpdate(ctx, applicationRef)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFound
		}
		if err != nil {
			return err
		}

		if app.LoanStatus != status && !canTransition(app.LoanStatus, status) {
			return ErrInvalidTransition
		}

		created, err := repo.InsertStatusEvent(ctx, &LoanApplicationStatusEvent{
			ID:             uuid.NewString(),
			EventID:        strings.TrimSpace(req.EventID),
			ApplicationRef: applicationRef,
			Status:         status,
			CoreLoanID:     coreLoanID,
			RawPayload:     string(rawPayload),
			ProcessedAt:    now,
		})
		if err != nil || !created {
			return err
		}

		if app.LoanStatus == status && sameStringPtr(app.CoreLoanID, coreLoanID) {
			return nil
		}

		return repo.UpdateApplicationStatus(ctx, applicationRef, status, coreLoanID, now)
	})
}

func (s *InternalService) LinkWalletUserByBVN(ctx context.Context, req LinkWalletUserByBVNRequest) (*LinkWalletUserByBVNResponse, error) {
	customerID := strings.TrimSpace(req.CustomerID)
	bvn := strings.TrimSpace(req.BVN)

	if customerID == "" || bvn == "" {
		return nil, ErrBadRequest
	}
	if !ascii.IsDigits([]byte(customerID)) || !ascii.IsDigits([]byte(bvn)) || len(bvn) != 11 {
		return nil, ErrBadRequest
	}

	linkedUsers, err := s.repo.LinkWalletUserCoreCustomerIDByBVN(ctx, bvn, customerID)
	if err != nil {
		return nil, err
	}

	return &LinkWalletUserByBVNResponse{
		LinkedUsers: linkedUsers,
	}, nil
}

func isAllowedCallbackStatus(s LoanStatus) bool {
	switch s {
	case LoanStatusApproved, LoanStatusDeclined, LoanStatusActive:
		return true
	default:
		return false
	}
}

func canTransition(from, to LoanStatus) bool {
	switch from {
	case LoanStatusPending:
		return to == LoanStatusApproved || to == LoanStatusDeclined
	case LoanStatusApproved:
		return to == LoanStatusActive
	case LoanStatusDeclined, LoanStatusActive:
		return false
	default:
		return false
	}
}

func trimmedPtr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func sameStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return strings.TrimSpace(*a) == strings.TrimSpace(*b)
}

func sameCustomerStatusPtr(current *models.CustomerStatus, next models.CustomerStatus) bool {
	return normalizedCustomerStatus(current) == next
}

var loanProductTypeNames = map[string]string{
	"BUSINESS-WK":   "Product Loan Business",
	"GROUP-WK":      "Group Loan",
	"INDIVIDUAL-WK": "Individual Loan",
	"SALARY-MTH":    "Product Loan Salary",
	"SME-WK":        "SME Loan",
	"SPECIAL-WK":    "Special Loan",
}

func loanProductTypeName(code string) string {
	if name, ok := loanProductTypeNames[code]; ok {
		return name
	}
	return code
}

func mapCBAApplicationItem(row *cbaApplicationReadRow) CBAListLoanApplicationItem {
	coreCustomerID := row.ApplicationCoreCustomerID
	if coreCustomerID == nil {
		coreCustomerID = row.UserCoreCustomerID
	}

	return CBAListLoanApplicationItem{
		ApplicationRef: row.ApplicationRef,
		Loan: CBALoanApplicationReadDTO{
			ApplicationRef:    row.ApplicationRef,
			MobileUserID:      row.MobileUserID,
			CoreCustomerID:    coreCustomerID,
			Username:          valueOrEmpty(row.UserUsername),
			PhoneNumber:       row.PhoneNumber,
			Name:              buildDisplayName(row.FirstName, row.MiddleName, row.LastName),
			LoanProductType:   loanProductTypeName(row.LoanProductType),
			BusinessStartDate: row.BusinessStartDate,
			BusinessAddress:   row.BusinessAddress,
			BusinessValue:     row.BusinessValue,
			BusinessType:      row.BusinessType,
			RequestedAmount:   row.RequestedAmount,
			LoanStatus:        row.LoanStatus,
			Tenure:            row.Tenure,
			TenureValue:       row.TenureValue,
		},
	}
}

func buildDisplayName(parts ...*string) string {
	trimmedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := valueOrEmpty(part); value != "" {
			trimmedParts = append(trimmedParts, value)
		}
	}
	return strings.Join(trimmedParts, " ")
}

func parseCustomerStatusValue(value string) (models.CustomerStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(models.CustomerStatusDraft):
		return models.CustomerStatusDraft, true
	case string(models.CustomerStatusPending):
		return models.CustomerStatusPending, true
	case string(models.CustomerStatusApproved):
		return models.CustomerStatusApproved, true
	case string(models.CustomerStatusEmbryo):
		return models.CustomerStatusEmbryo, true
	default:
		return "", false
	}
}

func normalizeCustomerStatusValue(value string) models.CustomerStatus {
	status, ok := parseCustomerStatusValue(value)
	if ok {
		return status
	}
	return models.CustomerStatusEmbryo
}

func normalizeCustomerStatusString(value *string) string {
	normalized := normalizeCustomerStatusValue(valueOrEmpty(value))
	return string(normalized)
}

func normalizedCustomerStatus(value *models.CustomerStatus) models.CustomerStatus {
	if value == nil {
		return models.CustomerStatusEmbryo
	}
	return normalizeCustomerStatusValue(string(*value))
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func normalizeEmbryoLoanApplicationsPagination(page, limit int) (int, int, int) {
	if page < 1 {
		page = defaultEmbryoLoanApplicationsPage
	}
	if limit < 1 {
		limit = defaultEmbryoLoanApplicationsLimit
	}
	if limit > maxEmbryoLoanApplicationsLimit {
		limit = maxEmbryoLoanApplicationsLimit
	}
	return page, limit, (page - 1) * limit
}

func formatDatePtr(v *time.Time) string {
	if v == nil || v.IsZero() {
		return ""
	}
	return v.Format("2006-01-02")
}

func isAllowedCustomerCallbackStatus(status models.CustomerStatus) bool {
	switch status {
	case models.CustomerStatusEmbryo, models.CustomerStatusPending, models.CustomerStatusApproved:
		return true
	default:
		return false
	}
}

func canTransitionCustomerStatus(from *models.CustomerStatus, to models.CustomerStatus) bool {
	switch normalizedCustomerStatus(from) {
	case models.CustomerStatusEmbryo:
		return to == models.CustomerStatusDraft
	case models.CustomerStatusDraft:
		return to == models.CustomerStatusPending || to == models.CustomerStatusApproved
	case models.CustomerStatusApproved:
		return to == models.CustomerStatusPending
	default:
		return false
	}
}
