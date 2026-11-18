package vas

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
	walletmodule "neat_mobile_app_backend/internal/modules/wallet"
	"neat_mobile_app_backend/providers/baas"
	vasprovider "neat_mobile_app_backend/providers/vas"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) ConfigureWema(provider WemaVASProvider) {
	s.Wema = provider
}

func (s *Service) isWemaWallet(wallet *CustomerWallet) bool {
	return wallet != nil && walletmodule.IsWemaProvider(wallet.Provider)
}

func (s *Service) FetchAllCategoriesForUser(ctx context.Context, mobileUserID string) ([]vasprovider.Category, error) {
	w, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		return nil, err
	}
	if !walletmodule.IsWemaProvider(w.Provider) {
		return s.FetchAllCategories(ctx)
	}
	if s.Wema == nil {
		return nil, appErr.ErrWemaUnavailable
	}
	bills, err := s.Wema.GetBills(ctx)
	if err != nil {
		return nil, err
	}
	categories := make([]vasprovider.Category, 0, len(bills.Result))
	for _, category := range bills.Result {
		categories = append(categories, vasprovider.Category{ID: category.ID, Name: category.Name})
	}
	return categories, nil
}

func (s *Service) FetchBillingsByCategoryIDForUser(ctx context.Context, mobileUserID string, payload BillingsByCategoryIDPayload, size, page int) (*BillingsByCategoryIDResponse, int, int, int, int, bool, bool, error) {
	w, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		return nil, 0, 0, 0, 0, false, false, err
	}
	if !walletmodule.IsWemaProvider(w.Provider) {
		return s.FetchBillingsByCategoryID(ctx, payload, size, page)
	}
	if s.Wema == nil {
		return nil, 0, 0, 0, 0, false, false, appErr.ErrWemaUnavailable
	}
	bills, err := s.Wema.GetBills(ctx)
	if err != nil {
		return nil, 0, 0, 0, 0, false, false, err
	}
	var selected *baas.AlatBillCategory
	for i := range bills.Result {
		if bills.Result[i].ID == payload.CategoryID {
			selected = &bills.Result[i]
			break
		}
	}
	if selected == nil {
		return nil, 0, 0, 0, 0, false, false, fmt.Errorf("wema vas: category %d not found", payload.CategoryID)
	}
	if strings.Contains(strings.ToLower(selected.Name), "data") {
		plans, err := s.Wema.GetDataPlans(ctx)
		if err != nil {
			return nil, 0, 0, 0, 0, false, false, err
		}
		return paginateWemaDataBillers(*selected, plans.Result, size, page)
	}
	requestedPage, providerPage, pageSize := normalizeBillingsPagination(page, size)
	totalCount := len(selected.Billers)
	totalPages := calculateTotalPages(totalCount, pageSize)
	if totalPages > 0 && requestedPage > totalPages {
		return nil, 0, 0, 0, 0, false, false, fmt.Errorf("page %d out of range, total pages: %d", requestedPage, totalPages)
	}
	start := providerPage * pageSize
	if start > totalCount {
		start = totalCount
	}
	end := start + pageSize
	if end > totalCount {
		end = totalCount
	}
	billers := make(BillingsByCategoryIDResponse, 0, end-start)
	for _, biller := range selected.Billers[start:end] {
		billers = append(billers, Biller{
			ID:           biller.ID,
			Name:         biller.Name,
			BillerCode:   biller.Identifier,
			Charge:       biller.Charge,
			Description:  biller.ShortCode,
			CategoryDTOs: []BillerCategory{{ID: selected.ID, Name: selected.Name}},
		})
	}
	hasPrev := requestedPage > 1
	hasNext := requestedPage < totalPages
	return &billers, requestedPage, pageSize, totalCount, totalPages, hasNext, hasPrev, nil
}

