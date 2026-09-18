package baas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// optimusTransferPathPrefix is shared by the bank-list, name-enquiry, and
// transfer endpoints - all live under this gateway prefix, unlike
// GenerateWallet ("/Customer/...") or ValidateAccount/UpgradeAccount
// ("/kyc-api/api/v1/...").
const optimusTransferPathPrefix = "/opti-finserve-api/v1"

// OptimusBank is one entry in the Get Banks response.
type OptimusBank struct {
	BankCode string `json:"bankCode"`
	BankName string `json:"bankName"`
}

type optimusBankListResponse struct {
	ResponseMessage string        `json:"responseMessage"`
	ResponseCode    string        `json:"responseCode"`
	Error           any           `json:"error"`
	Data            []OptimusBank `json:"data"`
}

// OptimusNameEnquiryData is the decoded Name Enquiry response - SessionID is
// required on the subsequent Transfer call for interbank transfers, and left
// out entirely for intrabank ones.
type OptimusNameEnquiryData struct {
	BankCode      string `json:"bankCode"`
	SessionID     string `json:"sessionId"`
	AccountNumber string `json:"accountNumber"`
	AccountName   string `json:"accountName"`
	BVN           string `json:"bvn"`
	KYCLevel      string `json:"kycLevel"`
}

type optimusNameEnquiryResponse struct {
	ResponseMessage string                 `json:"responseMessage"`
	ResponseCode    string                 `json:"responseCode"`
	Error           any                    `json:"error"`
	Data            OptimusNameEnquiryData `json:"data"`
}

// OptimusTransferRequest is the (pre-encryption) shape of a transfer request.
// SessionId must be left empty for intrabank transfers (beneficiary bank code
// equals the source account's own bank code) and populated with the SessionID
// from a fresh Name Enquiry call otherwise - see docs/README(1).md.
type OptimusTransferRequest struct {
	RequestId            string  `json:"requestId"`
	TransactionReference string  `json:"transactionReference"`
	Amount               float64 `json:"amount"`
	Narration            string  `json:"narration"`
	SourceAccount        string  `json:"sourceAccount"`
	BeneficiaryAccount   string  `json:"beneficiaryAccount"`
	BeneficiaryBankCode  string  `json:"beneficiaryBankCode"`
	SessionId            string  `json:"sessionId"`
}

type OptimusTransferResult struct {
	ResponseCode             string  `json:"responseCode"`
	ResponseMessage          string  `json:"responseMessage"`
	TransactionAmount        float64 `json:"transactionAmount"`
	TransactionAmountInWords string  `json:"transactionAmountInWords"`
	TransactionDate          string  `json:"transactionDate"`
	TransactionReference     string  `json:"transactionReference"`
	AccountDebited           string  `json:"accountDebited"`
	AccountCredited          string  `json:"accountCredited"`
	SenderName               string  `json:"senderName"`
	BeneficiaryName          string  `json:"beneficiaryName"`
	BeneficiaryBankName      string  `json:"beneficiaryBankName"`
	TransactionID            string  `json:"transactionId"`
	PaymentReference         string  `json:"paymentReference"`
	SessionID                string  `json:"sessionID"`
}

type OptimusTransferResponse struct {
	ResponseMessage string                  `json:"responseMessage"`
	ResponseCode    string                  `json:"responseCode"`
	Error           any                     `json:"error"`
	Data            []OptimusTransferResult `json:"data"`
}

