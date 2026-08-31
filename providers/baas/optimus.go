package baas

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/auth/registerv2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"os"

	"golang.org/x/crypto/openpgp"       //nolint:staticcheck
	"golang.org/x/crypto/openpgp/armor" //nolint:staticcheck
)

type Optimus struct {
	WalletBaseURL string
	AuthBaseURL   string
	Username      string
	Password      string
	PublicKey     string
	PrivateKey    string
	Client        *http.Client
	mu            sync.Mutex
	cachedToken   string
	tokenExpiry   time.Time
}

func NewOptimus(walletBaseURL, authBaseURL, username, password, publicKey, privateKey string) *Optimus {
	return &Optimus{
		WalletBaseURL: walletBaseURL,
		AuthBaseURL:   authBaseURL,
		Username:      username,
		Password:      password,
		PublicKey:     publicKey,
		PrivateKey:    privateKey,
		Client:        &http.Client{Timeout: time.Second * 15},
	}
}

// pgpEncrypt serializes data to JSON (unless it is already a string), encrypts
// the result with an ASCII-armored PGP public key, and returns the ciphertext
// as a base64-encoded string.
func pgpEncrypt(armoredPublicKey string, data any) (string, error) {
	var plaintext string
	switch v := data.(type) {
	case string:
		plaintext = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			log.Printf("pgp: marshal data failed: %v", err)
			return "", fmt.Errorf("pgp: marshal data: %w", err)
		}
		plaintext = string(b)
	}

	keyRing, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armoredPublicKey))
	if err != nil {
		log.Printf("pgp: parse public key failed: %v", err)
		return "", fmt.Errorf("pgp: parse public key: %w", err)
	}

	var buf bytes.Buffer
	armorWriter, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		log.Printf("pgp: create armor writer failed: %v", err)
		return "", fmt.Errorf("pgp: create armor writer: %w", err)
	}

	encWriter, err := openpgp.Encrypt(armorWriter, keyRing, nil, nil, nil)
	if err != nil {
		log.Printf("pgp: encrypt failed: %v", err)
		return "", fmt.Errorf("pgp: encrypt: %w", err)
	}

	if _, err := io.WriteString(encWriter, plaintext); err != nil {
		encWriter.Close()
		log.Printf("pgp: write plaintext failed: %v", err)
		return "", fmt.Errorf("pgp: write plaintext: %w", err)
	}

	if err := encWriter.Close(); err != nil {
		log.Printf("pgp: finalize failed: %v", err)
		return "", fmt.Errorf("pgp: finalize: %w", err)
	}

	// armorWriter must be closed to flush the PGP armor footer before we read buf
	if err := armorWriter.Close(); err != nil {
		log.Printf("pgp: close armor writer failed: %v", err)
		return "", fmt.Errorf("pgp: close armor writer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// pgpDecrypt base64-decodes ciphertextB64 then decrypts it using an
// ASCII-armored PGP private key. If the private key is passphrase-protected,
// the passphrase is read from the OPTIMUS_PASSPHRASE environment variable.
// If the decrypted plaintext is a JSON-encoded string (e.g. `"value"`), the
// outer quotes are stripped before returning — matching the behaviour of the
// upstream C# implementation.
func pgpDecrypt(armoredPrivateKey, ciphertextB64 string) (string, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		log.Printf("pgp: decode base64 failed: %v", err)
		return "", fmt.Errorf("pgp: decode base64: %w", err)
	}

	// Response is ASCII-armored PGP (-----BEGIN PGP MESSAGE-----); decode the
	// armor envelope before handing the binary payload to ReadMessage.
	armorBlock, err := armor.Decode(bytes.NewReader(ciphertextBytes))
	if err != nil {
		log.Printf("pgp: decode armor failed: %v", err)
		return "", fmt.Errorf("pgp: decode armor: %w", err)
	}

	keyRing, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armoredPrivateKey))
	if err != nil {
		log.Printf("pgp: parse private key failed: %v", err)
		return "", fmt.Errorf("pgp: parse private key: %w", err)
	}

	if passphrase := []byte(os.Getenv("OPTIMUS_PASSPHRASE")); len(passphrase) > 0 {
		for _, entity := range keyRing {
			if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
				if err := entity.PrivateKey.Decrypt(passphrase); err != nil {
					log.Printf("pgp: decrypt private key failed: %v", err)
					return "", fmt.Errorf("pgp: decrypt private key: %w", err)
				}
			}
			for _, subkey := range entity.Subkeys {
				if subkey.PrivateKey != nil && subkey.PrivateKey.Encrypted {
					if err := subkey.PrivateKey.Decrypt(passphrase); err != nil {
						log.Printf("pgp: decrypt subkey failed: %v", err)
						return "", fmt.Errorf("pgp: decrypt subkey: %w", err)
					}
				}
			}
		}
	}

	md, err := openpgp.ReadMessage(armorBlock.Body, keyRing, nil, nil)
	if err != nil {
		log.Printf("pgp: decrypt failed: %v", err)
		return "", fmt.Errorf("pgp: decrypt: %w", err)
	}

	plaintext, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
		log.Printf("pgp: read plaintext failed: %v", err)
		return "", fmt.Errorf("pgp: read plaintext: %w", err)
	}

	// Unwrap JSON-encoded strings: `"hello"` → `hello`
	result := string(plaintext)
	var unwrapped string
	if err := json.Unmarshal([]byte(result), &unwrapped); err == nil {
		return unwrapped, nil
	}
	return result, nil
}

