package loanproduct

import (
	"context"
	"errors"
	"fmt"
	"math"
	"neat_mobile_app_backend/internal/timeutil"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~shulhan/pakakeh.go/lib/ascii"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo               *Repository
	coreCustomerFinder CoreCustomerFinder
	coreLoanFinder     CoreLoanFinder
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

func NewService(repo *Repository, coreCustomerFinder CoreCustomerFinder, coreLoanFinder CoreLoanFinder) *Service {
	return &Service{
		repo:               repo,
		coreCustomerFinder: coreCustomerFinder,
		coreLoanFinder:     coreLoanFinder,
	}
}

func (s *Service) GetAllLoanProducts(ctx context.Context) ([]PartialLoanProduct, error) {
	return s.repo.GetAllLoanProducts(ctx)
}

func (s *Service) ApplyForLoan(ctx context.Context, req LoanRequest, userID string) (*ApplyForLoanResponse, error) {
	now := time.Now()
	var coreCustomerID *string

	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("current user does not exist")
		}
		return nil, err
	}

	if user.TransactionPinLockedUntil != nil {
		if user.TransactionPinLockedUntil.After(now) {
			return nil, newTransactionPinLockedError(now, *user.TransactionPinLockedUntil)
		}

		if user.FailedTransactionAttempts > 0 {
			if err := s.repo.ResetTransactionPinAttempts(ctx, userID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, errors.New("current user does not exist")
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

		if err := s.repo.UpdateTransactionPinAttempts(ctx, userID, failedAttempts, lockedUntil); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("current user does not exist")
			}
			return nil, err
		}

		if lockedUntil != nil {
			return nil, newTooManyTransactionPinAttemptsError(*lockedUntil, now)
		}

		return nil, ErrIncorrectTransactionPin
	}

	if user.FailedTransactionAttempts > 0 || user.TransactionPinLockedUntil != nil {
		if err := s.repo.ResetTransactionPinAttempts(ctx, userID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("current user does not exist")
			}
			return nil, err
		}
	}

	if user.DOB == nil {
		return nil, errors.New("dob is required")
	}

	userAge := timeutil.AgeFromDOB(*user.DOB, now)

	if userAge < 18 {
		return nil, errors.New("user is below the legal age to borrow a loan")
	}

	parsedAmount, err := strconv.ParseInt(req.LoanAmount, 10, 64)

	if err != nil {
		return nil, errors.New("invalid loan amount")
	}

	loanProduct, err := s.repo.GetLoanProductWithCode(ctx, req.LoanProductType)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("loan product type is invalid")
		}
		return nil, err
	}

	summary, parsedBV, parsedAmount, businessAgeYears, err := s.buildLoanSummary(req, loanProduct, now)
	if err != nil {
		return nil, err
	}

	loanRule, err := s.repo.GetRuleByProductID(ctx, loanProduct.ID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("loan rule not found")
		}
		return nil, err
	}

	if loanRule.RequireBVN != nil && *loanRule.RequireBVN {
		if !user.IsBVNVerified || strings.TrimSpace(user.BVN) == "" {
			return nil, errors.New("user's bvn is not verified")
		}
	}

	if loanRule.RequireNIN != nil && *loanRule.RequireNIN {
		if !user.IsNINVerified || strings.TrimSpace(user.NIN) == "" {
			return nil, errors.New("user's nin is not verified")
		}
	}

	if loanRule.RequirePhoneVerified != nil && *loanRule.RequirePhoneVerified {
		if !user.IsPhoneVerified {
			return nil, errors.New("user's phone is not verified")
		}
	}

	if parsedAmount > loanProduct.MaxLoanAmount || parsedAmount < loanProduct.MinLoanAmount {
		return nil, errors.New("loan amount must be in the range of the min and max amount of selected loan product")
	}

	if businessAgeYears < 1 {
		return nil, errors.New("business must be at least a year old")
	}

	// Core matching is best-effort. A locally registered user may not exist in CBA yet.
	coreCustomerID, err = s.resolveCoreCustomerIDIfAvailable(ctx, userID, user)
	if err != nil {
		return nil, err
	}

	if coreCustomerID != nil {
		customerLoans, err := s.getCoreCustomerLoans(ctx, *coreCustomerID)
		if err != nil {
			return nil, err
		}

		activeLoanCount := countActiveCoreLoans(customerLoans)
		if exceedsMaxActiveLoans(activeLoanCount, loanRule.MaxActiveLoans) {
			return nil, errors.New("customer has reached the maximum number of active loans")
		}

		if loanRule.RequireNoOutstandingDefault != nil && *loanRule.RequireNoOutstandingDefault {
			for _, loan := range customerLoans {
				if !shouldInspectLoanForOutstandingDefault(loan) {
					continue
				}

				loanDetail, err := s.GetCoreLoanDetail(ctx, loan.LoanID)
				if err != nil {
					return nil, err
				}

				if hasOutstandingDefaultLoan(loanDetail) {
					return nil, errors.New("customer has an outstanding defaulted loan")
				}
			}
		}
	}

	eoi := &LoanApplication{
		ID:              uuid.NewString(),
		ApplicationRef:  uuid.NewString(),
		CoreCustomerID:  coreCustomerID,
		PhoneNumber:     user.Phone,
		MobileUserID:    userID,
		LoanProductType: req.LoanProductType,
		LoanStatus:      LoanStatusEmbryo,
		BusinessAddress: req.BusinessAddress,
		BusinessValue:   parsedBV,
		RequestedAmount: parsedAmount,
		Tenure:          loanProduct.RepaymentFrequency,
		TenureValue:     loanProduct.LoanTermValue,
	}

	if err := s.repo.CreateEOI(ctx, eoi); err != nil {
		return nil, err
	}

	return &ApplyForLoanResponse{
		Message:        "loan application was successful",
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
		id := strings.TrimSpace(*user.CoreCustomerID)
		return &id, nil
	}

	if !user.IsBVNVerified || strings.TrimSpace(user.BVN) == "" || s.coreCustomerFinder == nil {
		return nil, nil
	}

	match, err := s.MatchCoreCustomerByBVN(ctx, user.BVN)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, errors.New("core app returned empty customer match response")
	}

	switch match.MatchStatus {
	case CoreCustomerNoMatch, CoreCustomerMultipleMatches:
		return nil, nil
	case CoreCustomerSingleMatch:
		if match.Customer == nil || strings.TrimSpace(match.Customer.CustomerID) == "" {
			return nil, errors.New("core app returned empty matched customer")
		}

		id := strings.TrimSpace(match.Customer.CustomerID)
		if err := s.repo.UpdateUserCoreCustomerID(ctx, userID, id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("current user does not exist")
			}
			return nil, err
		}

		return &id, nil
	default:
		return nil, errors.New("an error occured while looking up customer on the core app")
	}
}

