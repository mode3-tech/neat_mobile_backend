package referrals

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
)

const referralCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func GenerateReferralCode(length int) (string, error) {
	code := make([]byte, length)
	alphabetLen := big.NewInt(int64(len(referralCodeAlphabet)))

	for i := range code {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			log.Printf("failed to generate referral code: %v", err)
			return "", fmt.Errorf("failed to generate referral code: %w", err)
		}
		code[i] = referralCodeAlphabet[n.Int64()]
	}

	return string(code), nil
}
