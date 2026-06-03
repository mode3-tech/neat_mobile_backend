package loanproduct

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/pinverifier"
	"neat_mobile_app_backend/internal/timeutil"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~shulhan/pakakeh.go/lib/ascii"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo                 *Repository
	coreCustomerFinder   CoreCustomerFinder
	coreLoanFinder       CoreLoanFinder
	manualRepayer        ManualRepayer
	pinVerifier          *pinverifier.Verifier
	repaymentTransferrer RepaymentFundTransferrer
	deviceVerifier       DeviceVerifier
}

const (
	maxTransactionPinAttempts  = 5
	transactionPinLockDuration = 15 * time.Minute
)

var (
	ErrIncorrectTransactionPin         = errors.New("incorrect transaction pin")
	ErrTooManyTransactionPinAttempts   = errors.New("too many incorrect transaction pin attempts")
	ErrTransactionPinTemporarilyLocked = errors.New("transaction pin is temporarily locked")
)

func NewService(repo *Repository, coreCustomerFinder CoreCustomerFinder, coreLoanFinder CoreLoanFinder, manualRepayer ManualRepayer, pinVerifier *pinverifier.Verifier, repaymentTransferrer RepaymentFundTransferrer, deviceVerifier DeviceVerifier) *Service {
	return &Service{
		repo:                 repo,
		coreCustomerFinder:   coreCustomerFinder,
		coreLoanFinder:       coreLoanFinder,
		manualRepayer:        manualRepayer,
		pinVerifier:          pinVerifier,
		repaymentTransferrer: repaymentTransferrer,
		deviceVerifier:       deviceVerifier,
	}
}

func (s *Service) GetAllLoanProducts(ctx context.Context) ([]PartialLoanProduct, error) {
	return s.repo.GetAllLoanProducts(ctx)
}

