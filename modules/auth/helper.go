package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"neat_mobile_app_backend/internal/timeutil"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var dobRegex = regexp.MustCompile(`[/-]`)

func TestPasswordStrength(password string) bool {
	const minLength = 8
	if len(password) < minLength {
		return false
	}
	return true
}

func TitleCase(str string) string {
	caser := cases.Title(language.English)
	return caser.String(str)
}

func MaskSub(bvn string) string {
	trimmed := strings.TrimSpace(bvn)
	if len(trimmed) <= 4 {
		return trimmed
	}

	return strings.Repeat("*", len(trimmed)-4) + trimmed[len(trimmed)-4:]
}

func HashPassword(plainPassword string) (string, error) {
	passwordHashBytes, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(passwordHashBytes), nil
}

func CheckPassword(storedHash, plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(plainPassword))
	return err == nil
}

func SerializeDOB(dob string) string {
	return dobRegex.ReplaceAllString(strings.TrimSpace(dob), "")
}

func SerializeEmail(email string) string {
	trimmedEmail := strings.TrimSpace(email)
	return strings.ToLower(trimmedEmail)
}

func compareBVNAndNinDetails(bvnName, bvnDOB, ninName, ninDOB string) (bool, error) {
	if bvnName != ninName {
		return false, errors.New("bvn name does not match nin name")
	}
	if bvnDOB != ninDOB {
		return false, errors.New("bvn dob does not match nin dob")
	}
	return true, nil
}

func Generate6DigitOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))

	if err != nil {
		return "", errors.New("error generating OTP")
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}

func SplitFullName(fullName string) (string, string, string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", "", ""
	}
	firstName := parts[0]
	lastName := ""
	middleName := ""
	if len(parts) > 1 {
		lastName = parts[len(parts)-1]
		if len(parts) > 2 {
			middleName = strings.Join(parts[1:len(parts)-1], " ")
		}
	}
	return firstName, middleName, lastName
}

func UnparseDOB(dob string) string {
	if len(dob) != 8 {
		return dob
	}
	return fmt.Sprintf("%s-%s-%s", dob[0:4], dob[4:6], dob[6:8])
}

func parseBVNRecordDOB(value string) time.Time {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return time.Time{}
	}

	layouts := []string{
		"02-01-2006",
		"02/01/2006",
		"2006-01-02",
		"2006/01/02",
		"2006-01",
		"2006/01",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, clean); err == nil {
			return parsed
		}
	}

	if parsed, err := timeutil.ParseDOB(clean); err == nil {
		return parsed
	}

	return time.Time{}
}

func trimmedStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func trimmedStringValue(value *string) string {
	if value == nil {
		return ""
	}

	return strings.TrimSpace(*value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func verifyDeviceSignature(publicKeyEncoded, challenge, signatureEncoded string) (bool, error) {
	publicKeyBytes, err := decodeEncodedBytesAny(publicKeyEncoded)
	if err != nil {
		return false, errors.New("invalid public key encoding")
	}

	signatureBytes, err := decodeEncodedBytesAny(signatureEncoded)
	if err != nil {
		return false, errors.New("invalid signature enconding")
	}

	pub, err := parseP256PublicKey(publicKeyBytes)
	if err != nil {
		return false, err
	}

	digest := sha256.Sum256([]byte(challenge))

	// preferred: ASN.1 DER signature
	if ecdsa.VerifyASN1(pub, digest[:], signatureBytes) {
		return true, nil
	}

	// optional fallback: raw R||S (64 bytes)
	if len(signatureBytes) == 64 {
		r := new(big.Int).SetBytes(signatureBytes[:32])
		s := new(big.Int).SetBytes(signatureBytes[32:])
		return ecdsa.Verify(pub, digest[:], r, s), nil
	}

	return false, nil
}

func parseP256PublicKey(b []byte) (*ecdsa.PublicKey, error) {
	if block, _ := pem.Decode(b); block != nil {
		b = block.Bytes
	}

	if anyKey, err := x509.ParsePKIXPublicKey(b); err == nil {
		pub, ok := anyKey.(*ecdsa.PublicKey)
		if !ok || pub.Curve != elliptic.P256() {
			return nil, errors.New("public key is not ECDSA P-256")
		}
		return pub, nil
	}

	// uncompressed EC point: 65 bytes, starts with 0x04
	if len(b) == 65 && b[0] == 0x04 {
		x, y := elliptic.Unmarshal(elliptic.P256(), b)
		if x == nil {
			return nil, errors.New("invalid P-256 public key point")
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
	}

	return nil, errors.New("unsupported public key format")
}

func decodeEncodedBytesAny(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("empty value")
	}

	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
		hex.DecodeString,
	}

	for _, decode := range decoders {
		if b, err := decode(trimmed); err == nil && len(b) > 0 {
			return b, nil
		}
	}

	return nil, errors.New("invalid encoded value")
}

func generate6DigitOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", errors.New("error generating OTP")
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("invalid token size")
	}

	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
