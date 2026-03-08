package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
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

		signature := ed25519.Sign(privateKey, []byte(req.Challenge))
		writeJSON(w, http.StatusOK, signResponse{
			Signature: base64.RawURLEncoding.EncodeToString(signature),
			Algorithm: "ed25519",
		})
	})

	log.Printf("dev signer listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func loadDeviceKeys() (map[string]ed25519.PrivateKey, error) {
	rawMap, err := loadRawKeyMap()
	if err != nil {
		return nil, err
	}

	keys := make(map[string]ed25519.PrivateKey, len(rawMap))
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

func decodePrivateKey(encoded string) (ed25519.PrivateKey, error) {
	decoded, err := decodeEncodedBytes(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}

	switch len(decoded) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	default:
		return nil, errors.New("private key must decode to 32 or 64 bytes")
	}
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
