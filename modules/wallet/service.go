package wallet

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repo            *Repository
	providusService ProvidusService
}

func NewService(repo *Repository, providusService ProvidusService) *Service {
	return &Service{repo: repo, providusService: providusService}
}

func (s *Service) FetchBanks(ctx context.Context) ([]Bank, error) {
	return s.providusService.FetchBanks(ctx)
}

func (s *Service) FetchBankDetails(ctx context.Context, accountNumber, bankCode string) (*BankDetails, error) {
	return s.providusService.FetchBankDetails(ctx, accountNumber, bankCode)
}

func (s *Service) InitiateTransfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error) {
	if amount := req.Amount; amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	accountNumber := strings.TrimSpace(req.AccountNumber)
	accountName := strings.TrimSpace(*req.AccountName)

	if accountNumber == "" {
		return nil, fmt.Errorf("account number is required")
	}

	if accountName == "" {
		return nil, fmt.Errorf("account name is required")
	}

	transferInfo := &Transfer{
		ID:     uuid.NewString(),
		Status: "pending",
	}

	if err := s.repo.CreateTransfer(ctx, transferInfo); err != nil {
		return nil, fmt.Errorf("failed to create transfer record: %w", err)
	}

	return s.providusService.InitiateTransfer(ctx, req)
}

func (s *Service) AddBeneficiary(ctx context.Context, mobileUserID string, req *AddBeneficiaryRequest) (*Beneficiary, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	walletID := strings.TrimSpace(req.WalletID)
	accountNumber := strings.TrimSpace(req.AccountNumber)
	accountName := strings.TrimSpace(req.AccountName)
	bankCode := strings.TrimSpace(req.BankCode)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if walletID == "" {
		return nil, errors.New("wallet ID is required")
	}

	if accountNumber == "" {
		return nil, errors.New("account number is required")
	}

	if accountName == "" {
		return nil, errors.New("account name is required")
	}

	if bankCode == "" {
		return nil, errors.New("bank code is required")
	}

	beneficiary := &Beneficiary{
		ID:            uuid.NewString(),
		MobileUserID:  mobileUserID,
		WalletID:      walletID,
		BankCode:      bankCode,
		AccountNumber: accountNumber,
		AccountName:   accountName,
	}

	if err := s.repo.CreateBeneficiary(ctx, beneficiary); err != nil {
		return nil, err
	}

	return beneficiary, nil
}

func (s *Service) GetBeneficiaries(ctx context.Context, mobileUserID, walletID string) ([]Beneficiary, error) {
	mobileUserID = strings.TrimSpace(mobileUserID)
	walletID = strings.TrimSpace(walletID)

	if mobileUserID == "" {
		return nil, errors.New("mobile user ID is required")
	}

	if walletID == "" {
		return nil, errors.New("wallet ID is required")
	}

	return s.repo.GetBeneficiaries(ctx, mobileUserID, walletID)
}