func paginateWemaDataBillers(category baas.AlatBillCategory, plans []baas.AlatDataPlan, size, page int) (*BillingsByCategoryIDResponse, int, int, int, int, bool, bool, error) {
	requestedPage, providerPage, pageSize := normalizeBillingsPagination(page, size)
	totalCount := len(plans)
	totalPages := calculateTotalPages(totalCount, pageSize)
	if totalPages > 0 && requestedPage > totalPages {
		return nil, 0, 0, 0, 0, false, false, fmt.Errorf("page %d out of range, total pages: %d", requestedPage, totalPages)
	}
	start := providerPage * pageSize
	if start > totalCount {
		start = totalCount
	}
	end := start + pageSize
	if end > totalCount {
		end = totalCount
	}
	billers := make(BillingsByCategoryIDResponse, 0, end-start)
	for _, plan := range plans[start:end] {
		billers = append(billers, Biller{
			ID:           plan.ID,
			Name:         plan.NetworkProvider,
			BillerCode:   plan.NetworkProvider,
			CategoryDTOs: []BillerCategory{{ID: category.ID, Name: category.Name}},
		})
	}
	hasPrev := requestedPage > 1
	hasNext := requestedPage < totalPages
	return &billers, requestedPage, pageSize, totalCount, totalPages, hasNext, hasPrev, nil
}

func (s *Service) FetchProductsByCategoryIDAndBillerIDForUser(ctx context.Context, mobileUserID string, payload FetchProductsByCategoryIDAndBillerIDPayload, size, page int) (*ProductsResponse, int, int, int, int, bool, bool, error) {
	w, err := s.WalletService.GetBalance(ctx, mobileUserID)
	if err != nil {
		return nil, 0, 0, 0, 0, false, false, err
	}
	if !walletmodule.IsWemaProvider(w.Provider) {
		return s.FetchProductsByCategoryIDAndBillerID(ctx, payload, size, page)
	}
	if s.Wema == nil {
		return nil, 0, 0, 0, 0, false, false, appErr.ErrWemaUnavailable
	}
	bills, err := s.Wema.GetBills(ctx)
	if err != nil {
		return nil, 0, 0, 0, 0, false, false, err
	}
	for _, category := range bills.Result {
		if category.ID != payload.CategoryID {
			continue
		}
		if strings.Contains(strings.ToLower(category.Name), "data") {
			return s.wemaDataProducts(ctx, category.ID, payload.BillerID, size, page)
		}
		for _, biller := range category.Billers {
			if biller.ID == payload.BillerID {
				return paginateWemaBillProducts(category, biller, size, page)
			}
		}
	}
	return nil, 0, 0, 0, 0, false, false, fmt.Errorf("wema vas: biller %d not found in category %d", payload.BillerID, payload.CategoryID)
}

func paginateWemaBillProducts(category baas.AlatBillCategory, biller baas.AlatBiller, size, page int) (*ProductsResponse, int, int, int, int, bool, bool, error) {
	requestedPage, providerPage, pageSize := normalizeBillingsPagination(page, size)
	totalCount := len(biller.Packages)
	totalPages := calculateTotalPages(totalCount, pageSize)
	if totalPages > 0 && requestedPage > totalPages {
		return nil, 0, 0, 0, 0, false, false, fmt.Errorf("page %d out of range, total pages: %d", requestedPage, totalPages)
	}
	start := providerPage * pageSize
	if start > totalCount {
		start = totalCount
	}
	end := start + pageSize
	if end > totalCount {
		end = totalCount
	}
	products := make(ProductsResponse, 0, end-start)
	for _, product := range biller.Packages[start:end] {
		uniqueCode := strconv.Itoa(product.ID)
		network := ""
		if strings.Contains(strings.ToLower(category.Name), "airtime") {
			uniqueCode = biller.Identifier
			network = biller.Identifier
		}
		products = append(products, Product{
			Name:        product.Name,
			UniqueCode:  uniqueCode,
			Network:     network,
			LookUp:      biller.RequiredValidation,
			FixedAmount: !product.IsAmountEditable,
			Charge:      biller.Charge,
			Amount:      float32(product.Amount),
			MinAmount:   float32(product.MinAmount),
			MaxAmount:   float32(product.MaxAmount),
			BillerName:  biller.Name,
			CategoryDTO: struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{ID: category.ID, Name: category.Name},
		})
	}
	hasPrev := requestedPage > 1
	hasNext := requestedPage < totalPages
	return &products, requestedPage, pageSize, totalCount, totalPages, hasNext, hasPrev, nil
}

