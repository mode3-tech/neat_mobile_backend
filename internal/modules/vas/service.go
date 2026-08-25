package vas

import (
	"context"
	"errors"
	"fmt"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/referrals"
	"neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/providers/vas"
	vasprovider "neat_mobile_app_backend/providers/vas"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	Repo           *Repository
	WalletService  WalletService
	Txr            TransactionService
	Baas           BAAS
	XpressPayments VASService
	PinVerifier    AuthService
	User           UserFinder
}

func NewService(repo *Repository, xpressPayments VASService, walletService WalletService, txr TransactionService, baas BAAS, pinVerifier AuthService, userRepo UserFinder) *Service {
	return &Service{
		Repo:           repo,
		XpressPayments: xpressPayments,
		WalletService:  walletService,
		Txr:            txr,
		Baas:           baas,
		PinVerifier:    pinVerifier,
		User:           userRepo,
	}
}

func (s *Service) FetchAllCategories(ctx context.Context) ([]vas.Category, error) {
	xPayCats, err := s.XpressPayments.FetchAllCategories(ctx)
	if err != nil {
		return nil, appErr.ErrFetchingAllCategories
	}

	cats := make([]vas.Category, 0, len(xPayCats.Data.CategoryDTOList))

	for _, cat := range xPayCats.Data.CategoryDTOList {
		cats = append(cats, cat)
	}

	return cats, nil
}

const (
	defaultBillingsPageSize = 10
	maxBillingsPageSize     = 50
)

func normalizeBillingsPagination(page, size int) (requestedPage, providerPage, pageSize int) {
	if size <= 0 {
		size = defaultBillingsPageSize
	} else if size > maxBillingsPageSize {
		size = maxBillingsPageSize
	}

	providerPage = page - 1
	if providerPage < 0 {
		providerPage = 0
	}

	return page, providerPage, size
}

func calculateTotalPages(totalCount, size int) int {
	if totalCount <= 0 {
		return 0
	}
	return (totalCount + size - 1) / size
}

func (s *Service) FetchBillingsByCategoryID(ctx context.Context, payload BillingsByCategoryIDPayload, size, page int) (*BillingsByCategoryIDResponse, int, int, int, int, bool, bool, error) {
	requestedPage, providerPage, pageSize := normalizeBillingsPagination(page, size)

	result, err := s.XpressPayments.FetchBillersByCategoryID(ctx, payload.CategoryID, providerPage, pageSize)
	if err != nil {
		log.Printf("vas service: failed to fetch billers by category - %s\n", err)
		return nil, 0, 0, 0, 0, false, false, err
	}

	totalCount := result.Data.TotalCount
	totalPages := calculateTotalPages(totalCount, pageSize)

	if totalPages > 0 && requestedPage > totalPages {
		return nil, 0, 0, 0, 0, false, false, fmt.Errorf("page %d out of range, total pages: %d", requestedPage, totalPages)
	}

	hasPrev := requestedPage > 1
	hasNext := requestedPage < totalPages

	billers := make([]Biller, 0, len(result.Data.BillerDTOList))
	for _, b := range result.Data.BillerDTOList {
		categories := make([]BillerCategory, 0, len(b.CategoryDTOs))
		for _, c := range b.CategoryDTOs {
			categories = append(categories, BillerCategory{ID: c.ID, Name: c.Name})
		}
		billers = append(billers, Biller{
			ID:           b.ID,
			Name:         b.Name,
			BillerCode:   b.BillerCode,
			Description:  b.Description,
			CategoryDTOs: categories,
			Image:        b.Image,
		})
	}

	response := BillingsByCategoryIDResponse(billers)
	return &response, requestedPage, pageSize, totalCount, totalPages, hasNext, hasPrev, nil
}

func (s *Service) FetchProductsByCategoryIDAndBillerID(ctx context.Context, payload FetchProductsByCategoryIDAndBillerIDPayload, size, page int) (*ProductsResponse, int, int, int, int, bool, bool, error) {
	requestPage, providerPage, pageSize := normalizeBillingsPagination(page, size)

	result, err := s.XpressPayments.FetchProductsByCategoryIDAndBillerID(ctx, payload.CategoryID, payload.BillerID, providerPage, pageSize)
	if err != nil {
		log.Printf("vas service: failed to fetch products by category and biller - %s\n", err)
		return nil, 0, 0, 0, 0, false, false, err
	}

	totalCount := result.Data.TotalCount
	totalPages := calculateTotalPages(totalCount, pageSize)

	if totalPages > 0 && requestPage > totalPages {
		return nil, 0, 0, 0, 0, false, false, fmt.Errorf("page %d out of range, total pages: %d", requestPage, totalPages)
	}

	hasPrev := requestPage > 1
	hasNext := requestPage < totalPages

	products := make([]Product, 0, len(result.Data.ProductDTOList))
	for _, p := range result.Data.ProductDTOList {
		products = append(products, Product{
			Name:        p.Name,
			UniqueCode:  p.UniqueCode,
			LookUp:      p.LookUp,
			FixedAmount: p.FixedAmount,
			Amount:      p.Amount,
			MinAmount:   p.MinAmount,
			MaxAmount:   p.MaxAmount,
			ImageURL:    p.ImageURL,
			BillerName:  p.BillerName,
			CategoryDTO: p.CategoryDTO,
		})
	}

	response := ProductsResponse(products)
	return &response, requestPage, pageSize, totalCount, totalPages, hasNext, hasPrev, nil
}