// FetchBanks retrieves the list of banks Optimus can transfer to (including
// Optimus's own bank code, used elsewhere to distinguish intra- from
// interbank transfers). Request is plain; the response is decrypted like
// everywhere else via decodeOptimusEnvelope.
func (o *Optimus) FetchBanks(ctx context.Context) ([]OptimusBank, error) {
	baseURL := strings.TrimSpace(o.WalletBaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("optimus service is not configured")
	}

	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: get token failed: %v", err)
		return nil, fmt.Errorf("optimus: get token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+optimusTransferPathPrefix+"/Bank", nil)
	if err != nil {
		log.Printf("optimus: build bank list request: %v", err)
		return nil, fmt.Errorf("optimus: build bank list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/plain")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: send bank list request: %v", err)
		return nil, fmt.Errorf("optimus: send bank list request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("optimus: read bank list response: %v", err)
		return nil, fmt.Errorf("optimus: read bank list response: %w", err)
	}

	decoded, err := o.decodeOptimusEnvelope(body)
	if err != nil {
		log.Printf("optimus: decrypt bank list response failed status=%d body=%s", resp.StatusCode, body)
		return nil, fmt.Errorf("optimus: decrypt bank list response: %w", err)
	}

	var result optimusBankListResponse
	if err := json.Unmarshal(decoded, &result); err != nil {
		log.Printf("optimus: decode bank list response failed status=%d body=%s", resp.StatusCode, decoded)
		return nil, fmt.Errorf("optimus: decode bank list response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, optimusEnvelopeError("bank list", resp.StatusCode, result.ResponseMessage, result.Error)
	}

	return result.Data, nil
}

// NameEnquiry resolves an account number's holder name at the given bank,
// and - critically for interbank transfers - a SessionID that must be passed
// to the immediately-following Transfer call. Request is plain; response is
// decrypted like everywhere else.
func (o *Optimus) NameEnquiry(ctx context.Context, accountNumber, bankCode string) (*OptimusNameEnquiryData, error) {
	baseURL := strings.TrimSpace(o.WalletBaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("optimus service is not configured")
	}

	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: get token failed: %v", err)
		return nil, fmt.Errorf("optimus: get token: %w", err)
	}

	query := url.Values{}
	query.Set("BeneficiaryBankCode", bankCode)
	query.Set("BeneficiaryAccountNumber", accountNumber)
	reqURL := baseURL + optimusTransferPathPrefix + "/account/name-enquiry?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		log.Printf("optimus: build name enquiry request: %v", err)
		return nil, fmt.Errorf("optimus: build name enquiry request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/plain")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: send name enquiry request: %v", err)
		return nil, fmt.Errorf("optimus: send name enquiry request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("optimus: read name enquiry response: %v", err)
		return nil, fmt.Errorf("optimus: read name enquiry response: %w", err)
	}

	decoded, err := o.decodeOptimusEnvelope(body)
	if err != nil {
		log.Printf("optimus: decrypt name enquiry response failed status=%d body=%s", resp.StatusCode, body)
		return nil, fmt.Errorf("optimus: decrypt name enquiry response: %w", err)
	}

	var result optimusNameEnquiryResponse
	if err := json.Unmarshal(decoded, &result); err != nil {
		log.Printf("optimus: decode name enquiry response failed status=%d body=%s", resp.StatusCode, decoded)
		return nil, fmt.Errorf("optimus: decode name enquiry response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, optimusEnvelopeError("name enquiry", resp.StatusCode, result.ResponseMessage, result.Error)
	}

	return &result.Data, nil
}

// InitiateTransfer submits an intrabank or interbank transfer - the split is
// entirely in how req is built (SessionId set or empty), not the endpoint.
// Request is PGP-encrypted like GenerateWallet/validateIdentity; response is
// decrypted like everywhere else.
func (o *Optimus) InitiateTransfer(ctx context.Context, req *OptimusTransferRequest) (*OptimusTransferResponse, error) {
	baseURL := strings.TrimSpace(o.WalletBaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("optimus service is not configured")
	}

	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: get token failed: %v", err)
		return nil, fmt.Errorf("optimus: get token: %w", err)
	}

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		log.Printf("optimus: marshal transfer payload: %v", err)
		return nil, fmt.Errorf("optimus: marshal transfer payload: %w", err)
	}

	encryptedString, err := pgpEncrypt(o.PublicKey, string(payloadBytes))
	if err != nil {
		log.Printf("optimus: encrypt transfer payload: %v", err)
		return nil, fmt.Errorf("optimus: encrypt transfer payload: %w", err)
	}

	reqBody, err := json.Marshal(map[string]string{"encryptedString": encryptedString})
	if err != nil {
		log.Printf("optimus: marshal encrypted transfer body: %v", err)
		return nil, fmt.Errorf("optimus: marshal encrypted transfer body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+optimusTransferPathPrefix+"/Transaction/transfer", bytes.NewReader(reqBody))
	if err != nil {
		log.Printf("optimus: build transfer request: %v", err)
		return nil, fmt.Errorf("optimus: build transfer request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/plain")

	resp, err := o.Client.Do(httpReq)
	if err != nil {
		log.Printf("optimus: send transfer request: %v", err)
		return nil, fmt.Errorf("optimus: send transfer request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("optimus: read transfer response: %v", err)
		return nil, fmt.Errorf("optimus: read transfer response: %w", err)
	}

	decoded, err := o.decodeOptimusEnvelope(respBody)
	if err != nil {
		log.Printf("optimus: decrypt transfer response failed status=%d body=%s", resp.StatusCode, respBody)
		return nil, fmt.Errorf("optimus: decrypt transfer response: %w", err)
	}

	var result OptimusTransferResponse
	if err := json.Unmarshal(decoded, &result); err != nil {
		log.Printf("optimus: decode transfer response failed status=%d body=%s", resp.StatusCode, decoded)
		return nil, fmt.Errorf("optimus: decode transfer response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, optimusEnvelopeError("transfer", resp.StatusCode, result.ResponseMessage, result.Error)
	}

	return &result, nil
}

// optimusEnvelopeError mirrors the error-surfacing convention used throughout
// this file: Optimus's own message is user-actionable for anything below
// 500, otherwise stay generic since it's an infra-side failure.
func optimusEnvelopeError(op string, statusCode int, responseMessage string, responseError any) error {
	log.Printf("optimus: %s request failed status=%d message=%s error=%v", op, statusCode, responseMessage, responseError)
	if statusCode < 500 {
		message := strings.TrimSpace(responseMessage)
		if message == "" && responseError != nil {
			message = fmt.Sprint(responseError)
		}
		if message != "" {
			return fmt.Errorf("optimus: %s failed: %s", op, message)
		}
	}
	return fmt.Errorf("optimus: %s failed with status %d", op, statusCode)
}