func (s *Service) ApplyForLoan(ctx context.Context, req LoanRequest, mobileUserID string) (*ApplyForLoanResponse, error) {
	now := time.Now()
	var coreCustomerID *string

	user, err := s.repo.GetUser(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrUnauthorized
		}
		return nil, appErr.ErrApplyingForLoan
	}

	if user.TransactionPinLockedUntil != nil {
		if user.TransactionPinLockedUntil.After(now) {
			return nil, newTransactionPinLockedError(now, *user.TransactionPinLockedUntil)
		}

		if user.FailedTransactionAttempts > 0 {
			if err := s.repo.ResetTransactionPinAttempts(ctx, mobileUserID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, appErr.ErrUnauthorized
				}
				return nil, err
			}
			user.FailedTransactionAttempts = 0
			user.TransactionPinLockedUntil = nil
		}
	}

	if !CheckPassword(user.PinHash, req.TransactionPin) {
		failedAttempts := user.FailedTransactionAttempts + 1
		var lockedUntil *time.Time
		if failedAttempts >= maxTransactionPinAttempts {
			until := now.Add(transactionPinLockDuration)
			lockedUntil = &until
		}

		if err := s.repo.UpdateTransactionPinAttempts(ctx, mobileUserID, failedAttempts, lockedUntil); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, appErr.ErrUnauthorized
			}
			return nil, err
		}

		if lockedUntil != nil {
			return nil, newTooManyTransactionPinAttemptsError(*lockedUntil, now)
		}

		return nil, appErr.ErrIncorrectTransactionPin
	}

	if user.FailedTransactionAttempts > 0 || user.TransactionPinLockedUntil != nil {
		if err := s.repo.ResetTransactionPinAttempts(ctx, mobileUserID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, appErr.ErrUnauthorized
			}
			return nil, err
		}
	}

	if user.DOB == nil {
		return nil, appErr.ErrInvalidDOB
	}

	userAge := timeutil.AgeFromDOB(*user.DOB, now)

	if userAge < 18 {
		return nil, appErr.ErrUnderaged
	}

	parsedAmount, err := strconv.ParseInt(req.LoanAmount, 10, 64)

	if err != nil {
		return nil, appErr.ErrInvalidLoanAmount
	}

	loanProduct, err := s.repo.GetLoanProductWithCode(ctx, req.LoanProductType)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrInvalidLoanProduct
		}
		return nil, appErr.ErrApplyingForLoan
	}

	summary, parsedBV, parsedAmount, businessAgeYears, err := s.buildLoanSummary(req, loanProduct, now)
	if err != nil {
		return nil, appErr.ErrApplyingForLoan
	}

	loanRule, err := s.repo.GetRuleByProductID(ctx, loanProduct.ID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrInvalidLoanProduct
		}
		return nil, appErr.ErrApplyingForLoan
	}

	if loanRule.RequireBVN != nil && *loanRule.RequireBVN {
		if !user.IsBVNVerified || strings.TrimSpace(user.BVN) == "" {
			return nil, appErr.ErrIncompleteKYC
		}
	}

	if loanRule.RequireNIN != nil && *loanRule.RequireNIN {
		if !user.IsNINVerified || strings.TrimSpace(user.NIN) == "" {
			return nil, appErr.ErrIncompleteKYC
		}
	}

	if loanRule.RequirePhoneVerified != nil && *loanRule.RequirePhoneVerified {
		if !user.IsPhoneVerified {
			return nil, appErr.ErrIncompleteKYC
		}
	}

	if parsedAmount > loanProduct.MaxLoanAmount || parsedAmount < loanProduct.MinLoanAmount {
		return nil, appErr.ErrInvalidLoanAmount
	}

	if businessAgeYears < 1 {
		return nil, appErr.ErrIneligibleBusinessAge
	}

	// Core matching is best-effort. A locally registered user may not exist in CBA yet.
	coreCustomerID, err = s.resolveCoreCustomerIDIfAvailable(ctx, mobileUserID, user)
	if err != nil {
		return nil, appErr.ErrApplyingForLoan
	}

	if coreCustomerID != nil {
		customerLoans, err := s.getCoreCustomerLoans(ctx, *coreCustomerID)
		if err != nil {
			customerLoans = []CoreCustomerLoanItem{}
		}

		activeLoanCount := countActiveCoreLoans(customerLoans)
		if exceedsMaxActiveLoans(activeLoanCount, loanRule.MaxActiveLoans) {
			return nil, appErr.ErrIneligibleForLoan
		}

		if loanRule.RequireNoOutstandingDefault != nil && *loanRule.RequireNoOutstandingDefault {
			for _, loan := range customerLoans {
				if !shouldInspectLoanForOutstandingDefault(loan) {
					continue
				}

				loanDetail, err := s.GetCoreLoanDetail(ctx, loan.LoanID)
				if err != nil {
					return nil, appErr.ErrApplyingForLoan
				}

				if hasOutstandingDefaultLoan(loanDetail) {
					return nil, appErr.ErrIneligibleForLoan
				}
			}
		}
	}

	eoi := &LoanApplication{
		ID:                uuid.NewString(),
		ApplicationRef:    uuid.NewString(),
		CoreCustomerID:    coreCustomerID,
		PhoneNumber:       user.Phone,
		MobileUserID:      mobileUserID,
		LoanProductType:   req.LoanProductType,
		LoanStatus:        LoanStatusEmbryo,
		BusinessAddress:   req.BusinessAddress,
		BusinessValue:     parsedBV,
		BusinessStartDate: req.BusinessStartDate,
		RequestedAmount:   parsedAmount,
		Tenure:            loanProduct.RepaymentFrequency,
		TenureValue:       loanProduct.LoanTermValue,
	}

	if err := s.repo.CreateEOI(ctx, eoi); err != nil {
		return nil, err
	}

	return &ApplyForLoanResponse{
		ApplicationRef: eoi.ApplicationRef,
		LoanStatus:     eoi.LoanStatus,
		Summary:        *summary,
	}, nil
}

func (s *Service) resolveCoreCustomerIDIfAvailable(ctx context.Context, userID string, user *row) (*string, error) {
	if user == nil {
		return nil, nil
	}

	if user.CoreCustomerID != nil && strings.TrimSpace(*user.CoreCustomerID) != "" {
		coreCustomerID := strings.TrimSpace(*user.CoreCustomerID)
		return &coreCustomerID, nil
	}

	if !user.IsBVNVerified || strings.TrimSpace(user.BVN) == "" || s.coreCustomerFinder == nil {
		return nil, nil
	}

	match, err := s.MatchCoreCustomerByBVN(ctx, user.BVN)
	if err != nil {
		log.Printf("error matching core customer by BVN user_id=%s bvn=%s err=%v", userID, user.BVN, err)
		return nil, appErr.ErrApplyingForLoan
	}
	if match == nil {
		log.Printf("no core customer match found for user_id=%s bvn=%s", userID, user.BVN)
		return nil, appErr.ErrApplyingForLoan
	}

	switch match.MatchStatus {
	case CoreCustomerNoMatch, CoreCustomerMultipleMatches:
		return nil, nil
	case CoreCustomerSingleMatch:
		if match.Customer == nil || strings.TrimSpace(match.Customer.CustomerID) == "" {
			log.Printf("core customer match has no customer id user_id=%s bvn=%s", userID, user.BVN)
			return nil, appErr.ErrApplyingForLoan
		}

		coreCustomerID := strings.TrimSpace(match.Customer.CustomerID)
		if err := s.repo.UpdateUserCoreCustomerID(ctx, userID, coreCustomerID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("core customer match found but user not found when updating core customer id user_id=%s bvn=%s err=%v", userID, user.BVN, err)
				return nil, appErr.ErrUnauthorized
			}
			log.Printf("error updating user core customer id user_id=%s err=%v", userID, err)
			return nil, appErr.ErrApplyingForLoan
		}

		return &coreCustomerID, nil
	default:
		log.Printf("unknown core customer match status user_id=%s bvn=%s status=%s", userID, user.BVN, match.MatchStatus)
		return nil, appErr.ErrApplyingForLoan
	}
}

