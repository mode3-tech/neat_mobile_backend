package accountclosure

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"neat_mobile_app_backend/internal/database/tx"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/autorepayment"
	"neat_mobile_app_backend/internal/modules/device"
	"neat_mobile_app_backend/internal/types"
	"neat_mobile_app_backend/internal/user"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	repo                *Repository
	loanService         LoanService
	xpressWalletService XpressWalletService
	tx                  *tx.Transactor
	walletService       WalletService
	deviceService       DeviceService
	userService         UserService
}

func NewService(repo *Repository, loanService LoanService, xpressWalletService XpressWalletService, tx *tx.Transactor, walletService WalletService, deviceService DeviceService, userService UserService) *Service {
	return &Service{
		repo:                repo,
		loanService:         loanService,
		xpressWalletService: xpressWalletService,
		tx:                  tx,
		walletService:       walletService,
		deviceService:       deviceService,
		userService:         userService,
	}
}

func (s *Service) CloseAccount(ctx context.Context, payload *AccountClosureRequest, mobileUserID, deviceID, IP, requestID string) (*AccountClosure, error) {
	if _, err := s.repo.GetActiveClosureForUser(ctx, mobileUserID); err == nil {
		return nil, appErr.ErrAccountClosureAlreadyInProgress
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	period := now.Format("200601")

	closure := &AccountClosure{
		ID:              "cls-" + uuid.NewString(),
		UserID:          mobileUserID,
		Status:          AccountClosureStatusChecking,
		RequestedAt:     now,
		RequestedByType: AccountClosureAuthorTypeUser,
		RequestedByID:   mobileUserID,
		ReasonNote:      payload.ReasonNote,
	}

	err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		repo := NewRepository(txDB)

		sequence, err := repo.GetNextClosureSequence(ctx, period)
		if err != nil {
			log.Printf("failed to get closure sequence: %s\n", err)
			return err
		}

		closure.ClosureReference = fmt.Sprintf("CLS-%s-%06d", period, sequence)

		err = repo.CreateAccountClosure(ctx, closure)
		if err != nil {
			log.Printf("failed to create closure: %s\n", err)
			return err
		}

		return repo.CreateAccountClosureEvent(ctx, &AccountClosureEvent{
			ID:            "CLE-" + uuid.NewString(),
			ClosureID:     closure.ID,
			UserID:        mobileUserID,
			EventType:     AccountClosureEventTypeChecksStarted,
			Status:        AccountClosureStatusChecking,
			ActorType:     "account-closure-service",
			ActorID:       mobileUserID,
			ReasonNote:    payload.ReasonNote,
			RequestID:     requestID,
			IPAddress:     IP,
			DeviceID:      &deviceID,
			ClosureSource: "mobile_app",
			Metadata:      types.JSONMap{},
		})
	})

	if err != nil {
		return nil, err
	}

	wallet, err := s.walletService.GetUserWalletBalance(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}

	customerDetails, err := s.xpressWalletService.GetCustomerDetails(ctx, wallet.WalletCustomerID)
	if err != nil {
		return nil, err
	}

	if customerDetails.Customer.AvailableBalance > 0 {

		blockerCode := "positive_wallet_balance"
		blockerDetails := types.JSONMap{
			"available_balance": customerDetails.Customer.AvailableBalance,
			"currency":          "NGN",
		}

		if err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
			repo := NewRepository(txDB)

			if err := repo.MarkAccountClosureAsBlocked(ctx, closure.ID, blockerCode, blockerDetails); err != nil {
				return err
			}

			return repo.CreateAccountClosureEvent(ctx, &AccountClosureEvent{
				ID:            "CLE-" + uuid.NewString(),
				ClosureID:     closure.ID,
				UserID:        mobileUserID,
				EventType:     AccountClosureEventTypeBlocked,
				Status:        AccountClosureStatusBlocked,
				ActorType:     "account-closure-service",
				ActorID:       mobileUserID,
				ReasonNote:    payload.ReasonNote,
				RequestID:     requestID,
				IPAddress:     IP,
				DeviceID:      &deviceID,
				ClosureSource: "mobile_app",
				Metadata:      blockerDetails,
			})
		}); err != nil {
			return nil, err
		}

		closure.Status = AccountClosureStatusBlocked
		closure.BlockerCode = &blockerCode
		closure.BlockerDetails = blockerDetails

		return closure, nil
	}

	userDetails, err := s.userService.GetUserDetails(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}

	ok, err := s.loanService.HasActiveLoans(ctx, *userDetails.CoreCustomerID)
	if err != nil {
		return nil, err
	}
	if ok {
		blockerCode := "active_loans"
		blockerDetails := types.JSONMap{
			"active_loans": true,
		}

		err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
			repo := NewRepository(txDB)
			if err := repo.MarkAccountClosureAsBlocked(ctx, closure.ID, blockerCode, blockerDetails); err != nil {
				return err
			}

			return repo.CreateAccountClosureEvent(ctx, &AccountClosureEvent{
				ID:            "CLE-" + uuid.NewString(),
				ClosureID:     closure.ID,
				UserID:        mobileUserID,
				EventType:     AccountClosureEventTypeBlocked,
				Status:        AccountClosureStatusBlocked,
				ActorType:     "account-closure-service",
				ActorID:       mobileUserID,
				ReasonNote:    payload.ReasonNote,
				RequestID:     requestID,
				IPAddress:     IP,
				DeviceID:      &deviceID,
				ClosureSource: "mobile_app",
				Metadata:      blockerDetails,
			})
		})

		if err != nil {
			return nil, err
		}

		closure.Status = AccountClosureStatusBlocked
		closure.BlockerCode = &blockerCode
		closure.BlockerDetails = blockerDetails

		return closure, nil
	}

	if err := s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		repo := NewRepository(txDB)

		if err := repo.MarkAccountClosureAsProcessing(ctx, closure.ID); err != nil {
			return err
		}

		if err := repo.CreateAccountClosureEvent(ctx, &AccountClosureEvent{
			ID:            "CLE-" + uuid.NewString(),
			ClosureID:     closure.ID,
			UserID:        mobileUserID,
			EventType:     AccountClosureEventTypeProcessing,
			Status:        AccountClosureStatusProcessing,
			ActorType:     "account-closure-service",
			ActorID:       mobileUserID,
			ReasonNote:    payload.ReasonNote,
			RequestID:     requestID,
			DeviceID:      &deviceID,
			IPAddress:     IP,
			ClosureSource: "mobile_app",
			Metadata:      nil,
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.xpressWalletService.CloseWallet(ctx, wallet.WalletCustomerID); err != nil {
		return nil, err
	}

	err = s.tx.WithTx(ctx, func(txDB *gorm.DB) error {
		repo := NewRepository(txDB)
		userRepo := user.NewRepository(txDB)
		deviceRepo := device.NewRepository(txDB)
		authRepo := auth.NewRespository(txDB)
		autoRepaymentRepo := autorepayment.NewRepository(txDB)

		now := time.Now().UTC()
		if err := repo.MarkAccountClosureAsClosed(ctx, closure.ID, now, "success"); err != nil {
			return err
		}

		if err := userRepo.SetUserAsClosed(ctx, mobileUserID, now); err != nil {
			return err
		}

		if err := authRepo.RevokeAllSessions(ctx, mobileUserID, now); err != nil {
			return err
		}

		if err := deviceRepo.DeactivateAllDevices(ctx, mobileUserID); err != nil {
			return err
		}

		if err := autoRepaymentRepo.StopAllAutoRepayments(ctx, mobileUserID); err != nil {
			return err
		}

		return repo.CreateAccountClosureEvent(ctx, &AccountClosureEvent{
			ID:            "CLE-" + uuid.NewString(),
			ClosureID:     closure.ID,
			UserID:        mobileUserID,
			EventType:     AccountClosureEventTypeClosed,
			Status:        AccountClosureStatusClosed,
			ActorType:     string(AccountClosureAuthorTypeUser),
			ActorID:       mobileUserID,
			ReasonNote:    payload.ReasonNote,
			RequestID:     requestID,
			DeviceID:      &deviceID,
			ClosureSource: "mobile_app",
			IPAddress:     IP,
			Metadata:      types.JSONMap{},
		})
	})
	if err != nil {
		log.Printf("CRITICAL: wallet %s was closed at the provider for closure %s but local closure/session cleanup failed — manual reconciliation required: %s\n", wallet.WalletCustomerID, closure.ID, err)
		return nil, err
	}

	closure.Status = AccountClosureStatusClosed
	closure.BlockerCode = nil
	closure.BlockerDetails = nil

	return closure, nil
}