func (s *Service) wemaDataProducts(ctx context.Context, categoryID, billerID, size, page int) (*ProductsResponse, int, int, int, int, bool, bool, error) {
	plans, err := s.Wema.GetDataPlans(ctx)
	if err != nil {
		return nil, 0, 0, 0, 0, false, false, err
	}
	for _, plan := range plans.Result {
		if plan.ID != billerID {
			continue
		}
		requestedPage, providerPage, pageSize := normalizeBillingsPagination(page, size)
		totalCount := len(plan.DataPackages)
		totalPages := calculateTotalPages(totalCount, pageSize)
		if totalPages > 0 && requestedPage > totalPages {
			return nil, 0, 0, 0, 0, false, false, fmt.Errorf("page %d out of range, total pages: %d", requestedPage, totalPages)
		}
		start := providerPage * pageSize
		if start > totalCount {
			start = totalCount
		}
		end := start + pageSize
		if end > totalCount {
			end = totalCount
		}
		products := make(ProductsResponse, 0, end-start)
		for _, product := range plan.DataPackages[start:end] {
			products = append(products, Product{
				Name:        product.Name,
				UniqueCode:  fmt.Sprintf("%s|%d", plan.NetworkProvider, product.ID),
				Network:     plan.NetworkProvider,
				PackageCode: product.ID,
				FixedAmount: true,
				Amount:      float32(product.Amount),
				BillerName:  plan.NetworkProvider,
				CategoryDTO: struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}{ID: categoryID, Name: "Data"},
			})
		}
		hasPrev := requestedPage > 1
		hasNext := requestedPage < totalPages
		return &products, requestedPage, pageSize, totalCount, totalPages, hasNext, hasPrev, nil
	}
	return nil, 0, 0, 0, 0, false, false, fmt.Errorf("wema vas: data network %d not found", billerID)
}

func (s *Service) getWemaAirtime(ctx context.Context, payload AirtimePayload, mobileUserID, phoneNumber string, wallet *CustomerWallet) (*vasprovider.ISPResponse, error) {
	if s.Wema == nil {
		return nil, appErr.ErrWemaUnavailable
	}
	if payload.UseCashback {
		return nil, appErr.ErrWemaCashbackUnsupported
	}
	if payload.Amount < 100 || payload.Amount > 10000 {
		return nil, appErr.ErrInvalidISPAmount
	}
	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}
	if err := s.checkWemaBalance(ctx, wallet, float64(payload.Amount)); err != nil {
		return nil, err
	}
	network := strings.TrimSpace(payload.Network)
	if network == "" {
		network = strings.TrimSpace(payload.UniqueCode)
		if strings.Contains(network, "_") {
			network = ExtractBillingCompanyName(network)
		}
	}
	txID, err := s.createWemaVASTransaction(ctx, wallet, TransactionCategoryAirtime, "Airtime", payload.Amount*100, 0, network, phoneNumber, "airtime")
	if err != nil {
		return nil, err
	}
	resp, err := s.Wema.PurchaseAirtime(ctx, wallet.AccountNumber, network, phoneNumber, float64(payload.Amount), txID)
	if err != nil {
		handledErr := s.handleWemaVASCallError(ctx, mobileUserID, txID, wallet.AvailableBalance, err, appErr.ErrGettingAirtime)
		if errors.Is(handledErr, appErr.ErrVASAmbiguous) {
			return wemaAirtimeResult(txID, "", "Pending ALAT confirmation", network, phoneNumber, float64(payload.Amount), false), nil
		}
		return nil, handledErr
	}
	status, message := wemaVASStatus(resp)
	if err := s.finishWemaVASTransaction(ctx, wallet, txID, payload.Amount*100, status, "airtime", message); err != nil {
		return nil, err
	}
	if status == walletmodule.ProviderTransferFailed {
		return nil, appErr.ErrGettingAirtime
	}
	if status == walletmodule.ProviderTransferPending && message == "" {
		message = "Pending ALAT confirmation"
	}
	return wemaAirtimeResult(txID, wemaVASReference(resp), message, network, phoneNumber, float64(payload.Amount), status == walletmodule.ProviderTransferSuccessful), nil
}

