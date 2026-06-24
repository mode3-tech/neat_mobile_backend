package vas

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	appErr "neat_mobile_app_backend/internal/errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type XpressPayments struct {
	PublicKey  string
	PrivateKey string
	BaseURL    string
	Client     *http.Client
}

func NewXpressPayments(publicKey, privateKey, baseURL string) (*XpressPayments, error) {
	if strings.TrimSpace(publicKey) == "" || strings.TrimSpace(privateKey) == "" {
		return nil, &appErr.XpressWalletProviderError{Code: "", Message: "xpress payments: public and private keys are required"}
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, &appErr.XpressWalletProviderError{Code: "", Message: "xpress payments: base URL is required"}
	}
	return &XpressPayments{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		BaseURL:    baseURL,
		Client:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func readResponseBody(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func (x *XpressPayments) FetchAllCategories(ctx context.Context) (*CategoriesResponse, error) {
	url := x.BaseURL + "/api/v1/categories"

	payload := map[string]int{
		"size": 10,
		"page": 0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: "failed to marshal payload"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: "failed to create request"}
	}

	req.Header.Set("Authorization", "Bearer "+x.PublicKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: "failed to read response body"}
	}

	var result CategoriesResponse
	if err := json.Unmarshal(respBodyBytes, &result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: "failed to decode response"}
	}

	return &result, nil
}