func (s *Service) GetAirtime(ctx context.Context, payload AirtimePayload, mobileUserID string) (*vasprovider.ISPResponse, error) {
	if payload.UseCashback {
		return s.getAirtimeWithCashback(ctx, payload, mobileUserID)
	}
	requestID := uuid.NewString()
	log.Printf("vas service: request ID: %s\n", requestID)
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	// categoryID := strings.TrimSpace(payload.CategoryID)
	// billerID := strings.TrimSpace(payload.BillerID)

	localizedPhone, err := phone.ToLocalFormat(strings.TrimSpace(payload.PhoneNumber))
	if err != nil {
		log.Printf("vas service: failed to normalize phone number - %s\n", err)
		return nil, err
	}
	amount := payload.Amount

	if amount < 100.00 {
		log.Println("vas service: amount is less than NGN 100")
		return nil, appErr.ErrInvalidISPAmount
	}

	if amount > 10000.00 {
		log.Println("vas service: amount is greater than NGN 10,000")
		return nil, appErr.ErrInvalidISPAmount
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrGettingAirtime
	}

	hasSufficientBalance, err := s.hasSufficientBalance(ctx, wallet.WalletCustomerID, float64(amount))
	log.Printf("vas service: wallet customer id: %s\n", wallet.WalletCustomerID)
	if err != nil {
		log.Printf("vas service: failed to check wallet balance - %s\n", err)
		return nil, appErr.ErrGettingAirtime
	}
	if !hasSufficientBalance {
		log.Println("vas service: insufficient balance")
		return nil, appErr.ErrInsufficientBalance
	}

	// Check provider wallet balance before proceeding
	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		log.Printf("vas service: failed to check provider balance - %s\n", err)
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		log.Printf("vas service: provider balance API error: code=%s msg=%q", providerBal.ResponseCode, providerBal.ResponseMessage)
		return nil, &appErr.XpressPayProviderError{
			Code:    providerBal.ResponseCode,
			Message: providerBal.ResponseMessage,
		}
	}
	if providerBal.Data < float64(amount) {
		log.Printf("vas service: provider wallet balance %.2f insufficient for amount %d", providerBal.Data, amount)
		return nil, appErr.ErrProviderServiceUnavailable
	}

	metadata := map[string]any{
		"isp":  ExtractBillingCompanyName(uniqueCode),
		"type": "airtime",
	}

	log.Printf("extracted company name: %s\n", ExtractBillingCompanyName(uniqueCode))

	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		log.Printf("vas service: failed to verify transaction pin - %s\n", err)
		return nil, err
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryAirtime,
		Description:         "Airtime",
		Amount:              amount * 100,
		BalanceBefore:       wallet.AvailableBalance,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: localizedPhone,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrGettingAirtime
	}

	result, err := s.XpressPayments.GetAirtime(ctx, requestID, uniqueCode, localizedPhone, amount)
	log.Printf("vas service: local phone: %s, amount: %d\n", localizedPhone, amount)
	if err != nil {
		log.Printf("vas service: unable to purchase airtime - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err, requestID)
		return nil, appErr.ErrGettingAirtime
	}

	switch result.ResponseCode {
	case "01":
		log.Printf("vas service: airtime purchase pending - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, appErr.ErrVASAmbiguous, requestID)
		return nil, appErr.ErrGettingAirtime
	case "00":
	default:
		log.Printf("vas service: airtime purchase failed - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID,
			&appErr.XpressPayProviderError{Code: result.ResponseCode, Message: result.ResponseMessage}, requestID)
		return nil, appErr.ErrGettingAirtime
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrGettingAirtime
	}

	go func() {
		bgCtx := context.Background()
		ctx, cancel := context.WithTimeout(bgCtx, time.Second*5)
		defer cancel()
		beneficiary := VASBeneficiary{
			ID:             uuid.NewString(),
			MobileUserID:   mobileUserID,
			PhoneNumber:    localizedPhone,
			BillingCompany: strings.ToLower(ExtractBillingCompanyName(strings.TrimSpace(payload.UniqueCode))),
		}

		if err := s.Repo.StoreVASAsBeneficiary(ctx, &beneficiary); err != nil {
			log.Printf("vas service: failed to store vas beneficiary - %s", err)
		}
	}()

	return result, nil
}

func (s *Service) GetData(ctx context.Context, payload DataPayload, mobileUserID string) (*vasprovider.ISPResponse, error) {
	if payload.UseCashback {
		return s.getDataWithCashback(ctx, payload, mobileUserID)
	}
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	localizedPhone, err := phone.ToLocalFormat(strings.TrimSpace(payload.PhoneNumber))
	if err != nil {
		log.Printf("vas service: failed to normalize phone number - %s\n", err)
		return nil, err
	}
	amount := payload.Amount

	if amount < 100 {
		return nil, appErr.ErrInvalidISPAmount
	}

	if amount > 10000 {
		return nil, appErr.ErrInvalidISPAmount
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrGettingData
	}

	hasSufficientBalance, err := s.hasSufficientBalance(ctx, wallet.WalletCustomerID, float64(amount))
	if err != nil {
		log.Printf("vas service: failed to check sufficient balance - %s\n", err)
		return nil, appErr.ErrGettingData
	}
	if !hasSufficientBalance {
		log.Println("vas service: insufficient balance")
		return nil, appErr.ErrInsufficientBalance
	}

	// Check provider wallet balance before proceeding
	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		log.Printf("vas service: failed to check provider balance - %s\n", err)
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		log.Printf("vas service: provider balance API error: code=%s msg=%q", providerBal.ResponseCode, providerBal.ResponseMessage)
		return nil, &appErr.XpressPayProviderError{
			Code:    providerBal.ResponseCode,
			Message: providerBal.ResponseMessage,
		}
	}
	if providerBal.Data < float64(amount) {
		log.Printf("vas service: provider wallet balance %.2f insufficient for amount %d", providerBal.Data, amount)
		return nil, appErr.ErrProviderServiceUnavailable
	}

	metadata := map[string]any{
		"isp":  ExtractBillingCompanyName(uniqueCode),
		"type": "data",
	}

	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryMobileData,
		Amount:              amount * 100,
		BalanceBefore:       wallet.AvailableBalance,
		Description:         "Mobile Data",
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: localizedPhone,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	log.Printf("vas service: customer id %s, amount %d\n", wallet.WalletCustomerID, amount)
	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrGettingData
	}

	result, err := s.XpressPayments.GetData(ctx, requestID, uniqueCode, localizedPhone, amount)
	if err != nil {
		log.Printf("vas service: unable to purchase data - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err, requestID)
		return nil, appErr.ErrGettingData
	}

	switch result.ResponseCode {
	case "01":
		log.Printf("vas service: data purchase pending - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, appErr.ErrVASAmbiguous, requestID)
		return nil, appErr.ErrGettingData
	case "00":
	default:
		log.Printf("vas service: data purchase failed - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID,
			&appErr.XpressPayProviderError{Code: result.ResponseCode, Message: result.ResponseMessage}, requestID)
		return nil, appErr.ErrGettingData
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrGettingData
	}

	go func() {
		bgCtx := context.Background()
		ctx, cancel := context.WithTimeout(bgCtx, time.Second*5)
		defer cancel()
		beneficiary := VASBeneficiary{
			ID:             uuid.NewString(),
			MobileUserID:   mobileUserID,
			PhoneNumber:    localizedPhone,
			BillingCompany: strings.ToLower(ExtractBillingCompanyName(strings.TrimSpace(payload.UniqueCode))),
		}

		if err := s.Repo.StoreVASAsBeneficiary(ctx, &beneficiary); err != nil {
			log.Printf("vas service: failed to store vas beneficiary - %s", err)
		}
	}()

	return result, nil
}

func (s *Service) validateElectricity(ctx context.Context, payload ElectricityValidationPayload) (*vasprovider.ElectricityValidationResponse, error) {
	result, err := s.XpressPayments.ValidateElectricity(
		ctx,
		uuid.NewString(),
		strings.TrimSpace(payload.UniqueCode),
		strings.TrimSpace(payload.AccountNumber),
		vasprovider.AccountType(payload.AccountType),
	)
	if err != nil {
		log.Printf("vas service: failed to validate electricity account - %s\n", err)
		return nil, err
	}
	return result, nil
}

func (s *Service) PayElectricity(ctx context.Context, payload PayElectricityPayload, mobileUserID string) (*vasprovider.PayElectricityResponse, error) {
	if payload.UseCashback {
		return s.payElectricityWithCashback(ctx, payload, mobileUserID)
	}
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	accountNumber := strings.TrimSpace(payload.AccountNumber)
	amount := payload.Amount

	user, err := s.User.GetUserByUserID(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get user - %s\n", err)
		return nil, appErr.ErrPayingElectricityBill
	}
	if user == nil {
		return nil, appErr.ErrPayingElectricityBill
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrPayingElectricityBill
	}

	hasSufficientBalance, err := s.hasSufficientBalance(ctx, wallet.WalletCustomerID, float64(amount))
	if err != nil {
		log.Printf("vas service: failed to check sufficient balance - %s\n", err)
		return nil, appErr.ErrPayingElectricityBill
	}
	if !hasSufficientBalance {
		return nil, appErr.ErrInsufficientBalance
	}

	// Check provider wallet balance before proceeding
	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		log.Printf("vas service: failed to check provider balance - %s\n", err)
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		log.Printf("vas service: provider balance API error: code=%s msg=%q", providerBal.ResponseCode, providerBal.ResponseMessage)
		return nil, &appErr.XpressPayProviderError{
			Code:    providerBal.ResponseCode,
			Message: providerBal.ResponseMessage,
		}
	}
	if providerBal.Data < float64(amount) {
		log.Printf("vas service: provider wallet balance %.2f insufficient for amount %d", providerBal.Data, amount)
		return nil, appErr.ErrProviderServiceUnavailable
	}

	extractedBillingCompany := ExtractBillingCompanyName(uniqueCode)

	metadata := map[string]any{
		"provider": extractedBillingCompany,
		"type":     "electricity",
	}

	validateElectricityPayload := ElectricityValidationPayload{
		UniqueCode:    uniqueCode,
		AccountNumber: accountNumber,
		AccountType:   payload.AccountType,
	}

	validationResult, err := s.validateElectricity(ctx, validateElectricityPayload)
	if err != nil {
		log.Printf("vas service: failed to validate electricity account - %s\n", err)
		return nil, err
	}

	if validationResult != nil {
		if &validationResult.Data == nil {
			return nil, appErr.ErrValidatingElectricity
		}
		if validationResult.Data.AccountNumber != accountNumber {
			return nil, appErr.ErrInvalidAccountNumber
		}

		if string(validationResult.Data.AccountType) != string(payload.AccountType) {
			return nil, appErr.ErrInvalidAccountType
		}

		if validationResult.ResponseCode != "00" && validationResult.ResponseCode != "01" {
			log.Printf("vas service: failed to validate electricity account - %s\n", validationResult.ResponseMessage)
			return nil, &appErr.XpressPayProviderError{
				Code:    validationResult.ResponseCode,
				Message: validationResult.ResponseMessage,
			}
		}
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryElectricity,
		Amount:              amount * 100,
		Description:         fmt.Sprintf("Electricity"),
		BalanceBefore:       wallet.AvailableBalance,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: accountNumber,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrPayingElectricityBill
	}

	address := ""
	if user.Address != nil {
		address = *user.Address
	}

	result, err := s.XpressPayments.PayElectricityBill(
		ctx, requestID, uniqueCode, accountNumber,
		user.FullName, address, user.Phone,
		vasprovider.AccountType(payload.AccountType), amount,
	)
	if err != nil {
		log.Printf("vas service: failed to pay electricity bill - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err, requestID)
		return nil, appErr.ErrPayingElectricityBill
	}

	switch result.ResponseCode {
	case "01":
		log.Printf("vas service: electricity payment pending - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, appErr.ErrVASAmbiguous, requestID)
		return nil, appErr.ErrPayingElectricityBill
	case "00":
	default:
		log.Printf("vas service: electricity payment failed - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID,
			&appErr.XpressPayProviderError{Code: result.ResponseCode, Message: result.ResponseMessage}, requestID)
		return nil, appErr.ErrPayingElectricityBill
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrPayingElectricityBill
	}

	if result.Data.Token != "" {
		tokenMetadata := map[string]any{
			"provider": ExtractBillingCompanyName(uniqueCode),
			"type":     "electricity",
			"token":    result.Data.Token,
			"units":    result.Data.Unit,
		}
		if updateErr := s.Repo.UpdateTransactionMetadata(ctx, txID, tokenMetadata); updateErr != nil {
			log.Printf("vas service: failed to store electricity token in metadata - %s\n", updateErr)
		}
	}

	return result, nil
}

func (s *Service) ValidateCable(ctx context.Context, payload ValidateCablePayload) (*vasprovider.CableValidationResponse, error) {
	result, err := s.XpressPayments.ValidateCable(
		ctx,
		uuid.NewString(),
		strings.TrimSpace(payload.UniqueCode),
		strings.TrimSpace(payload.AccountNumber),
		payload.NoOfMonth,
	)
	if err != nil {
		log.Printf("vas service: failed to validate cable account - %s\n", err)
		return nil, err
	}
	return result, nil
}

func (s *Service) PayCable(ctx context.Context, payload PayCablePayload, mobileUserID string) (*vasprovider.PayCableResponse, error) {
	if payload.UseCashback {
		return s.payCableWithCashback(ctx, payload, mobileUserID)
	}
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	accountNumber := strings.TrimSpace(payload.AccountNumber)
	amount := payload.Amount

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get wallet balance - %s\n", err)
		return nil, appErr.ErrPayingCableBill
	}

	user, err := s.User.GetUserByUserID(ctx, mobileUserID)
	if err != nil {
		log.Printf("vas service: failed to get user - %s\n", err)
		return nil, appErr.ErrPayingCableBill
	}
	if user == nil {
		return nil, appErr.ErrPayingCableBill
	}

	hasSufficientBalance, err := s.hasSufficientBalance(ctx, wallet.WalletCustomerID, float64(amount))
	if err != nil {
		log.Printf("vas service: failed to check sufficient balance - %s\n", err)
		return nil, appErr.ErrPayingCableBill
	}
	if !hasSufficientBalance {
		return nil, appErr.ErrInsufficientBalance
	}

	// Check provider wallet balance before proceeding
	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		log.Printf("vas service: failed to check provider balance - %s\n", err)
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		log.Printf("vas service: provider balance API error: code=%s msg=%q", providerBal.ResponseCode, providerBal.ResponseMessage)
		return nil, &appErr.XpressPayProviderError{
			Code:    providerBal.ResponseCode,
			Message: providerBal.ResponseMessage,
		}
	}
	if providerBal.Data < float64(amount) {
		log.Printf("vas service: provider wallet balance %.2f insufficient for amount %d", providerBal.Data, amount)
		return nil, appErr.ErrProviderServiceUnavailable
	}

	extractedBillingCompany := ExtractBillingCompanyName(uniqueCode)

	metadata := map[string]any{
		"provider": extractedBillingCompany,
		"type":     "cable",
	}

	validateCablePayload := ValidateCablePayload{
		UniqueCode:    uniqueCode,
		AccountNumber: accountNumber,
		NoOfMonth:     payload.NoOfMonth,
	}

	validateResult, err := s.ValidateCable(ctx, validateCablePayload)
	if err != nil {
		log.Printf("vas service: failed to validate cable account - %s\n", err)
		return nil, err
	}
	if validateResult.Data.AccountNumber != accountNumber {
		return nil, appErr.ErrInvalidAccountNumber
	}

	if validateResult.ResponseCode != "00" && validateResult.ResponseCode != "01" {
		log.Printf("vas service: failed to validate cable account - %s\n", validateResult.ResponseMessage)
		return nil, &appErr.XpressPayProviderError{
			Code:    validateResult.ResponseCode,
			Message: validateResult.ResponseMessage,
		}
	}

	txID, ref := uuid.NewString(), uuid.NewString()

	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryTV,
		Amount:              amount * 100,
		Description:         fmt.Sprintf("TV"),
		BalanceBefore:       wallet.AvailableBalance,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: accountNumber,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		CreatedAt:           time.Now().UTC(),
	}

	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		log.Printf("vas service: failed to add transaction record at pending state - %s\n", err)
		return nil, err
	}

	debitResult, err := s.Baas.DebitCustomer(ctx, amount, wallet.WalletCustomerID, ref, metadata)
	if err != nil {
		log.Printf("vas service: failed to debit customer wallet - %s\n", err)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); updateErr != nil {
			log.Printf("vas service: failed to update transaction to failed after debit error - %s\n", updateErr)
		}
		return nil, appErr.ErrPayingCableBill
	}

	normalizedPhone, err := phone.ToLocalFormat(user.Phone)
	if err != nil {
		log.Printf("vas service: failed to normalize phone number - %s\n", err)
		return nil, appErr.ErrPayingCableBill
	}

	result, err := s.XpressPayments.PayCableBill(
		ctx, requestID, uniqueCode, accountNumber,
		payload.AccountType, user.FullName, normalizedPhone,
		payload.NoOfMonth, amount,
	)
	if err != nil {
		log.Printf("vas service: failed to pay cable bill - %s\n", err)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, err, requestID)
		return nil, appErr.ErrPayingCableBill
	}

	switch result.ResponseCode {
	case "01":
		log.Printf("vas service: cable payment pending - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID, appErr.ErrVASAmbiguous, requestID)
		return nil, appErr.ErrPayingCableBill
	case "00":
	default:
		log.Printf("vas service: cable payment failed - %s\n", result.ResponseMessage)
		s.handleFulfilFailure(ctx, txID, amount, debitResult.Data.TransactionFee, wallet.AvailableBalance, metadata, wallet.WalletCustomerID,
			&appErr.XpressPayProviderError{Code: result.ResponseCode, Message: result.ResponseMessage}, requestID)
		return nil, appErr.ErrPayingCableBill
	}

	balanceAfter := wallet.AvailableBalance - ((amount + int64(debitResult.Data.TransactionFee)) * 100)
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
		log.Printf("vas service: failed to update transaction record to successful - %s", err)
		return nil, appErr.ErrPayingCableBill
	}

	return result, nil
}