func (s *Service) getWemaData(ctx context.Context, payload DataPayload, mobileUserID, phoneNumber string, wallet *CustomerWallet) (*vasprovider.ISPResponse, error) {
	if s.Wema == nil {
		return nil, appErr.ErrWemaUnavailable
	}
	if payload.UseCashback {
		return nil, appErr.ErrWemaCashbackUnsupported
	}
	if payload.Amount < 100 || payload.Amount > 10000 {
		return nil, appErr.ErrInvalidISPAmount
	}
	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}
	if err := s.checkWemaBalance(ctx, wallet, float64(payload.Amount)); err != nil {
		return nil, err
	}
	network, packageCode, err := s.wemaDataSelection(payload)
	if err != nil {
		return nil, err
	}
	txID, err := s.createWemaVASTransaction(ctx, wallet, TransactionCategoryMobileData, "Mobile Data", payload.Amount*100, 0, network, phoneNumber, "data")
	if err != nil {
		return nil, err
	}
	resp, err := s.Wema.PurchaseData(ctx, wallet.AccountNumber, network, phoneNumber, packageCode, float64(payload.Amount), txID)
	if err != nil {
		handledErr := s.handleWemaVASCallError(ctx, mobileUserID, txID, wallet.AvailableBalance, err, appErr.ErrGettingData)
		if errors.Is(handledErr, appErr.ErrVASAmbiguous) {
			return wemaAirtimeResult(txID, "", "Pending ALAT confirmation", network, phoneNumber, float64(payload.Amount), false), nil
		}
		return nil, handledErr
	}
	status, message := wemaVASStatus(resp)
	if err := s.finishWemaVASTransaction(ctx, wallet, txID, payload.Amount*100, status, "data", message); err != nil {
		return nil, err
	}
	if status == walletmodule.ProviderTransferFailed {
		return nil, appErr.ErrGettingData
	}
	if status == walletmodule.ProviderTransferPending && message == "" {
		message = "Pending ALAT confirmation"
	}
	return wemaAirtimeResult(txID, wemaVASReference(resp), message, network, phoneNumber, float64(payload.Amount), status == walletmodule.ProviderTransferSuccessful), nil
}

func wemaAirtimeResult(txID, reference, message, network, phoneNumber string, amount float64, successful bool) *vasprovider.ISPResponse {
	out := &vasprovider.ISPResponse{}
	out.RequestID = txID
	out.ReferenceID = reference
	out.ResponseCode = "01"
	if successful {
		out.ResponseCode = "00"
	}
	out.ResponseMessage = message
	out.Data.Amount = amount
	out.Data.PhoneNumber = phoneNumber
	out.Data.Channel = network
	return out
}

func (s *Service) wemaDataSelection(payload DataPayload) (string, int, error) {
	network := strings.TrimSpace(payload.Network)
	packageCode := payload.PackageCode
	if network != "" && packageCode > 0 {
		return network, packageCode, nil
	}
	network, rawCode, ok := strings.Cut(strings.TrimSpace(payload.UniqueCode), "|")
	if !ok {
		return "", 0, appErr.ErrInvalidRequestBody
	}
	packageCode, err := strconv.Atoi(strings.TrimSpace(rawCode))
	if err != nil || packageCode <= 0 || strings.TrimSpace(network) == "" {
		return "", 0, appErr.ErrInvalidRequestBody
	}
	return strings.TrimSpace(network), packageCode, nil
}