func (s *Service) buildLoanSummary(req LoanRequest, product *LoanProduct, now time.Time) (*LoanSummaryResponse, int64, int64, int, error) {
	businessValue, err := strconv.ParseInt(strings.TrimSpace(req.BusinessValue), 10, 64)
	if err != nil {
		return nil, 0, 0, 0, appErr.ErrInvalidBusinessValue
	}

	loanAmount, err := strconv.ParseInt(strings.TrimSpace(req.LoanAmount), 10, 64)
	if err != nil {
		return nil, 0, 0, 0, appErr.ErrInvalidLoanAmount
	}

	startDate, err := timeutil.ParseDOB(req.BusinessStartDate)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	businessAgeYears := timeutil.AgeFromDOB(startDate, now)

	if product.LoanTermValue <= 0 {
		return nil, 0, 0, 0, appErr.ErrInvalidLoanTerm
	}

	ratePercent := float64(product.InterestRateBPS) // current repo meaning: 24 => 24%
	interestAmount := float64(loanAmount) * ratePercent / 100
	totalRepayment := float64(loanAmount) + interestAmount
	periodicRepayment := totalRepayment / float64(product.LoanTermValue)

	return &LoanSummaryResponse{
		BusinessValue:       businessValue,
		BusinessAgeYears:    businessAgeYears,
		LoanAmount:          loanAmount,
		InterestRatePercent: ratePercent,
		InterestAmount:      round2(interestAmount),
		TotalRepayment:      round2(totalRepayment),
		PeriodicRepayment:   round2(periodicRepayment),
		LoanTermValue:       product.LoanTermValue,
		RepaymentFrequency:  product.RepaymentFrequency,
		IsEstimate:          true,
	}, businessValue, loanAmount, businessAgeYears, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func (s *Service) MatchCoreCustomerByBVN(ctx context.Context, bvn string) (*CoreCustomerMatchData, error) {
	bvn = strings.TrimSpace(bvn)

	if bvn == "" {
		return nil, appErr.ErrInvalidBVN
	}

	if !ascii.IsDigits([]byte(bvn)) || len(bvn) != 11 {
		return nil, appErr.ErrInvalidBVN
	}

	if s.coreCustomerFinder == nil {
		log.Print("core customer finder not configured")
		return nil, appErr.ErrApplyingForLoan
	}

	return s.coreCustomerFinder.MatchCustomerByBVN(ctx, bvn)
}

func (s *Service) GetAllLoans(ctx context.Context, mobileUserID string) ([]CoreCustomerLoanItem, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	if mobileUserID == "" {
		return nil, appErr.ErrUnauthorized
	}

	user, err := s.repo.GetUser(ctx, mobileUserID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrUnauthorized
		}

		return nil, err
	}

	if user == nil || user.CoreCustomerID == nil {
		return nil, appErr.ErrNoLoansFound
	}

	allLoans, err := s.repo.ListLoansByCustomerID(ctx, *user.CoreCustomerID)
	if err != nil {
		return nil, appErr.ErrApplyingForLoan
	}

	return allLoans, nil
}

func (s *Service) GetActiveLoans(ctx context.Context, mobileUserID string) ([]ActiveLoanItem, error) {
	user, err := s.repo.GetUser(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrUnauthorized
		}
		return nil, appErr.ErrFetchingActiveLoans
	}

	if user == nil || user.CoreCustomerID == nil {
		return []ActiveLoanItem{}, nil
	}

	activeLoans, err := s.repo.ListActiveLoansByCustomerID(ctx, *user.CoreCustomerID)
	if err != nil {
		return nil, appErr.ErrFetchingActiveLoans
	}

	return activeLoans, nil
}

func (s *Service) GetLoanHistory(ctx context.Context, mobileUserID string) ([]LoanHistoryItem, error) {
	user, err := s.repo.GetUser(ctx, mobileUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrUnauthorized
		}
		log.Print("error getting user from the db")
		return nil, appErr.ErrFetchingLoanHistory
	}

	if user.CoreCustomerID == nil {
		return nil, appErr.ErrNoLoansFound
	}

	history, err := s.repo.GetLoanRepaymentHistory(ctx, *user.CoreCustomerID)
	if err != nil {
		log.Printf("error fetching loan history for user_id=%s core_customer_id=%s err=%v", mobileUserID, *user.CoreCustomerID, err)
		return nil, appErr.ErrFetchingLoanHistory
	}

	return history, nil
}