// handleFulfilFailure handles the post-debit failure path for all fulfil operations.
// For ambiguous outcomes (timeout/5xx), it queries CheckStatus to resolve:
//
//	"00" → update to successful (no reversal needed)
//	"01" → reversal_pending (manual reconciliation)
//	other → fall through to auto-reverse
//
// If CheckStatus itself fails, defaults to reversal_pending.
// For deterministic errors → credits the customer back and marks reversed.
func (s *Service) handleFulfilFailure(ctx context.Context, txID string, amount int64, txFee int, balanceBefore int64, metadata map[string]any, customerID string, vasErr error, requestID string) {
	if errors.Is(vasErr, appErr.ErrVASAmbiguous) {
		// Try to resolve ambiguity by checking status with the provider
		status, checkErr := s.XpressPayments.CheckStatus(ctx, requestID)
		if checkErr == nil {
			switch status.ResponseCode {
			case "00":
				// Transaction actually succeeded — update to successful
				balanceAfter := balanceBefore - ((amount + int64(txFee)) * 100)
				if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); updateErr != nil {
					log.Printf("vas service: failed to mark transaction as successful after status check - %s\n", updateErr)
				}
				return
			case "01":
				// Still pending — mark for manual reconciliation
				debitedBalance := balanceBefore - ((amount + int64(txFee)) * 100)
				if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, debitedBalance, TransactionStatusReversalPending); updateErr != nil {
					log.Printf("vas service: failed to mark transaction as reversal_pending - %s\n", updateErr)
				}
				return
			default:
				// Failed — fall through to auto-reverse
			}
		}

		// Status check failed or returned unknown code — default to reversal_pending
		debitedBalance := balanceBefore - ((amount + int64(txFee)) * 100)
		if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, debitedBalance, TransactionStatusReversalPending); updateErr != nil {
			log.Printf("vas service: failed to mark transaction as reversal_pending - %s\n", updateErr)
		}
		return
	}

	// Deterministic failure — auto-reverse
	reversalRef := uuid.NewString()
	if _, creditErr := s.Baas.CreditCustomer(ctx, amount, reversalRef, customerID, metadata); creditErr != nil {
		log.Printf("vas service: failed to credit customer back after VAS failure - %s\n", creditErr)
	}
	if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, balanceBefore, TransactionStatusReversed); updateErr != nil {
		log.Printf("vas service: failed to mark transaction as reversed - %s\n", updateErr)
	}
}