func (s *Service) payWemaBill(ctx context.Context, mobileUserID string, wallet *CustomerWallet, packageID int, accountNumber string, amount int64, charge float64, billType string, userName, phoneNumber string) (*baas.AlatClientVASResponse, string, error) {
	if s.Wema == nil {
		return nil, "", appErr.ErrWemaUnavailable
	}
	if packageID <= 0 {
		return nil, "", appErr.ErrInvalidRequestBody
	}
	if charge < 0 {
		return nil, "", appErr.ErrInvalidRequestBody
	}
	if charge == 0 {
		bills, err := s.Wema.GetBills(ctx)
		if err != nil {
			return nil, "", err
		}
		found := false
		for _, category := range bills.Result {
			for _, biller := range category.Billers {
				for _, product := range biller.Packages {
					if product.ID == packageID {
						charge = biller.Charge
						found = true
						break
					}
				}
			}
		}
		if !found {
			return nil, "", appErr.ErrInvalidRequestBody
		}
	}
	if err := s.checkWemaBalance(ctx, wallet, float64(amount)+charge); err != nil {
		return nil, "", err
	}
	validation, err := s.Wema.ValidateBill(ctx, accountNumber, packageID)
	if err != nil {
		return nil, "", err
	}
	category := TransactionCategoryElectricity
	validationFailure := appErr.ErrValidatingElectricity
	paymentFailure := appErr.ErrPayingElectricityBill
	if billType == "cable" {
		category = TransactionCategoryTV
		validationFailure = appErr.ErrValidatingCable
		paymentFailure = appErr.ErrPayingCableBill
	}
	if validation.Result == nil || !validation.Result.IsValidated {
		return nil, "", validationFailure
	}
	if strings.TrimSpace(validation.Result.CustomerName) != "" {
		userName = validation.Result.CustomerName
	}
	amountKobo := amount * 100
	chargeKobo := int64(math.Round(charge * 100))
	txID, err := s.createWemaVASTransaction(ctx, wallet, category, billType, amountKobo, chargeKobo, billType, accountNumber, billType)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.Wema.PayBill(ctx, accountNumber, accountNumber, "", phoneNumber, userName, packageID, float64(amount), float64(charge), txID)
	if err != nil {
		return nil, txID, s.handleWemaVASCallError(ctx, mobileUserID, txID, wallet.AvailableBalance, err, paymentFailure)
	}
	status, message := wemaVASStatus(resp)
	if err := s.finishWemaVASTransaction(ctx, wallet, txID, amountKobo+chargeKobo, status, billType, message); err != nil {
		return nil, txID, err
	}
	return resp, txID, nil
}

func (s *Service) payWemaElectricity(ctx context.Context, payload PayElectricityPayload, mobileUserID string, wallet *CustomerWallet, userName, phoneNumber string) (*vasprovider.PayElectricityResponse, error) {
	if payload.UseCashback {
		return nil, appErr.ErrWemaCashbackUnsupported
	}
	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}
	packageID, err := wemaPackageID(payload.UniqueCode, payload.PackageID)
	if err != nil {
		return nil, err
	}
	resp, txID, err := s.payWemaBill(ctx, mobileUserID, wallet, packageID, payload.AccountNumber, payload.Amount, payload.Charge, "electricity", userName, phoneNumber)
	if err != nil {
		if errors.Is(err, appErr.ErrVASAmbiguous) {
			return wemaElectricityResult(txID, "", "Pending ALAT confirmation", payload.AccountNumber, payload.Amount, userName, false), nil
		}
		return nil, err
	}
	status, message := wemaVASStatus(resp)
	if status == walletmodule.ProviderTransferFailed {
		return nil, appErr.ErrPayingElectricityBill
	}
	if status == walletmodule.ProviderTransferPending && message == "" {
		message = "Pending ALAT confirmation"
	}
	result := wemaElectricityResult(txID, wemaVASReference(resp), message, payload.AccountNumber, payload.Amount, userName, status == walletmodule.ProviderTransferSuccessful)
	if resp != nil && resp.Result != nil {
		result.Data.Token = strings.TrimSpace(resp.Result.Value)
	}
	return result, nil
}

func wemaElectricityResult(txID, reference, message, accountNumber string, amount int64, userName string, successful bool) *vasprovider.PayElectricityResponse {
	out := &vasprovider.PayElectricityResponse{}
	out.RequestID = txID
	out.ReferenceID = reference
	out.ResponseCode = "01"
	if successful {
		out.ResponseCode = "00"
	}
	out.ResponseMessage = message
	out.Data.AccountNumber = accountNumber
	out.Data.Amount = strconv.FormatInt(amount, 10)
	return out
}