func (x *XpressPayments) FetchBillersByCategoryID(ctx context.Context, categoryID, page, size int) (*BillersByCategoryIDResponse, error) {
	baseURL := x.BaseURL + "/api/v1/billers"

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to parse billers url: %v", err)}
	}

	q := u.Query()
	q.Set("categoryId", strconv.Itoa(categoryID))
	u.RawQuery = q.Encode()

	payload := map[string]int{
		"page": page,
		"size": size,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result BillersByCategoryIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) FetchProductsByCategoryIDAndBillerID(ctx context.Context, categoryID, billerID, page, size int) (*ProductResponse, error) {
	reqURL := x.BaseURL + "/api/v1/products"
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to parse products url: %v", err)}
	}

	q := u.Query()
	q.Set("categoryId", strconv.Itoa(categoryID))
	q.Set("billerId", strconv.Itoa(billerID))
	u.RawQuery = q.Encode()

	payload := map[string]int{
		"page": page,
		"size": size,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result ProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) GetAirtime(ctx context.Context, requestID, uniqueCode, phoneNumber string, amount int64) (*ISPResponse, error) {
	url := x.BaseURL + "/api/v1/airtime/fulfil"

	payload := ISPPayload{
		Payload: Payload{RequestID: requestID, UniqueCode: uniqueCode},
		Details: ispDetails{
			PhoneNumber: phoneNumber,
			Amount:      amount,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+x.PublicKey)
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result ISPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) GetData(ctx context.Context, requestId, uniqueCode, phoneNumber string, amount int64) (*ISPResponse, error) {
	url := x.BaseURL + "/api/v1/data/fulfil"

	payload := ISPPayload{
		Payload: Payload{RequestID: requestId, UniqueCode: uniqueCode},
		Details: ispDetails{
			PhoneNumber: phoneNumber,
			Amount:      amount,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result ISPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) ValidateElectricity(ctx context.Context, requestId, uniqueCode, accountNumber string, accountType AccountType) (*ElectricityValidationResponse, error) {
	url := x.BaseURL + "/api/v1/electricity/validate"

	payload := ElectricityValidationPayload{
		Payload: Payload{
			RequestID:  requestId,
			UniqueCode: uniqueCode,
		},
		Details: electricityValidationDetails{
			AccountNumber: accountNumber,
			AccountType:   accountType,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "SERVER_ERROR", Message: msg}
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result ElectricityValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) PayElectricityBill(ctx context.Context, requestId, uniqueCode, accountNumber, name, address, phoneNumber string, accountType AccountType, amount int64) (*PayElectricityResponse, error) {
	url := x.BaseURL + "/api/v1/electricity/fulfil"

	payload := PayElectricityBillPayload{
		Payload: Payload{
			RequestID:  requestId,
			UniqueCode: uniqueCode,
		},
		Details: payElectricityBillDetails{
			AccountNumber: accountNumber,
			AccountType:   accountType,
			Amount:        amount,
			Address:       address,
			PhoneNumber:   phoneNumber,
			Name:          name,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result PayElectricityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) ValidateCable(ctx context.Context, requestId, uniqueCode, accountNumber string, noOfMonth int) (*CableValidationResponse, error) {
	url := x.BaseURL + "/api/v1/cable/validate"

	payload := CableValidationPayload{
		Payload: Payload{RequestID: requestId, UniqueCode: uniqueCode},
		Details: cableValidationDetails{
			AccountNumber: accountNumber,
			NoOfMonth:     noOfMonth,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "SERVER_ERROR", Message: msg}
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result CableValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) PayCableBill(ctx context.Context, requestId, uniqueCode, accountNumber, accountType, name, phoneNumber string, noOfMonth int, amount int64) (*PayCableResponse, error) {
	url := x.BaseURL + "/api/v1/cable/fulfil"

	payload := PayCableBillPayload{
		Payload: Payload{RequestID: requestId, UniqueCode: uniqueCode},
		Details: payCableBillDetails{
			AccountNumber: accountNumber,
			AccountType:   accountType,
			NoOfMonth:     noOfMonth,
			Amount:        amount,
			Name:          name,
			PhoneNumber:   phoneNumber,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	var result PayCableResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) GetWAECResultCheckerPin(ctx context.Context, requestId, uniqueCode, email, phoneNumber, amount string) (*WAECResultCheckerPinResponse, error) {
	url := x.BaseURL + "/api/v1/education/waec/pin/fulfil"

	payload := map[string]any{
		"requestId":  requestId,
		"uniqueCode": uniqueCode,
		"details": map[string]string{
			"email":       email,
			"phoneNumber": phoneNumber,
			"amount":      amount,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	hash, err := generatePaymentHash(jsonPayload, x.PrivateKey)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", hash)
	req.Header.Set("Accept", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to read response body: %v", err)}
	}

	var result WAECResultCheckerPinResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) ValidateWAECRegistration(ctx context.Context, requestId, uniqueCode string, numberOfCandidate int) (*WAECRegistrationValidationResponse, error) {
	url := x.BaseURL + "/api/v1/education/waec/registration/validate"

	payload := map[string]interface{}{
		"requestId":  requestId,
		"uniqueCode": uniqueCode,
		"details": map[string]int{
			"noOfCandidate": numberOfCandidate,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to marshal payload: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to generate payment hash: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "SERVER_ERROR", Message: msg}
	}

	if resp.StatusCode != http.StatusOK {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "REQUEST_FAILED", Message: msg}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to read response body: %v", err)}
	}

	var result WAECRegistrationValidationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) CheckStatus(ctx context.Context, requestID string) (*CheckStatusResponse, error) {
	url := x.BaseURL + "/api/v1/status/" + url.PathEscape(requestID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Accept", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "SERVER_ERROR", Message: msg}
	}

	// Non-OK responses carry a JSON body with the business error
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		var errResp Response
		if json.Unmarshal(body, &errResp) == nil && errResp.ResponseMessage != "" {
			return nil, &appErr.XpressWalletProviderError{
				Status:  resp.StatusCode,
				Code:    errResp.ResponseCode,
				Message: errResp.ResponseMessage,
			}
		}
		return nil, &appErr.XpressWalletProviderError{
			Status:  resp.StatusCode,
			Code:    "REQUEST_FAILED",
			Message: strings.TrimSpace(string(body)),
		}
	}

	var result CheckStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func (x *XpressPayments) GetWalletBalance(ctx context.Context) (*WalletBalanceResponse, error) {
	url := x.BaseURL + "/api/v1/wallets/balance"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 500, Code: "", Message: fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Accept", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, &appErr.XpressWalletProviderError{Status: 408, Code: "TIMEOUT", Message: "request timed out"}
		}
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "NETWORK_ERROR", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		msg := readResponseBody(resp)
		return nil, &appErr.XpressWalletProviderError{Status: resp.StatusCode, Code: "SERVER_ERROR", Message: msg}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		var errResp Response
		if json.Unmarshal(body, &errResp) == nil && errResp.ResponseMessage != "" {
			return nil, &appErr.XpressWalletProviderError{
				Status:  resp.StatusCode,
				Code:    errResp.ResponseCode,
				Message: errResp.ResponseMessage,
			}
		}
		return nil, &appErr.XpressWalletProviderError{
			Status:  resp.StatusCode,
			Code:    "REQUEST_FAILED",
			Message: strings.TrimSpace(string(body)),
		}
	}

	var result WalletBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &appErr.XpressWalletProviderError{Status: 502, Code: "", Message: fmt.Sprintf("failed to decode response: %v", err)}
	}

	return &result, nil
}

func generatePaymentHash(payload []byte, privateKey string) (string, error) {
	mac := hmac.New(sha512.New, []byte(privateKey))

	_, err := mac.Write(payload)
	if err != nil {
		return "", err
	}

	result := mac.Sum(nil)
	computed := hex.EncodeToString(result)

	return computed, nil
}