func (s *Service) FetchBeneficiaries(ctx context.Context, mobileUserID, biller string) ([]VAS, error) {
	return s.Repo.FetchVASBeneficiaries(ctx, mobileUserID, biller)
}

func (s *Service) CheckStatus(ctx context.Context, requestID string) (*vasprovider.CheckStatusResponse, error) {
	result, err := s.XpressPayments.CheckStatus(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) hasSufficientBalance(ctx context.Context, customerID string, amount float64) (bool, error) {
	customerWallet, err := s.Baas.GetCustomerWallet(ctx, customerID)
	if err != nil {
		return false, err
	}
	if customerWallet.Wallet.AvailableBalance < amount {
		log.Println("vas service: insufficient balance")
		return false, appErr.ErrInsufficientBalance
	}
	return true, nil
}

func (s *Service) checkCashbackBalance(ctx context.Context, mobileUserID string, amountKobo int64) (int64, error) {
	balance, err := s.Repo.GetLatestCashbackBalance(ctx, mobileUserID)
	if err != nil {
		return 0, err
	}
	if balance < amountKobo {
		return 0, appErr.ErrInsufficientCashback
	}
	return balance, nil
}

func (s *Service) handleFulfilFailureCashback(ctx context.Context, txID, requestID string, cashbackBefore, amountKobo int64, mobileUserID string) {
	status, checkErr := s.XpressPayments.CheckStatus(ctx, requestID)
	if checkErr == nil {
		switch status.ResponseCode {
		case "00":
			if _, spendErr := s.Repo.CompleteCashbackSpend(ctx, txID, mobileUserID, amountKobo, referrals.CashbackSourceVAS); spendErr != nil {
				log.Printf("vas service: cashback spend failed after ambiguous success txID=%s: %v", txID, spendErr)
			}
			return
		case "01":
			if err := s.Txr.UpdateTransactionStatus(ctx, txID, cashbackBefore, TransactionStatusReversalPending); err != nil {
				log.Printf("vas service: failed to mark cashback txn reversal_pending txID=%s: %v", txID, err)
			}
			return
		}
	}
	if err := s.Txr.UpdateTransactionStatus(ctx, txID, cashbackBefore, TransactionStatusReversed); err != nil {
		log.Printf("vas service: failed to mark cashback txn reversed txID=%s: %v", txID, err)
	}
}

func (s *Service) getAirtimeWithCashback(ctx context.Context, payload AirtimePayload, mobileUserID string) (*vasprovider.ISPResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)

	localizedPhone, err := phone.ToLocalFormat(strings.TrimSpace(payload.PhoneNumber))
	if err != nil {
		return nil, err
	}
	amount := payload.Amount
	if amount < 100 || amount > 10000 {
		return nil, appErr.ErrInvalidISPAmount
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrGettingAirtime
	}

	cashbackBefore, err := s.checkCashbackBalance(ctx, mobileUserID, amount*100)
	if err != nil {
		return nil, err
	}

	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		return nil, &appErr.XpressPayProviderError{Code: providerBal.ResponseCode, Message: providerBal.ResponseMessage}
	}
	if providerBal.Data < float64(amount) {
		return nil, appErr.ErrProviderServiceUnavailable
	}

	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}

	txID, ref := uuid.NewString(), uuid.NewString()
	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryAirtime,
		Description:         "Airtime",
		Amount:              amount * 100,
		BalanceBefore:       cashbackBefore,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: localizedPhone,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		UsedCashback:        true,
		CreatedAt:           time.Now().UTC(),
	}
	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		return nil, err
	}

	result, err := s.XpressPayments.GetAirtime(ctx, requestID, uniqueCode, localizedPhone, amount)
	if err != nil {
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrGettingAirtime
	}

	switch result.ResponseCode {
	case "01":
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrGettingAirtime
	case "00":
	default:
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrGettingAirtime
	}

	if _, err := s.Repo.CompleteCashbackSpend(ctx, txID, mobileUserID, amount*100, referrals.CashbackSourceVAS); err != nil {
		log.Printf("vas service: cashback spend failed on success txID=%s: %v", txID, err)
		return nil, appErr.ErrGettingAirtime
	}

	return result, nil
}