func (s *Service) payWemaCable(ctx context.Context, payload PayCablePayload, mobileUserID string, wallet *CustomerWallet, userName, phoneNumber string) (*vasprovider.PayCableResponse, error) {
	if payload.UseCashback {
		return nil, appErr.ErrWemaCashbackUnsupported
	}
	if err := s.PinVerifier.VerifyTransactionPin(ctx, mobileUserID, strings.TrimSpace(payload.Pin)); err != nil {
		return nil, err
	}
	packageID, err := wemaPackageID(payload.UniqueCode, payload.PackageID)
	if err != nil {
		return nil, err
	}
	resp, txID, err := s.payWemaBill(ctx, mobileUserID, wallet, packageID, payload.AccountNumber, payload.Amount, payload.Charge, "cable", userName, phoneNumber)
	if err != nil {
		if errors.Is(err, appErr.ErrVASAmbiguous) {
			return wemaCableResult(txID, "", "Pending ALAT confirmation", payload.AccountNumber, payload.Amount, userName, false), nil
		}
		return nil, err
	}
	status, message := wemaVASStatus(resp)
	if status == walletmodule.ProviderTransferFailed {
		return nil, appErr.ErrPayingCableBill
	}
	if status == walletmodule.ProviderTransferPending && message == "" {
		message = "Pending ALAT confirmation"
	}
	return wemaCableResult(txID, wemaVASReference(resp), message, payload.AccountNumber, payload.Amount, userName, status == walletmodule.ProviderTransferSuccessful), nil
}

func wemaCableResult(txID, reference, message, accountNumber string, amount int64, userName string, successful bool) *vasprovider.PayCableResponse {
	out := &vasprovider.PayCableResponse{}
	out.RequestID = txID
	out.ReferenceID = reference
	out.ResponseCode = "01"
	if successful {
		out.ResponseCode = "00"
	}
	out.ResponseMessage = message
	out.Data.AccountNumber = accountNumber
	out.Data.Amount = strconv.FormatInt(amount, 10)
	out.Data.Name = userName
	return out
}

func (s *Service) checkWemaBalance(ctx context.Context, wallet *CustomerWallet, amount float64) error {
	if s.Wema == nil {
		return appErr.ErrWemaUnavailable
	}
	balance, err := s.Wema.GetWalletBalance(ctx, wallet.AccountNumber)
	if err != nil {
		log.Printf("vas service: failed to check Wema balance: %v", err)
		return appErr.ErrProviderServiceUnavailable
	}
	if balance < amount {
		return appErr.ErrInsufficientBalance
	}
	wallet.AvailableBalance = int64(math.Round(balance * 100))
	return nil
}

func (s *Service) createWemaVASTransaction(ctx context.Context, wallet *CustomerWallet, category TransactionCategory, description string, amount, charges int64, counterpartyName, counterpartyAccount, typ string) (string, error) {
	txID := uuid.NewString()
	txn := Transaction{
		ID:                  txID,
		MobileUserID:        wallet.MobileUserID,
		WalletID:            wallet.InternalWalletID,
		Type:                TransactionTypeDebit,
		Category:            category,
		Description:         description,
		Amount:              amount,
		Charges:             charges,
		BalanceBefore:       wallet.AvailableBalance,
		Reference:           txID,
		CounterpartyName:    counterpartyName,
		CounterpartyAccount: counterpartyAccount,
		Status:              TransactionStatusPending,
		Source:              TransactionSourceDebit,
		Metadata:            map[string]any{"provider": "wema", "type": typ},
		VASRequestID:        txID,
		CreatedAt:           time.Now().UTC(),
	}
	if err := s.Txr.AddTransaction(ctx, &txn); err != nil {
		return "", err
	}
	return txID, nil
}

func (s *Service) handleWemaVASCallError(ctx context.Context, mobileUserID, txID string, balanceBefore int64, err, failure error) error {
	var alatErr *appErr.AlatError
	if errors.As(err, &alatErr) && alatErr.NoResponse {
		s.logVASAudit(ctx, mobileUserID, txID, auditlog.StatusFailure, map[string]interface{}{
			"provider": "wema",
			"reason":   appErr.ErrVASAmbiguous.Error(),
		})
		return appErr.ErrVASAmbiguous
	}
	if updateErr := s.Txr.UpdateTransactionStatus(ctx, txID, balanceBefore, TransactionStatusFailed); updateErr != nil {
		log.Printf("vas service: failed to mark Wema VAS transaction %s failed: %v", txID, updateErr)
	}
	s.logVASAudit(ctx, mobileUserID, txID, auditlog.StatusFailure, map[string]interface{}{
		"provider": "wema",
		"reason":   err.Error(),
	})
	return failure
}

