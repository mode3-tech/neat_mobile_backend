package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type signRequest struct {
	DeviceID  string `json:"device_id"`
	Challenge string `json:"challenge"`
}

type signResponse struct {
	Signature string `json:"signature"`
	Algorithm string `json:"algorithm"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found for dev signer (using system environment)")
	}

	addr := strings.TrimSpace(os.Getenv("DEV_SIGNER_ADDR"))
	if addr == "" {
		addr = ":9090"
	}

	keys, err := loadDeviceKeys()
	if err != nil {
		log.Fatalf("failed to load dev signer keys: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/sign", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}

		var req signRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
			return
		}

		req.DeviceID = strings.TrimSpace(req.DeviceID)
		req.Challenge = strings.TrimSpace(req.Challenge)

		if req.DeviceID == "" || req.Challenge == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "device_id and challenge are required"})
			return
		}

		privateKey, ok := keys[req.DeviceID]
		if !ok {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "device key not found"})
			return
		}

		digest := sha256.Sum256([]byte(req.Challenge))
		signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to sign challenge"})
			return
		}

		writeJSON(w, http.StatusOK, signResponse{
			Signature: base64.RawURLEncoding.EncodeToString(signature),
			Algorithm: "ecdsa-p256-sha256",
		})
	})

	log.Printf("dev signer listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func loadDeviceKeys() (map[string]*ecdsa.PrivateKey, error) {
	rawMap, err := loadRawKeyMap()
	if err != nil {
		return nil, err
	}

	keys := make(map[string]*ecdsa.PrivateKey, len(rawMap))
	for deviceID, encoded := range rawMap {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			return nil, errors.New("device id cannot be empty")
		}

		privateKey, err := decodePrivateKey(encoded)
		if err != nil {
			return nil, errors.New("invalid private key for device " + deviceID)
		}
		keys[deviceID] = privateKey
	}

	if len(keys) == 0 {
		return nil, errors.New("no device keys configured")
	}

	return keys, nil
}

func loadRawKeyMap() (map[string]string, error) {
	if value := strings.TrimSpace(os.Getenv("DEV_SIGNER_KEYS_JSON")); value != "" {
		return parseKeyMap(value)
	}

	if path := strings.TrimSpace(os.Getenv("DEV_SIGNER_KEYS_FILE")); path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return parseKeyMap(string(content))
	}

	deviceID := strings.TrimSpace(os.Getenv("DEV_SIGNER_DEVICE_ID"))
	privateKey := strings.TrimSpace(os.Getenv("DEV_SIGNER_PRIVATE_KEY"))
	if deviceID != "" && privateKey != "" {
		return map[string]string{deviceID: privateKey}, nil
	}

	return nil, errors.New("set DEV_SIGNER_KEYS_JSON or DEV_SIGNER_KEYS_FILE (or DEV_SIGNER_DEVICE_ID + DEV_SIGNER_PRIVATE_KEY)")
}

func parseKeyMap(input string) (map[string]string, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func decodePrivateKey(encoded string) (*ecdsa.PrivateKey, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, errors.New("empty private key")
	}

	// Allow PEM directly in JSON/file values.
	if key, err := parseP256PrivateKey([]byte(trimmed)); err == nil {
		return key, nil
	}

	decoded, err := decodeEncodedBytes(trimmed)
	if err != nil {
		return nil, err
	}

	return parseP256PrivateKey(decoded)
}

func parseP256PrivateKey(b []byte) (*ecdsa.PrivateKey, error) {
	if block, _ := pem.Decode(b); block != nil {
		b = block.Bytes
	}

	if key, err := x509.ParseECPrivateKey(b); err == nil {
		if key.Curve != elliptic.P256() {
			return nil, errors.New("private key is not ECDSA P-256")
		}
		return key, nil
	}

	if anyKey, err := x509.ParsePKCS8PrivateKey(b); err == nil {
		key, ok := anyKey.(*ecdsa.PrivateKey)
		if !ok || key.Curve != elliptic.P256() {
			return nil, errors.New("private key is not ECDSA P-256")
		}
		return key, nil
	}

	// Raw scalar support for compact env values.
	if len(b) == 32 {
		curve := elliptic.P256()
		d := new(big.Int).SetBytes(b)
		if d.Sign() <= 0 || d.Cmp(curve.Params().N) >= 0 {
			return nil, errors.New("private key scalar is out of range")
		}

		x, y := curve.ScalarBaseMult(b)
		if x == nil {
			return nil, errors.New("invalid P-256 private key scalar")
		}

		return &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: curve,
				X:     x,
				Y:     y,
			},
			D: d,
		}, nil
	}

	return nil, errors.New("unsupported private key format")
}

func decodeEncodedBytes(value string) ([]byte, error) {
	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
		hex.DecodeString,
	}

	for _, decode := range decoders {
		decoded, err := decode(value)
		if err != nil {
			continue
		}
		if len(decoded) > 0 {
			return decoded, nil
		}
	}

	return nil, errors.New("invalid encoded value")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
