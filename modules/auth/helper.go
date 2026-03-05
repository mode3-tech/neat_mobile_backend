package auth

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

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

func CheckPassword(storedHash, plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(plainPassword))
	return err == nil
}
