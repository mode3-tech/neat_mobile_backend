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
	"net/http"
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
			return "", fmt.Errorf("pgp: marshal data: %w", err)
		}
		plaintext = string(b)
	}

	keyRing, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armoredPublicKey))
	if err != nil {
		return "", fmt.Errorf("pgp: parse public key: %w", err)
	}

	var buf bytes.Buffer
	armorWriter, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		return "", fmt.Errorf("pgp: create armor writer: %w", err)
	}

	encWriter, err := openpgp.Encrypt(armorWriter, keyRing, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("pgp: encrypt: %w", err)
	}

	if _, err := io.WriteString(encWriter, plaintext); err != nil {
		encWriter.Close()
		return "", fmt.Errorf("pgp: write plaintext: %w", err)
	}

	if err := encWriter.Close(); err != nil {
		return "", fmt.Errorf("pgp: finalize: %w", err)
	}

	// armorWriter must be closed to flush the PGP armor footer before we read buf
	if err := armorWriter.Close(); err != nil {
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
		return "", fmt.Errorf("pgp: decode base64: %w", err)
	}

	// Response is ASCII-armored PGP (-----BEGIN PGP MESSAGE-----); decode the
	// armor envelope before handing the binary payload to ReadMessage.
	armorBlock, err := armor.Decode(bytes.NewReader(ciphertextBytes))
	if err != nil {
		return "", fmt.Errorf("pgp: decode armor: %w", err)
	}

	keyRing, err := openpgp.ReadArmoredKeyRing(strings.NewReader(armoredPrivateKey))
	if err != nil {
		return "", fmt.Errorf("pgp: parse private key: %w", err)
	}

	if passphrase := []byte(os.Getenv("OPTIMUS_PASSPHRASE")); len(passphrase) > 0 {
		for _, entity := range keyRing {
			if entity.PrivateKey != nil && entity.PrivateKey.Encrypted {
				if err := entity.PrivateKey.Decrypt(passphrase); err != nil {
					return "", fmt.Errorf("pgp: decrypt private key: %w", err)
				}
			}
			for _, subkey := range entity.Subkeys {
				if subkey.PrivateKey != nil && subkey.PrivateKey.Encrypted {
					if err := subkey.PrivateKey.Decrypt(passphrase); err != nil {
						return "", fmt.Errorf("pgp: decrypt subkey: %w", err)
					}
				}
			}
		}
	}

	md, err := openpgp.ReadMessage(armorBlock.Body, keyRing, nil, nil)
	if err != nil {
		return "", fmt.Errorf("pgp: decrypt: %w", err)
	}

	plaintext, err := io.ReadAll(md.UnverifiedBody)
	if err != nil {
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
		return "", fmt.Errorf("optimus: marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
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
		return "", fmt.Errorf("optimus: decode token response: %w", err)
	}
	if result.AccessToken == "" {
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
		// Error bodies are also PGP-encrypted; attempt to decrypt before logging.
		if plainErr, err := pgpDecrypt(o.PrivateKey, strings.TrimSpace(string(respBody))); err == nil {
			log.Printf("Optimus wallet generation failed status=%d error=%s", resp.StatusCode, plainErr)
		} else {
			log.Printf("Optimus wallet generation failed status=%d body=%s", resp.StatusCode, respBody)
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

func (o *Optimus) VerifyOTPWithOptimus(ctx context.Context, phone, otpToken, email, referenceID string) error {
	token, err := o.getToken(ctx)
	if err != nil {
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
		return fmt.Errorf("optimus: encrypt otp payload: %w", err)
	}

	reqBody, err := json.Marshal(map[string]string{"encryptedString": encryptedString})
	if err != nil {
		return fmt.Errorf("optimus: marshal otp request: %w", err)
	}

	url := strings.TrimSpace(o.AuthBaseURL) + "/otp/verify"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("optimus: build otp verify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return fmt.Errorf("optimus: otp verify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if plainErr, decErr := pgpDecrypt(o.PrivateKey, strings.TrimSpace(string(respBody))); decErr == nil {
			log.Printf("optimus: otp verify failed status=%d error=%s", resp.StatusCode, plainErr)
			return fmt.Errorf("optimus: otp verification failed: %s", plainErr)
		}
		log.Printf("optimus: otp verify failed status=%d body=%s", resp.StatusCode, respBody)
		return fmt.Errorf("optimus: otp verification failed with status %d", resp.StatusCode)
	}

	return nil
}
