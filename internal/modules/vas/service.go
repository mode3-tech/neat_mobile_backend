package vas

import (
	"context"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/phone"
	"strings"
)

type Service struct {
	Repo           *Repository
	WalletService  WalletService
	XpressPayments VASService
}

func NewService(repo *Repository, xpressPayments VASService, walletService WalletService) *Service {
	return &Service{Repo: repo, XpressPayments: xpressPayments, WalletService: walletService}
}

func (s *Service) GetAirtime(ctx context.Context, payload AirtimePayload, mobileUserID string) (*ISPResponse, error) {
	requestID := strings.TrimSpace(payload.RequestID)
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	localizedPhone, err := phone.ToLocalFormat(strings.TrimSpace(payload.PhoneNumber))
	if err != nil {
		log.Printf("vas service : failed to normalize phone number - %s\n", err)
		return nil, err
	}
	phoneNumber := localizedPhone
	amount := payload.Amount

	if amount > 50 {
		log.Println("vas service: amount is less than NGN 50")
		return nil, appErr.ErrInvalidISPAmount
	}

	balance, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrGettingAirtime
	}

	if balance < amount*100 {
		log.Println("vas service: insufficient balance")
		return nil, appErr.ErrInsufficientBalance
	}

	result, err := s.XpressPayments.GetAirtime(ctx, requestID, uniqueCode, phoneNumber, amount)
	if err != nil {
		log.Printf("vas service: unable to purchase airtime - %s\n", err)
		return nil, appErr.ErrGettingAirtime
	}

	return result, nil
}
