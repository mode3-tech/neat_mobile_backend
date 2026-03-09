package auth

import (
	"errors"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var dobRegex = regexp.MustCompile(`[/-]`)

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

func compareBVNAndNinDetails(bvnName, bvnDOB, ninName, ninDOB string) bool {
	return bvnName == ninName && SerializeDOB(bvnDOB) == SerializeDOB(ninDOB)
}