func (s *Service) finishWemaVASTransaction(ctx context.Context, wallet *CustomerWallet, txID string, amount int64, status walletmodule.ProviderTransferStatus, typ, message string) error {
	switch status {
	case walletmodule.ProviderTransferSuccessful:
		balanceAfter := wallet.AvailableBalance - amount
		if err := s.Txr.UpdateTransactionStatus(ctx, txID, balanceAfter, TransactionStatusSuccessful); err != nil {
			return err
		}
		s.logVASAudit(ctx, wallet.MobileUserID, txID, auditlog.StatusSuccess, map[string]interface{}{
			"provider": "wema",
			"type":     typ,
		})
	case walletmodule.ProviderTransferFailed:
		if err := s.Txr.UpdateTransactionStatus(ctx, txID, wallet.AvailableBalance, TransactionStatusFailed); err != nil {
			return err
		}
		s.logVASAudit(ctx, wallet.MobileUserID, txID, auditlog.StatusFailure, map[string]interface{}{
			"provider": "wema",
			"type":     typ,
			"reason":   message,
		})
	default:
		s.logVASAudit(ctx, wallet.MobileUserID, txID, auditlog.StatusFailure, map[string]interface{}{
			"provider": "wema",
			"type":     typ,
			"reason":   appErr.ErrVASAmbiguous.Error(),
		})
	}
	return nil
}

func wemaVASStatus(resp *baas.AlatClientVASResponse) (walletmodule.ProviderTransferStatus, string) {
	if resp == nil || resp.Result == nil {
		return walletmodule.ProviderTransferPending, "ALAT returned no transaction result"
	}
	status := strings.ToLower(strings.TrimSpace(resp.Result.Status))
	message := strings.TrimSpace(resp.Result.Message)
	if message == "" {
		message = strings.TrimSpace(resp.Result.Narration)
	}
	switch {
	case strings.Contains(status, "fail"), strings.Contains(status, "declin"),
		strings.Contains(status, "reject"), strings.Contains(status, "cancel"),
		strings.Contains(status, "unsuccess"), strings.Contains(status, "not successful"):
		return walletmodule.ProviderTransferFailed, message
	case strings.Contains(status, "success"), status == "complete", status == "completed":
		return walletmodule.ProviderTransferSuccessful, message
	default:
		return walletmodule.ProviderTransferPending, message
	}
}

func wemaVASReference(resp *baas.AlatClientVASResponse) string {
	if resp == nil || resp.Result == nil {
		return ""
	}
	return strings.TrimSpace(resp.Result.PlatformTransactionReference)
}

func (s *Service) CheckStatusForUser(ctx context.Context, requestID, mobileUserID string) (*vasprovider.CheckStatusResponse, error) {
	var txn Transaction
	err := s.Repo.db.WithContext(ctx).
		Where("vas_request_id = ? AND mobile_user_id = ?", requestID, mobileUserID).
		First(&txn).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil && txn.Metadata["provider"] == "wema" {
		out := &vasprovider.CheckStatusResponse{}
		out.RequestID = requestID
		switch txn.Status {
		case TransactionStatusSuccessful:
			out.ResponseCode = "00"
			out.ResponseMessage = "Successful"
		case TransactionStatusFailed, TransactionStatusReversed:
			out.ResponseCode = "99"
			out.ResponseMessage = "Failed"
		default:
			out.ResponseCode = "01"
			out.ResponseMessage = "Pending"
		}
		return out, nil
	}
	return s.CheckStatus(ctx, requestID)
}

func wemaPackageID(raw string, explicit int) (int, error) {
	if explicit > 0 {
		return explicit, nil
	}
	id, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || id <= 0 {
		return 0, appErr.ErrInvalidRequestBody
	}
	return id, nil
}
