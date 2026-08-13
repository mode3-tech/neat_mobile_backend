package loanproduct

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"neat_mobile_app_backend/internal/authchecker"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/phone"
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
	pinVerifier          *authchecker.Verifier
	repaymentTransferrer RepaymentFundTransferrer
	deviceVerifier       DeviceVerifier
	smsSender            SMSSender
	appName              string
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

func NewService(repo *Repository, coreCustomerFinder CoreCustomerFinder, coreLoanFinder CoreLoanFinder, manualRepayer ManualRepayer, pinVerifier *authchecker.Verifier, repaymentTransferrer RepaymentFundTransferrer, deviceVerifier DeviceVerifier, smsSender SMSSender, appName string) *Service {
	return &Service{
		repo:                 repo,
		coreCustomerFinder:   coreCustomerFinder,
		coreLoanFinder:       coreLoanFinder,
		manualRepayer:        manualRepayer,
		pinVerifier:          pinVerifier,
		repaymentTransferrer: repaymentTransferrer,
		deviceVerifier:       deviceVerifier,
		smsSender:            smsSender,
		appName:              appName,
	}
}

func (s *Service) GetAllLoanProducts(ctx context.Context) ([]PartialLoanProduct, error) {
	return s.repo.GetAllLoanProducts(ctx)
}

func (s *Service) ApplyForLoan(ctx context.Context, req LoanRequest, mobileUserID string) (*ApplyForLoanResponse, error) {
	now := time.Now()
	var coreCustomerID int64

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

	if !authchecker.CheckPassword(user.PinHash, req.TransactionPin) {
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
			log.Printf("loan product not found: %v", req.LoanProductType)
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
			log.Printf("loan rule not found: %v", loanProduct.ID)
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

	appliedLoan, err := s.repo.GetAppliedLoans(ctx, mobileUserID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrApplyingForLoan
		}
	}

	if appliedLoan != nil {
		return nil, appErr.ErrUnprocessedAppliedLoanExists
	}

	customer, err := s.repo.GetCoreUser(ctx, user.BVN)
	if err != nil {
		return nil, appErr.ErrApplyingForLoan
	}

	if customer.Id > 0 {
		coreCustomerID = customer.Id

		activeLoans, err := s.repo.CountActiveLoans(ctx, coreCustomerID)
		if err != nil {
			return nil, appErr.ErrApplyingForLoan
		}
		// Product doesn't allow concurrent loans and one already exists
		if !loanProduct.AllowsConcurrentLoans && activeLoans > 0 {
			return nil, appErr.ErrIneligibleForLoan
		}

		// Rule limits max active loans and user is at or above it
		if loanRule.MaxActiveLoans > 0 && activeLoans >= int64(loanRule.MaxActiveLoans) {
			return nil, appErr.ErrIneligibleForLoan
		}

	}

	eoi := &LoanApplication{
		ID:                uuid.NewString(),
		ApplicationRef:    uuid.NewString(),
		CoreCustomerID:    strconv.FormatInt(coreCustomerID, 10),
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

	normalizedPhone, err := phone.NormalizeNigerianNumber(user.Phone)
	if err != nil {
		log.Printf("loan service: failed to normalize phone number - %s\n", err)
		return nil, appErr.ErrApplyingForLoan
	}

	message := fmt.Sprintf("%s: We've received your loan application. Ref: %s. We'll notify you as soon as it's reviewed.", s.appName, eoi.ApplicationRef)
	if err := s.smsSender.Send(ctx, normalizedPhone, message); err != nil {
		log.Printf("loan service: failed to send sms - %s\n", err)
	}

	return &ApplyForLoanResponse{
		ApplicationRef: eoi.ApplicationRef,
		LoanStatus:     eoi.LoanStatus,
		Summary:        *summary,
	}, nil
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

	var allLoans []CoreCustomerLoanItem

	embryoLoans, err := s.repo.ListEmbryoLoanApplications(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrApplyingForLoan
	}

	allLoans = append(allLoans, embryoLoans...)

	if user.CoreCustomerID != nil {

		cbaLoans, err := s.repo.ListLoansByCustomerID(ctx, *user.CoreCustomerID)
		if err != nil {
			return nil, appErr.ErrApplyingForLoan
		}

		allLoans = append(allLoans, cbaLoans...)
	}

	if len(allLoans) <= 0 {
		return nil, appErr.ErrNoLoansFound
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

func (s *Service) HasActiveLoans(ctx context.Context, coreCustomerID string) (bool, error) {
	loans, err := s.repo.ListActiveLoansByCustomerID(ctx, coreCustomerID)
	if err != nil {
		return false, err
	}
	return len(loans) > 0, nil
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
