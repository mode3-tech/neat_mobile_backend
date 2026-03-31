package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

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

var nonDigit = regexp.MustCompile(`\D`)

func NormalizeNigerianNumber(input string) (string, error) {
	cleaned := nonDigit.ReplaceAllString(strings.TrimSpace(input), "")
	if cleaned == "" {
		return "", errors.New("invalid Nigerian number")
	}

	switch {
	case strings.HasPrefix(cleaned, "0") && len(cleaned) == 11:
		return "234" + cleaned[1:], nil
	case strings.HasPrefix(cleaned, "234") && len(cleaned) == 13:
		return cleaned, nil
	case len(cleaned) == 10:
		return "234" + cleaned, nil
	}

	return "", errors.New("invalid Nigerian number")

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