// decodeOptimusEnvelope unwraps an {"encryptedString": "..."} PGP envelope if
// present, decrypting it with the configured private key; otherwise it returns
// body unchanged. Callers should apply this unconditionally, before branching
// on HTTP status — Optimus PGP-encrypts both success and error response bodies.
func (o *Optimus) decodeOptimusEnvelope(body []byte) ([]byte, error) {
	// Shape 1: {"encryptedString": "<base64>"}
	var envelope struct {
		EncryptedString string `json:"encryptedString"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && strings.TrimSpace(envelope.EncryptedString) != "" {
		plaintext, err := pgpDecrypt(o.PrivateKey, envelope.EncryptedString)
		if err != nil {
			log.Printf("optimus: decrypt response (encryptedString envelope) failed: %v", err)
			return nil, fmt.Errorf("optimus: decrypt response: %w", err)
		}
		return []byte(plaintext), nil
	}

	// Shape 2: "<base64>" — the whole body is a JSON string literal wrapping the
	// ciphertext (quotes are part of the actual wire bytes, confirmed from raw
	// response logs — json.Unmarshal strips the quotes and un-escapes properly).
	var bare string
	if err := json.Unmarshal(body, &bare); err == nil && strings.TrimSpace(bare) != "" {
		plaintext, err := pgpDecrypt(o.PrivateKey, bare)
		if err != nil {
			log.Printf("optimus: decrypt response (quoted body) failed: %v", err)
			return nil, fmt.Errorf("optimus: decrypt response: %w", err)
		}
		return []byte(plaintext), nil
	}

	// Shape 3: the entire body IS the base64 ciphertext, with no JSON encoding at
	// all (not even quotes) — kept as a fallback in case some endpoint sends it
	// truly bare.
	trimmed := strings.TrimSpace(string(body))
	if trimmed != "" && !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		plaintext, err := pgpDecrypt(o.PrivateKey, trimmed)
		if err != nil {
			log.Printf("optimus: decrypt response (raw body) failed: %v", err)
			return nil, fmt.Errorf("optimus: decrypt response: %w", err)
		}
		return []byte(plaintext), nil
	}

	// Shape 4: plain, unencrypted JSON object — passthrough.
	return body, nil
}

// getToken returns a valid access token, generating a new one if the cached
// token is absent or within 60 seconds of expiry.
func (o *Optimus) getToken(ctx context.Context) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cachedToken != "" && time.Now().Add(60*time.Second).Before(o.tokenExpiry) {
		return o.cachedToken, nil
	}

	url := strings.TrimSpace(o.AuthBaseURL) + "/tokens/generate"
	body, err := json.Marshal(optimusTokenRequest{
		Username: strings.TrimSpace(o.Username),
		Password: strings.TrimSpace(o.Password),
	})
	if err != nil {
		log.Printf("optimus: marshal token request failed: %v", err)
		return "", fmt.Errorf("optimus: marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("optimus: build token request failed: %v", err)
		return "", fmt.Errorf("optimus: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: token generation request failed: %v", err)
		return "", fmt.Errorf("optimus: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("optimus: token generation failed status=%d body=%s", resp.StatusCode, respBody)
		return "", fmt.Errorf("optimus: token generation failed with status %d", resp.StatusCode)
	}

	var result optimusTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("optimus: decode token response failed: %v", err)
		return "", fmt.Errorf("optimus: decode token response: %w", err)
	}
	if result.AccessToken == "" {
		log.Printf("optimus: token response contained empty accessToken")
		return "", fmt.Errorf("optimus: token response contained empty accessToken")
	}

	o.cachedToken = result.AccessToken
	// Tokens appear to live ~30 minutes; treat them as valid for 25 minutes so
	// we refresh before actual expiry without needing to parse the JWT.
	o.tokenExpiry = time.Now().Add(25 * time.Minute)
	log.Printf("optimus: access token refreshed, valid until %s", o.tokenExpiry.Format(time.RFC3339))

	return o.cachedToken, nil
}

func (o *Optimus) GenerateWallet(ctx context.Context, walletInfo *auth.WalletPayload) (*auth.WalletResponse, error) {
	baseURL := strings.TrimSpace(o.WalletBaseURL)
	if baseURL == "" || strings.TrimSpace(o.Username) == "" {
		log.Printf("Optimus is not configured")
		return nil, fmt.Errorf("Optimus is not configured")
	}

	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("Optimus: failed to obtain access token: %v", err)
		return nil, err
	}

	// TODO: confirm with Optimus API docs the exact plaintext to encrypt
	// (likely the BVN, but could be a JSON blob of multiple fields).

	url := baseURL + "/Customer/create-by-bvn"
	log.Printf("Optimus URL for creating customer account with BVN: %s", url)
	payload := OptimusPayload{
		RequestId:         walletInfo.RequestID,
		Email:             walletInfo.Email,
		Gender:            walletInfo.Gender,
		MaritalStatus:     walletInfo.MaritalStatus,
		MothersMaidenName: walletInfo.MothersMaidenName,
		Address:           walletInfo.Address,
		HouseNo:           walletInfo.HouseNo,
		ProductId:         walletInfo.ProductId,
		PhoneNumber:       walletInfo.PhoneNumber,
		BVN:               walletInfo.BVN,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to encode payload with json.Marshal: %s", err)
		return nil, fmt.Errorf("Failed to encode payload with json.Marshal: %s", err)
	}

	payloadString := string(body)
	log.Printf("Optimus wallet generation request payload: %s", payloadString)

	encryptedString, err := pgpEncrypt(o.PublicKey, payloadString)
	if err != nil {
		log.Printf("Optimus: failed to encrypt payload: %v", err)
		return nil, fmt.Errorf("optimus: encrypt payload: %w", err)
	}

	reqBody, err := json.Marshal(map[string]string{"encryptedString": encryptedString})
	if err != nil {
		log.Printf("Optimus: failed to marshal encrypted body: %s", err)
		return nil, fmt.Errorf("optimus: marshal encrypted body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Failed to create new request with context: %s", err)
		return nil, fmt.Errorf("Failed to create new request with context: %s", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Accept", "text/plain")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("Optimus wallet generation request failed: %v", err)
		return nil, fmt.Errorf("Optimus wallet request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

		var result registerv2.OptimusResponse
		if decoded, decErr := o.decodeOptimusEnvelope(respBody); decErr == nil {
			if jsonErr := json.Unmarshal(decoded, &result); jsonErr != nil {
				log.Printf("Optimus wallet generation failed status=%d body=%s", resp.StatusCode, decoded)
			}
		} else {
			log.Printf("Optimus wallet generation decrypt failed status=%d body=%s", resp.StatusCode, respBody)
		}
		log.Printf("Optimus wallet generation failed status=%d response=%+v", resp.StatusCode, result)

		// Surface Optimus's own message for anything below 500 - it's
		// user-actionable (e.g. "No record found for customer"). 5xx responses
		// stay generic since they're infra-side failures, not user-fixable.
		if resp.StatusCode < 500 {
			message := strings.TrimSpace(result.ResponseMessage)
			if message == "" && result.Error != nil {
				message = fmt.Sprint(result.Error)
			}
			if message != "" {
				return nil, fmt.Errorf("%s", message)
			}
		}
		return nil, fmt.Errorf("Optimus wallet generation failed with status: %d", resp.StatusCode)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Optimus wallet generation: failed to read response body: %v", err)
		return nil, fmt.Errorf("Optimus wallet generation: failed to read response body: %w", err)
	}

	var encryptedResp struct {
		EncryptedString string `json:"encryptedString"`
	}
	if err := json.Unmarshal(respBytes, &encryptedResp); err != nil || encryptedResp.EncryptedString == "" {
		log.Printf("Optimus wallet generation: response is not an encryptedString envelope, body=%s", respBytes)
		return nil, fmt.Errorf("optimus: unexpected response format (expected encryptedString envelope)")
	}

	plaintext, err := pgpDecrypt(o.PrivateKey, encryptedResp.EncryptedString)
	if err != nil {
		log.Printf("Optimus wallet generation: failed to decrypt response: %v", err)
		return nil, fmt.Errorf("optimus: decrypt response: %w", err)
	}

	var result auth.WalletResponse
	if err := json.Unmarshal([]byte(plaintext), &result); err != nil {
		log.Printf("Optimus wallet generation: failed to decode decrypted response body=%s err=%v", plaintext, err)
		return nil, fmt.Errorf("Failed to decode Optimus wallet generation response: %w", err)
	}

	sub := result.Data.Data
	result.Customer = &auth.WalletCustomer{
		ID:       sub.CustomerID,
		Currency: "NGN",
	}
	result.Wallet = &auth.WalletInfo{
		AccountNumber: sub.NUBAN,
		AccountName:   sub.AccountName,
		BankCode:      "000036",
		BankName:      "OPTIMUS BANK",
		WalletId:      sub.CustomerID,
	}

	return &result, nil
}

func (o *Optimus) LookupWalletByCustomerID(ctx context.Context, customerID string) (*auth.WalletResponse, bool, error) {
	return nil, true, nil
}

// LookupCustomerByPhone is not needed on the Optimus path - Optimus already
// correlates wallet creation via our own RequestID, so there's no gap to
// recover from the way there is on Providus. Always reports not-found so
// callers fall through to their original error instead of mistakenly
// treating this as a successful recovery.
func (o *Optimus) LookupCustomerByPhone(ctx context.Context, phoneNumber string) (string, bool, error) {
	return "", false, nil
}

// ValidateBVN validates a BVN through the Optimus wallet API. Optimus wraps
// both successful and failed responses in its own response envelope, including
// when the HTTP status itself is not 2xx.
func (o *Optimus) ValidateBVN(ctx context.Context, payload registerv2.OptimusBVNValidationRequest) (*registerv2.OptimusResponse, error) {
	return o.validateIdentity(ctx, "/Customer/validate-bvn", payload)
}

// ValidateNIN validates a NIN through the Optimus wallet API.
func (o *Optimus) ValidateNIN(ctx context.Context, payload registerv2.OptimusNINValidationRequest) (*registerv2.OptimusResponse, error) {
	return o.validateIdentity(ctx, "/Customer/validate-nin", payload)
}

func (o *Optimus) validateIdentity(ctx context.Context, path string, payload any) (*registerv2.OptimusResponse, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(o.WalletBaseURL), "/")
	if baseURL == "" || strings.TrimSpace(o.Username) == "" {
		log.Printf("optimus: validation service is not configured")
		return nil, fmt.Errorf("optimus validation service is not configured")
	}

	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: failed to obtain access token for validation: %v", err)
		return nil, err
	}

	encryptedString, err := pgpEncrypt(o.PublicKey, payload)
	if err != nil {
		log.Printf("optimus: encrypt validation payload failed: %v", err)
		return nil, fmt.Errorf("optimus: encrypt validation payload: %w", err)
	}
	reqBody, err := json.Marshal(map[string]string{"encryptedString": encryptedString})
	if err != nil {
		log.Printf("optimus: marshal validation request failed: %v", err)
		return nil, fmt.Errorf("optimus: marshal validation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(reqBody))
	if err != nil {
		log.Printf("optimus: build validation request failed: %v", err)
		return nil, fmt.Errorf("optimus: build validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: validation request failed: %v", err)
		return nil, fmt.Errorf("optimus: validation request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		log.Printf("optimus: read validation response failed: %v", err)
		return nil, fmt.Errorf("optimus: read validation response: %w", err)
	}

	result, decodeErr := o.decodeValidationResponse(body)
	if decodeErr != nil {
		log.Printf("optimus: raw response = %s", string(body))
		log.Printf("optimus: validation response decode failed status=%d err=%v", resp.StatusCode, decodeErr)
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("we could not verify your details right now. Please try again")
		}
		return nil, decodeErr
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		log.Printf("optimus: validation request failed path=%s status=%d response=%+v", path, resp.StatusCode, result)

		// Surface Optimus's own message for anything below 500 - it's
		// user-actionable. 5xx responses stay generic since they're infra-side
		// failures, not user-fixable.
		if resp.StatusCode < 500 {
			message := strings.TrimSpace(result.ResponseMessage)
			if message == "" && result.Error != nil {
				message = fmt.Sprint(result.Error)
			}
			if message != "" {
				return nil, fmt.Errorf("%s", message)
			}
		}
		return nil, fmt.Errorf("we could not verify your details. Please check them and try again")
	}
	return result, nil
}

func (o *Optimus) decodeValidationResponse(body []byte) (*registerv2.OptimusResponse, error) {
	decoded, err := o.decodeOptimusEnvelope(body)
	if err != nil {
		log.Printf("optimus: decrypt validation response failed: %v", err)
		return nil, fmt.Errorf("optimus: decrypt validation response: %w", err)
	}
	body = decoded

	var result registerv2.OptimusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("optimus: decode validation response failed: %v", err)
		return nil, fmt.Errorf("optimus: decode validation response: %w", err)
	}
	if strings.TrimSpace(result.ResponseMessage) == "" && result.Error == nil && strings.TrimSpace(result.ResponseCode) == "" {
		log.Printf("optimus: validation response did not contain an Optimus response envelope, body=%s", body)
		return nil, fmt.Errorf("optimus: validation response did not contain an Optimus response envelope")
	}
	return &result, nil
}

func (o *Optimus) VerifyOTPWithOptimus(ctx context.Context, phone, otpToken, email, referenceID string) error {
	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: failed to obtain access token for otp verify: %v", err)
		return err
	}

	payload := optimusVerifyOTPRequest{
		PhoneNumber: phone,
		OTPToken:    otpToken,
		Email:       email,
		ReferenceID: referenceID,
	}

	encryptedString, err := pgpEncrypt(o.PublicKey, payload)
	if err != nil {
		log.Printf("optimus: encrypt otp payload failed: %v", err)
		return fmt.Errorf("optimus: encrypt otp payload: %w", err)
	}

	reqBody, err := json.Marshal(map[string]string{"encryptedString": encryptedString})
	if err != nil {
		log.Printf("optimus: marshal otp request failed: %v", err)
		return fmt.Errorf("optimus: marshal otp request: %w", err)
	}

	// /Customer/validate-otp lives alongside /Customer/validate-bvn and
	// /Customer/validate-nin (see validateIdentity) - same WalletBaseURL, not
	// AuthBaseURL. WalletBaseURL already ends in the API version segment
	// (.../opti-finserve-api/v1), so no extra "/api/v1" prefix here.
	url := strings.TrimRight(strings.TrimSpace(o.WalletBaseURL), "/") + "/Customer/validate-otp"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("optimus: build otp verify request failed: %v", err)
		return fmt.Errorf("optimus: build otp verify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: otp verify request failed: %v", err)
		return fmt.Errorf("optimus: otp verify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		var result registerv2.OptimusResponse
		if decoded, decErr := o.decodeOptimusEnvelope(respBody); decErr == nil {
			_ = json.Unmarshal(decoded, &result)
		}
		log.Printf("optimus: otp verify failed status=%d body=%s response=%+v", resp.StatusCode, respBody, result)

		// Surface Optimus's own message for anything below 500 - it's
		// user-actionable. 5xx responses stay generic since they're infra-side
		// failures, not user-fixable.
		if resp.StatusCode < 500 {
			message := strings.TrimSpace(result.ResponseMessage)
			if message == "" && result.Error != nil {
				message = fmt.Sprint(result.Error)
			}
			if message != "" {
				return fmt.Errorf("optimus: otp verification failed: %s", message)
			}
		}
		return fmt.Errorf("optimus: otp verification failed with status %d", resp.StatusCode)
	}

	return nil
}

// ResendOTPWithOptimus asks Optimus to resend the OTP tied to referenceID. The
// reference id alone is enough for Optimus to identify which OTP challenge to
// resend, regardless of what originally triggered it (BVN/NIN validation etc).
// Optimus issues a new reference id for the resent OTP, which is returned so
// the caller can use it for the next verify/resend instead of the old one.
func (o *Optimus) ResendOTPWithOptimus(ctx context.Context, referenceID string) (string, error) {
	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: failed to obtain access token for otp resend: %v", err)
		return "", err
	}

	// Mirrors /Customer/validate-otp (see VerifyOTPWithOptimus): WalletBaseURL
	// already ends in the API version segment (.../opti-finserve-api/v1), so no
	// extra "/api/v1" prefix here - that was the same wrong assumption that
	// caused validate-otp to 404 until it was moved off AuthBaseURL.
	query := url.Values{}
	query.Set("referenceId", strings.TrimSpace(referenceID))
	requestURL := strings.TrimRight(strings.TrimSpace(o.WalletBaseURL), "/") + "/Otp/resend?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		log.Printf("optimus: build otp resend request failed: %v", err)
		return "", fmt.Errorf("optimus: build otp resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: otp resend request failed: %v", err)
		return "", fmt.Errorf("optimus: otp resend request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		log.Printf("optimus: read otp resend response failed: %v", err)
		return "", fmt.Errorf("optimus: read otp resend response: %w", err)
	}

	decoded, decodeErr := o.decodeOptimusEnvelope(body)
	if decodeErr != nil {
		log.Printf("optimus: decrypt otp resend response failed, falling back to raw body: %v", decodeErr)
		decoded = body
	}

	var result registerv2.OptimusResponse
	if err := json.Unmarshal(decoded, &result); err != nil {
		log.Printf("optimus: decode otp resend response failed status=%d body=%s", resp.StatusCode, decoded)
	}
	log.Printf("optimus: otp resend decoded response status=%d response=%+v", resp.StatusCode, result)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Surface Optimus's own message for anything below 500 - it's
		// user-actionable. 5xx responses stay generic since they're infra-side
		// failures, not user-fixable.
		if resp.StatusCode < 500 {
			message := strings.TrimSpace(result.ResponseMessage)
			if message == "" && result.Error != nil {
				message = fmt.Sprint(result.Error)
			}
			if message != "" {
				return "", fmt.Errorf("%s", message)
			}
		}
		return "", fmt.Errorf("otp resend failed with status %d", resp.StatusCode)
	}

	newReferenceID := strings.TrimSpace(result.Data)
	log.Printf("optimus: otp resend succeeded oldReferenceId=%s newReferenceId=%s", referenceID, newReferenceID)
	return newReferenceID, nil
}

func (o *Optimus) ValidateAccount(ctx context.Context, accountNumber string) error {
	url := strings.TrimSpace(o.WalletBaseURL) + "/kyc-api/api/v1/AccountRequest/account"
	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: get token failed: %v", err)
		return fmt.Errorf("optimus: get token: %w", err)
	}

	body, err := json.Marshal(map[string]string{"accountNumber": accountNumber})
	if err != nil {
		log.Printf("optimus: marshal account request: %v", err)
		return fmt.Errorf("optimus: marshal account request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("optimus: build account request: %v", err)
		return fmt.Errorf("optimus: build account request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		log.Printf("optimus: send account request: %v", err)
		return fmt.Errorf("optimus: send account request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		var result registerv2.OptimusResponse
		if decoded, decErr := o.decodeOptimusEnvelope(respBody); decErr == nil {
			_ = json.Unmarshal(decoded, &result)
		}
		log.Printf("optimus: account validation failed status=%d body=%s response=%+v", resp.StatusCode, respBody, result)

		// Surface Optimus's own message for anything below 500 - it's
		// user-actionable. 5xx responses stay generic since they're infra-side
		// failures, not user-fixable.
		if resp.StatusCode < 500 {
			message := strings.TrimSpace(result.ResponseMessage)
			if message == "" && result.Error != nil {
				message = fmt.Sprint(result.Error)
			}
			if message != "" {
				return fmt.Errorf("optimus: account validation failed: %s", message)
			}
		}
		return fmt.Errorf("optimus: account validation failed with status %d", resp.StatusCode)
	}

	return nil
}

func (o *Optimus) UpgradeAccount(ctx context.Context, payload *OptimusAccountUpgradeRequest) (*registerv2.OptimusResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("optimus: marshal account upgrade request: %v", err)
		return nil, fmt.Errorf("optimus: marshal account upgrade request: %w", err)
	}

	url := strings.TrimSpace(o.WalletBaseURL) + "/kyc-api/api/v1/AccountRequest/account"
	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: get token failed: %v", err)
		return nil, fmt.Errorf("optimus: get token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("optimus: build account upgrade request: %v", err)
		return nil, fmt.Errorf("optimus: build account upgrade request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("optimus: send account upgrade request: %v", err)
		return nil, fmt.Errorf("optimus: send account upgrade request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("optimus: read account upgrade response failed: %v", err)
		return nil, fmt.Errorf("optimus: read account upgrade response: %w", err)
	}

	decoded, err := o.decodeOptimusEnvelope(respBody)
	if err != nil {
		log.Printf("optimus: decrypt account upgrade response failed status=%d body=%s", resp.StatusCode, respBody)
		return nil, fmt.Errorf("optimus: decrypt account upgrade response: %w", err)
	}

	var result registerv2.OptimusResponse
	if err := json.Unmarshal(decoded, &result); err != nil {
		log.Printf("optimus: decode account upgrade response failed status=%d body=%s", resp.StatusCode, decoded)
		return nil, fmt.Errorf("optimus: decode account upgrade response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("optimus: account upgrade request failed status=%d body=%s", resp.StatusCode, decoded)

		// Surface Optimus's own message for anything below 500 - it's
		// user-actionable. 5xx responses stay generic since they're infra-side
		// failures, not user-fixable.
		if resp.StatusCode < 500 {
			message := strings.TrimSpace(result.ResponseMessage)
			if message == "" && result.Error != nil {
				message = fmt.Sprint(result.Error)
			}
			if message != "" {
				return nil, fmt.Errorf("optimus: %s", message)
			}
		}
		return nil, fmt.Errorf("account upgrade request failed with status %d", resp.StatusCode)
	}

	log.Printf("optimus: account upgrade request succeeded")
	return &result, nil
}

func (o *Optimus) CheckAccountUpgradeStatus(ctx context.Context, accountNumber string) (*registerv2.OptimusResponse, error) {
	url := strings.TrimSpace(o.WalletBaseURL) + "/kyc-api/api/v1/AccountRequest/account/" + accountNumber
	token, err := o.getToken(ctx)
	if err != nil {
		log.Printf("optimus: get token failed: %v", err)
		return nil, fmt.Errorf("optimus: get token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("optimus: build account upgrade status request: %v", err)
		return nil, fmt.Errorf("optimus: build account upgrade status request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("optimus: send account upgrade status request: %v", err)
		return nil, fmt.Errorf("optimus: send account upgrade status request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("optimus: read account upgrade status response failed: %v", err)
		return nil, fmt.Errorf("optimus: read account upgrade status response: %w", err)
	}

	decoded, err := o.decodeOptimusEnvelope(body)
	if err != nil {
		log.Printf("optimus: decrypt account upgrade status response failed status=%d body=%s", resp.StatusCode, body)
		return nil, fmt.Errorf("optimus: decrypt account upgrade status response: %w", err)
	}

	var result registerv2.OptimusResponse
	if err := json.Unmarshal(decoded, &result); err != nil {
		log.Printf("optimus: decode account upgrade status response failed status=%d body=%s", resp.StatusCode, decoded)
		return nil, fmt.Errorf("optimus: decode account upgrade status response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("optimus: account upgrade status request failed status=%d body=%s", resp.StatusCode, decoded)

		// Surface Optimus's own message for anything below 500 - it's
		// user-actionable. 5xx responses stay generic since they're infra-side
		// failures, not user-fixable.
		if resp.StatusCode < 500 {
			message := strings.TrimSpace(result.ResponseMessage)
			if message == "" && result.Error != nil {
				message = fmt.Sprint(result.Error)
			}
			if message != "" {
				return nil, fmt.Errorf("optimus: %s", message)
			}
		}
		return nil, fmt.Errorf("account upgrade status request failed with status %d", resp.StatusCode)
	}

	log.Printf("optimus: account upgrade status request succeeded")
	return &result, nil
}