func (s *Service) getDataWithCashback(ctx context.Context, payload DataPayload, mobileUserID string) (*vasprovider.ISPResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	localizedPhone, err := phone.ToLocalFormat(strings.TrimSpace(payload.PhoneNumber))
	if err != nil {
		return nil, err
	}
	amount := payload.Amount
	if amount < 100 || amount > 10000 {
		return nil, appErr.ErrInvalidISPAmount
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrGettingData
	}

	cashbackBefore, err := s.checkCashbackBalance(ctx, mobileUserID, amount*100)
	if err != nil {
		return nil, err
	}

	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		return nil, &appErr.XpressPayProviderError{Code: providerBal.ResponseCode, Message: providerBal.ResponseMessage}
	}
	if providerBal.Data < float64(amount) {
		return nil, appErr.ErrProviderServiceUnavailable
	}

	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}

	txID, ref := uuid.NewString(), uuid.NewString()
	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryMobileData,
		Amount:              amount * 100,
		BalanceBefore:       cashbackBefore,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: localizedPhone,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		UsedCashback:        true,
		CreatedAt:           time.Now().UTC(),
	}
	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		return nil, err
	}

	result, err := s.XpressPayments.GetData(ctx, requestID, uniqueCode, localizedPhone, amount)
	if err != nil {
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrGettingData
	}

	switch result.ResponseCode {
	case "01":
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrGettingData
	case "00":
	default:
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrGettingData
	}

	if _, err := s.Repo.CompleteCashbackSpend(ctx, txID, mobileUserID, amount*100, referrals.CashbackSourceVAS); err != nil {
		log.Printf("vas service: cashback spend failed on success txID=%s: %v", txID, err)
		return nil, appErr.ErrGettingData
	}

	return result, nil
}

