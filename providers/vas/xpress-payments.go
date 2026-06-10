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
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
)

type XpressPayments struct {
	PublicKey  string
	PrivateKey string
	Client     *http.Client
}

func NewXpressPayments(publicKey, privateKey string) (*XpressPayments, error) {
	if strings.TrimSpace(publicKey) == "" || strings.TrimSpace(privateKey) == "" {
		log.Println("xpress payments: public and private keys are required")
		return nil, errors.New("xpress payments: public and private keys are required")
	}
	return &XpressPayments{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Client:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (x *XpressPayments) FetchAllCategories(ctx context.Context) (*CategoriesResponse, error) {
	url := "https://billerstest.xpresspayments.com:9603/api/v1/products"

	payload := map[string]int{
		"size": 10,
		"page": 0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("xpress pay: failed to marshal payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress pay: failed to create new request - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+x.PublicKey)

	resp, err := x.Client.Do(req)
	if err != nil {
		log.Printf("xpress pay: request failed - %s\n", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress pay: request failed with status code: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	var result CategoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("xpress pay: failed to decode body into json - %s\n", err)
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) FetchBillersByCategoryID(ctx context.Context, categoryID, page, size int) (*BillersByCategoryIDResponse, error) {
	baseURL := "https://billerstest.xpresspayments.com:9603/api/v1/billers"

	u, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("xpress pay: failed to parse billers url - %s\n", err)
		return nil, err
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
		log.Printf("xpress pay: failed to marshal billers payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress pay: failed to create billers request - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		log.Printf("xpress pay: billers request failed - %s\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress pay: billers request failed with status code: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	var result BillersByCategoryIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("xpress pay: failed to decode billers response - %s\n", err)
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) FetchProductsByCategoryIDAndBillerID(ctx context.Context, categoryID, billerID, page, size int) (*ProductResponse, error) {
	reqURL := "https://billerstest.xpresspayments.com:9603/api/v1/products"
	u, err := url.Parse(reqURL)
	if err != nil {
		log.Printf("xpress pay: failed to parse products url - %s\n", err)
		return nil, err
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
		log.Printf("xpress pay: failed to marshal category products payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress pay: failed to create category products request - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := x.Client.Do(req)
	if err != nil {
		log.Printf("xpress pay: request failed - %s\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress pay: request failed with status code: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	var result ProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("xpress pay: failed to decode body into json - %s\n", err)
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) GetAirtime(ctx context.Context, requestID, uniqueCode, phoneNumber string, amount int64) (*ISPResponse, error) {
	url := "https://billerstest.xpresspayments.com:9603/api/v1/airtime/fulfil"

	payload := ISPPayload{
		Payload: Payload{RequestID: requestID, UniqueCode: uniqueCode},
		Details: ispDetails{
			PhoneNumber: phoneNumber,
			Amount:      amount,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+x.PublicKey)
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		log.Printf("xpress payments: server error %d — outcome unknown\n", resp.StatusCode)
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress payments: non-OK response %d\n", resp.StatusCode)
		return nil, fmt.Errorf("request failed")
	}

	var result ISPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) GetData(ctx context.Context, requestId, uniqueCode, phoneNumber string, amount int64) (*ISPResponse, error) {
	url := "https://billerstest.xpresspayments.com:9603/api/v1/data/fulfil"

	payload := ISPPayload{
		Payload: Payload{RequestID: requestId, UniqueCode: uniqueCode},
		Details: ispDetails{
			PhoneNumber: phoneNumber,
			Amount:      amount,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("xpress payments: failed to marshal payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress payments: failed to create a request - %s\n", err)
		return nil, err
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		log.Printf("xpress payments: failed to generate payment hash for getting data - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		log.Printf("xpress payments: failed to make data request - %s\n", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		log.Printf("xpress payments: server error %d on data request — outcome unknown\n", resp.StatusCode)
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress payments: non-OK response %d on data request\n", resp.StatusCode)
		return nil, errors.New("failed to make request")
	}

	var result ISPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("xpress payments: failed to decode data response body - %s", err)
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) ValidateElectricity(ctx context.Context, requestId, uniqueCode, accountNumber string, accountType AccountType) (*ElectricityValidationResponse, error) {
	url := "https://billerstest.xpresspayments.com:9603/api/v1/electricity/validate"

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
		log.Printf("xpress pay: failed to marshal the payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress pay: failed to create new request - %s\n", err)
		return nil, err
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		log.Printf("xpress pay: failed to generate payment hash - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)

	resp, err := x.Client.Do(req)
	if err != nil {
		log.Printf("xpress pay: failed to send http req to xpress pay - %s\n", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress pay: request failed with code: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("xpress pay: electricity validation failed with status code: %d", resp.StatusCode)
	}

	var result ElectricityValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("xpress pay: failed to decode response body")
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) PayElectricityBill(ctx context.Context, requestId, uniqueCode, accountNumber, name, address, phoneNumber string, accountType AccountType, amount int64) (*PayElectricityResponse, error) {
	url := "https://billerstest.xpresspayments.com:9603/api/v1/electricity/fulfil"

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
		log.Printf("xpress pay: failed to marshal payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress pay: failed to create a request - %s\n", err)
		return nil, err
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		log.Printf("xpress pay: failed to generate payment hash - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		log.Printf("xpress pay: failed to get a response from the provider - %s\n", err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		log.Printf("xpress pay: server error %d on electricity payment — outcome unknown\n", resp.StatusCode)
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress pay: electricity payment failed with status code: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("xpress pay: request failed with status code: %d", resp.StatusCode)
	}

	var result PayElectricityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("xpress pay: failed to decode response body into json - %s\n", err)
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) ValidateCable(ctx context.Context, requestId, uniqueCode, accountNumber string, noOfMonth int) (*CableValidationResponse, error) {
	url := "https://billerstest.xpresspayments.com:9603/api/v1/cable/validate"

	payload := CableValidationPayload{
		Payload: Payload{RequestID: requestId, UniqueCode: uniqueCode},
		Details: cableValidationDetails{
			AccountNumber: accountNumber,
			NoOfMonth:     noOfMonth,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("xpress pay: failed to marshal cable validation payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress pay: failed to create cable validation request - %s\n", err)
		return nil, err
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		log.Printf("xpress pay: failed to generate payment hash for cable validation - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)

	resp, err := x.Client.Do(req)
	if err != nil {
		log.Printf("xpress pay: failed to send cable validation request - %s\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress pay: cable validation failed with status code: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("xpress pay: cable validation failed with status code: %d", resp.StatusCode)
	}

	var result CableValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("xpress pay: failed to decode cable validation response - %s\n", err)
		return nil, err
	}

	return &result, nil
}

func (x *XpressPayments) PayCableBill(ctx context.Context, requestId, uniqueCode, accountNumber, accountType, name, phoneNumber string, noOfMonth int, amount int64) (*PayCableResponse, error) {
	url := "https://billerstest.xpresspayments.com:9603/api/v1/cable/fulfil"

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
		log.Printf("xpress pay: failed to marshal cable bill payload - %s\n", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("xpress pay: failed to create cable bill request - %s\n", err)
		return nil, err
	}

	paymentHash, err := generatePaymentHash(body, strings.TrimSpace(x.PrivateKey))
	if err != nil {
		log.Printf("xpress pay: failed to generate payment hash for cable bill - %s\n", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(x.PublicKey))
	req.Header.Set("Channel", "api")
	req.Header.Set("PaymentHash", paymentHash)

	resp, err := x.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, appErr.ErrVASAmbiguous
		}
		log.Printf("xpress pay: failed to send cable bill request - %s\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		log.Printf("xpress pay: server error %d on cable payment — outcome unknown\n", resp.StatusCode)
		return nil, appErr.ErrVASAmbiguous
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("xpress pay: cable bill failed with status code: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("xpress pay: cable bill failed with status code: %d", resp.StatusCode)
	}

	var result PayCableResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("xpress pay: failed to decode cable bill response - %s\n", err)
		return nil, err
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