func (s *Service) buildLoanSummary(req LoanRequest, product *LoanProduct, now time.Time) (*LoanSummaryResponse, int64, int64, int, error) {
	businessValue, err := strconv.ParseInt(strings.TrimSpace(req.BusinessValue), 10, 64)
	if err != nil {
		return nil, 0, 0, 0, errors.New("invalid business value")
	}

	loanAmount, err := strconv.ParseInt(strings.TrimSpace(req.LoanAmount), 10, 64)
	if err != nil {
		return nil, 0, 0, 0, errors.New("invalid loan amount")
	}

	startDate, err := timeutil.ParseDOB(req.BusinessStartDate)
	if err != nil {
		return nil, 0, 0, 0, errors.New(err.Error())
	}

	businessAgeYears := timeutil.AgeFromDOB(startDate, now)

	if product.LoanTermValue <= 0 {
		return nil, 0, 0, 0, errors.New("loan term must be greater than zero")
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
		return nil, errors.New("bvn is required")
	}

	if !ascii.IsDigits([]byte(bvn)) || len(bvn) != 11 {
		return nil, errors.New("invalid bvn number")
	}

	if s.coreCustomerFinder == nil {
		return nil, errors.New("core customer finder is not configured")
	}

	fmt.Println(bvn)

	return s.coreCustomerFinder.MatchCustomerByBVN(ctx, bvn)
}

func (s *Service) GetAllLoans(ctx context.Context, userID string) ([]CoreCustomerLoanItem, error) {
	userID = strings.TrimSpace(userID)

	if userID == "" {
		return nil, errors.New("invalid user id")
	}

	user, err := s.repo.GetUser(ctx, userID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no user found")
		}

		return nil, err
	}

	if user == nil || user.CoreCustomerID == nil {
		return nil, errors.New("user has not existing loan")
	}

	loans, err := s.getCoreCustomerLoans(ctx, *user.CoreCustomerID)

	if err != nil {
		return nil, errors.New(err.Error())
	}

	return loans, nil

}

func (s *Service) GetLoanRepayments(ctx context.Context, loanID string) (*[]LoanRepayment, error) {
	loanID = strings.TrimSpace(loanID)

	if loanID == "" {
		return nil, errors.New("invalid loan id")
	}

	if s.coreLoanFinder == nil {
		return nil, errors.New("core loan finder is  not configured")
	}

	return s.coreLoanFinder.GetLoanRepayments(ctx, loanID)
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