func (s *Service) payElectricityWithCashback(ctx context.Context, payload PayElectricityPayload, mobileUserID string) (*vasprovider.PayElectricityResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	accountNumber := strings.TrimSpace(payload.AccountNumber)
	amount := payload.Amount

	user, err := s.User.GetUserByUserID(ctx, mobileUserID)
	if err != nil || user == nil {
		return nil, appErr.ErrPayingElectricityBill
	}

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrPayingElectricityBill
	}

	cashbackBefore, err := s.checkCashbackBalance(ctx, mobileUserID, amount*100)
	if err != nil {
		return nil, err
	}

	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		return nil, &appErr.XpressPayProviderError{Code: providerBal.ResponseCode, Message: providerBal.ResponseMessage}
	}
	if providerBal.Data < float64(amount) {
		return nil, appErr.ErrProviderServiceUnavailable
	}

	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}

	validationResult, err := s.validateElectricity(ctx, ElectricityValidationPayload{
		UniqueCode: uniqueCode, AccountNumber: accountNumber, AccountType: payload.AccountType,
	})
	if err != nil {
		return nil, err
	}
	if validationResult != nil {
		if validationResult.Data.AccountNumber != accountNumber {
			return nil, appErr.ErrInvalidAccountNumber
		}
		if string(validationResult.Data.AccountType) != string(payload.AccountType) {
			return nil, appErr.ErrInvalidAccountType
		}
		if validationResult.ResponseCode != "00" && validationResult.ResponseCode != "01" {
			return nil, &appErr.XpressPayProviderError{Code: validationResult.ResponseCode, Message: validationResult.ResponseMessage}
		}
	}

	txID, ref := uuid.NewString(), uuid.NewString()
	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryElectricity,
		Amount:              amount * 100,
		Description:         "Electricity",
		BalanceBefore:       cashbackBefore,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: accountNumber,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		UsedCashback:        true,
		CreatedAt:           time.Now().UTC(),
	}
	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		return nil, err
	}

	address := ""
	if user.Address != nil {
		address = *user.Address
	}
	result, err := s.XpressPayments.PayElectricityBill(
		ctx, requestID, uniqueCode, accountNumber,
		user.FullName, address, user.Phone,
		vasprovider.AccountType(payload.AccountType), amount,
	)
	if err != nil {
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrPayingElectricityBill
	}

	switch result.ResponseCode {
	case "01":
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrPayingElectricityBill
	case "00":
	default:
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrPayingElectricityBill
	}

	if _, spendErr := s.Repo.CompleteCashbackSpend(ctx, txID, mobileUserID, amount*100, referrals.CashbackSourceVAS); spendErr != nil {
		log.Printf("vas service: cashback spend failed on electricity success txID=%s: %v", txID, spendErr)
		return nil, appErr.ErrPayingElectricityBill
	}

	if result.Data.Token != "" {
		tokenMetadata := map[string]any{
			"provider": ExtractBillingCompanyName(uniqueCode),
			"type":     "electricity",
			"token":    result.Data.Token,
			"units":    result.Data.Unit,
		}
		if updateErr := s.Repo.UpdateTransactionMetadata(ctx, txID, tokenMetadata); updateErr != nil {
			log.Printf("vas service: failed to store electricity token in metadata - %s", updateErr)
		}
	}

	return result, nil
}