func (s *Service) GetLoanDetails(ctx context.Context, mobileUserID, loanID string) (*LoanDetailsResponse, error) {

	details, err := s.repo.GetLoanDetailsByID(ctx, loanID)
	if err != nil {
		return nil, appErr.ErrFetchingLoanDetails
	}

	history, err := s.repo.GetRecentLoanRepaymentHistory(ctx, loanID)
	if err != nil {
		return nil, appErr.ErrFetchingLoanDetails
	}

	details.RepaymentHistory = history

	return &LoanDetailsResponse{
		Details: *details,
	}, nil
}

func (s *Service) GetLoanHistoryByLoanID(ctx context.Context, mobileUserID, loanID string) ([]LoanHistoryItem, error) {
	history, err := s.repo.GetLoanRepaymentHistoryByLoanID(ctx, loanID)
	if err != nil {
		return nil, appErr.ErrFetchingLoanHistory
	}

	return history, nil
}

func (s *Service) GetLoanRepayments(ctx context.Context, userID, loanID string) (*LoanRepaymentResponse, error) {
	repaymentSummary, err := s.repo.GetLoanRepaymentSummary(ctx, loanID)
	if err != nil {
		return nil, err
	}

	return &LoanRepaymentResponse{
		Repayment: *repaymentSummary,
	}, nil
}

func (s *Service) getCoreCustomerLoans(ctx context.Context, customerID string) ([]CoreCustomerLoanItem, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return nil, errors.New("customer id is required")
	}
	if !ascii.IsDigits([]byte(customerID)) {
		return nil, errors.New("invalid customer id")
	}
	if s.coreLoanFinder == nil {
		return nil, errors.New("core loan finder is not configured")
	}

	return s.coreLoanFinder.GetCustomerLoans(ctx, customerID)
}

func (s *Service) GetCoreLoanDetail(ctx context.Context, loanID string) (*CoreLoanDetail, error) {
	loanID = strings.TrimSpace(loanID)
	if loanID == "" {
		return nil, errors.New("loan id is required")
	}
	if !ascii.IsDigits([]byte(loanID)) {
		return nil, errors.New("invalid loan id")
	}
	if s.coreLoanFinder == nil {
		return nil, errors.New("core loan finder is not configured")
	}

	return s.coreLoanFinder.GetLoanDetail(ctx, loanID)
}

func (s *Service) MakeManualRepayment(ctx context.Context, mobileUserID string, req ManualRepaymentRequest) error {
	if err := s.pinVerifier.Verify(ctx, mobileUserID, req.TransactionPin); err != nil {
		log.Printf("manual repayment pin verification failed user=%s err=%v", mobileUserID, err)
		return err
	}

	if s.manualRepayer == nil {
		log.Print("manual repayment service not configured")
		return errors.New("repayment service is not configured")
	}

	if s.repaymentTransferrer == nil {
		log.Print("repayment fund transferrer not configured")
		return errors.New("wallet service is not configured")
	}

	if err := s.repaymentTransferrer.TransferForLoanRepayment(ctx, mobileUserID, req.Amount); err != nil {
		log.Printf("manual repayment wallet transfer failed user=%s amount=%d err=%v", mobileUserID, req.Amount, err)
		return appErr.ErrMakingRepayment
	}

	log.Printf("manual repayment wallet transfer ok user=%s amount=%d — calling CBA", mobileUserID, req.Amount)

	err := s.manualRepayer.MakeManualRepayment(ctx, RepaymentRequest{
		Amount:      req.Amount,
		RepaymentID: req.LoanID,
	})
	if err != nil {
		log.Printf("manual repayment CBA call failed user=%s loan_id=%s amount=%d err=%v", mobileUserID, req.LoanID, req.Amount, err)
		return appErr.ErrMakingRepayment
	}

	log.Printf("manual repayment CBA call ok user=%s loan_id=%s", mobileUserID, req.LoanID)
	return nil
}

func newTooManyTransactionPinAttemptsError(lockedUntil, now time.Time) error {
	return fmt.Errorf("%w, try again in %s", ErrTooManyTransactionPinAttempts, remainingLockDuration(now, lockedUntil))
}

func newTransactionPinLockedError(now, lockedUntil time.Time) error {
	return fmt.Errorf("%w, try again in %s", ErrTransactionPinTemporarilyLocked, remainingLockDuration(now, lockedUntil))
}

func remainingLockDuration(now, lockedUntil time.Time) string {
	remaining := lockedUntil.Sub(now)
	if remaining <= 0 {
		return "1 minute"
	}

	minutes := int(math.Ceil(remaining.Minutes()))
	if minutes <= 1 {
		return "1 minute"
	}

	return fmt.Sprintf("%d minutes", minutes)
}