func (s *Service) payCableWithCashback(ctx context.Context, payload PayCablePayload, mobileUserID string) (*vasprovider.PayCableResponse, error) {
	requestID := uuid.NewString()
	uniqueCode := strings.TrimSpace(payload.UniqueCode)
	accountNumber := strings.TrimSpace(payload.AccountNumber)
	amount := payload.Amount

	wallet, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		return nil, appErr.ErrPayingCableBill
	}

	user, err := s.User.GetUserByUserID(ctx, mobileUserID)
	if err != nil || user == nil {
		return nil, appErr.ErrPayingCableBill
	}

	cashbackBefore, err := s.checkCashbackBalance(ctx, mobileUserID, amount*100)
	if err != nil {
		return nil, err
	}

	providerBal, err := s.XpressPayments.GetWalletBalance(ctx)
	if err != nil {
		return nil, appErr.ErrProviderServiceUnavailable
	}
	if providerBal.ResponseCode != "00" && providerBal.ResponseCode != "0" {
		return nil, &appErr.XpressPayProviderError{Code: providerBal.ResponseCode, Message: providerBal.ResponseMessage}
	}
	if providerBal.Data < float64(amount) {
		return nil, appErr.ErrProviderServiceUnavailable
	}

	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}

	validateResult, err := s.ValidateCable(ctx, ValidateCablePayload{
		UniqueCode: uniqueCode, AccountNumber: accountNumber, NoOfMonth: payload.NoOfMonth,
	})
	if err != nil {
		return nil, err
	}
	if validateResult.Data.AccountNumber != accountNumber {
		return nil, appErr.ErrInvalidAccountNumber
	}
	if validateResult.ResponseCode != "00" && validateResult.ResponseCode != "01" {
		return nil, &appErr.XpressPayProviderError{Code: validateResult.ResponseCode, Message: validateResult.ResponseMessage}
	}

	txID, ref := uuid.NewString(), uuid.NewString()
	txn := Transaction{
		ID:                  txID,
		MobileUserID:        mobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            TransactionCategoryTV,
		Amount:              amount * 100,
		Description:         "TV",
		BalanceBefore:       cashbackBefore,
		BalanceAfter:        0,
		Reference:           ref,
		CounterpartyName:    ExtractBillingCompanyName(uniqueCode),
		CounterpartyAccount: accountNumber,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		UsedCashback:        true,
		CreatedAt:           time.Now().UTC(),
	}
	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		return nil, err
	}

	normalizedPhone, err := phone.ToLocalFormat(user.Phone)
	if err != nil {
		return nil, appErr.ErrPayingCableBill
	}

	result, err := s.XpressPayments.PayCableBill(
		ctx, requestID, uniqueCode, accountNumber,
		payload.AccountType, user.FullName, normalizedPhone,
		payload.NoOfMonth, amount,
	)
	if err != nil {
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrPayingCableBill
	}

	switch result.ResponseCode {
	case "01":
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrPayingCableBill
	case "00":
	default:
		s.handleFulfilFailureCashback(ctx, txID, requestID, cashbackBefore, amount*100, mobileUserID)
		return nil, appErr.ErrPayingCableBill
	}

	if _, spendErr := s.Repo.CompleteCashbackSpend(ctx, txID, mobileUserID, amount*100, referrals.CashbackSourceVAS); spendErr != nil {
		log.Printf("vas service: cashback spend failed on cable success txID=%s: %v", txID, spendErr)
		return nil, appErr.ErrPayingCableBill
	}

	return result, nil
}
