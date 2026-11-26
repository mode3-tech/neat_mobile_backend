// Command seed-wema-users saves Wema (ALAT) accounts that were opened outside
// the app (e.g. from Postman) into our database, so their owners can log in.
//
// It writes what /register would have written for a Wema customer: a
// wallet_users row (BVN/NIN encrypted, password and PIN bcrypt-hashed) and a
// wallet_customer_wallets row pointing at the Wema account. It does not call
// Wema. Run it against the database the server uses (DB_URL and
// BVN_NIN_ENCRYPTION_KEY come from the environment / .env). It does not run
// schema migrations: start the server once against that database first if
// the tables may be missing. -dry-run only reads (duplicate checks).
//
// Usage:
//
//	go run ./cmd/seed-wema-users -file accounts.json [-dry-run]
//
// accounts.json is a list:
//
//	[
//	  {
//	    "phone": "08012345678",
//	    "email": "jane@example.com",
//	    "first_name": "Jane",
//	    "last_name": "Doe",
//	    "dob": "1990-01-31",
//	    "bvn": "22222222222",
//	    "nin": "33333333333",
//	    "address": "12 Broad Street, Lagos",
//	    "password": "a-password-to-log-in-with",
//	    "pin": "1234",
//	    "account_number": "0123456789",
//	    "account_name": "JANE DOE"        // optional, defaults to first + last name
//	  }
//	]
//
// An account whose phone, email, BVN or NIN already exists is skipped, so the
// command can be re-run. Afterwards run `go run ./cmd/seed-referral-codes` to
// give the new users referral codes. The file holds passwords, PINs and
// BVN/NIN: keep it out of git and delete it when done.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"neat_mobile_app_backend/internal/config"
	"neat_mobile_app_backend/internal/crypto"
	"neat_mobile_app_backend/internal/database"
	"neat_mobile_app_backend/internal/modules/auth"
	"neat_mobile_app_backend/internal/modules/wallet"
	phoneutil "neat_mobile_app_backend/internal/phone"
	"neat_mobile_app_backend/internal/types"
	"neat_mobile_app_backend/models"
)

type account struct {
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	DOB           string `json:"dob"`
	BVN           string `json:"bvn"`
	NIN           string `json:"nin"`
	Address       string `json:"address"`
	Password      string `json:"password"`
	PIN           string `json:"pin"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

// Wema's bank details, as registration records them.
const (
	wemaBankCode = "000017"
	wemaBankName = "WEMA BANK"
)

func main() {
	file := flag.String("file", "", "path to the JSON list of accounts")
	dryRun := flag.Bool("dry-run", false, "validate and report, write nothing")
	flag.Parse()
	_ = godotenv.Load()

	if strings.TrimSpace(*file) == "" {
		log.Fatal("-file is required (see the usage in cmd/seed-wema-users/main.go)")
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read %s: %v", *file, err)
	}
	var accounts []account
	if err := json.Unmarshal(raw, &accounts); err != nil {
		log.Fatalf("parse %s: %v", *file, err)
	}
	if len(accounts) == 0 {
		log.Fatal("no accounts in the file")
	}

	cfg := config.Load()
	if strings.TrimSpace(cfg.DBUrl) == "" {
		log.Fatal("DB_URL is required")
	}
	cipher, err := crypto.NewFieldCipherFromBase64(cfg.BVNNINEncryptionKey)
	if err != nil {
		log.Fatalf("BVN_NIN_ENCRYPTION_KEY: %v", err)
	}
	db, err := database.NewPostgres(cfg.DBUrl)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	// No database.Migrate here: the server migrates the schema when it starts,
	// and running it from this command would change the database even on a
	// -dry-run (and take minutes over a remote connection).
	log.Printf("connected; dry_run=%t (a dry run only reads, to check for duplicates)", *dryRun)

	ctx := context.Background()
	var created, skipped, failed int
	for i, a := range accounts {
		label := fmt.Sprintf("#%d %s", i+1, maskPhone(a.Phone))
		user, w, err := build(a)
		if err != nil {
			log.Printf("%s: invalid, not saved: %v", label, err)
			failed++
			continue
		}
		exists, err := alreadyExists(ctx, db, user, w)
		if err != nil {
			log.Printf("%s: duplicate check failed: %v", label, err)
			failed++
			continue
		}
		if exists != "" {
			log.Printf("%s: skipped, %s already exists", label, exists)
			skipped++
			continue
		}
		if *dryRun {
			log.Printf("%s: ok (dry run) - would create user %s %s, wema account %s", label, user.FirstName, user.LastName, w.AccountNumber)
			created++
			continue
		}
		if err := save(ctx, db, cipher, user, w); err != nil {
			log.Printf("%s: save failed (nothing written for it): %v", label, err)
			failed++
			continue
		}
		log.Printf("%s: created user_id=%s wema_account=%s", label, user.ID, w.AccountNumber)
		created++
	}
	log.Printf("done: created=%d skipped=%d failed=%d dry_run=%t", created, skipped, failed, *dryRun)
	if failed > 0 {
		os.Exit(1)
	}
}

// build validates one entry and turns it into the user and wallet rows. User
// BVN/NIN stay plaintext here; save() encrypts them.
func build(a account) (*models.User, *wallet.CustomerWallet, error) {
	phone, err := phoneutil.NormalizeNigerianNumber(a.Phone)
	if err != nil {
		return nil, nil, fmt.Errorf("phone: %w", err)
	}
	email := strings.ToLower(strings.TrimSpace(a.Email))
	first, last := strings.TrimSpace(a.FirstName), strings.TrimSpace(a.LastName)
	bvn, nin := strings.TrimSpace(a.BVN), strings.TrimSpace(a.NIN)
	acct := strings.TrimSpace(a.AccountNumber)
	switch {
	case email == "" || !strings.Contains(email, "@"):
		return nil, nil, errors.New("email is required")
	case first == "" || last == "":
		return nil, nil, errors.New("first_name and last_name are required")
	case !digits(bvn, 11):
		return nil, nil, errors.New("bvn must be 11 digits")
	case !digits(nin, 11):
		return nil, nil, errors.New("nin must be 11 digits")
	case !digits(acct, 10):
		return nil, nil, errors.New("account_number must be the 10-digit Wema account number")
	case len(a.Password) < 8:
		return nil, nil, errors.New("password must be at least 8 characters")
	case len(strings.TrimSpace(a.PIN)) < 4 || len(strings.TrimSpace(a.PIN)) > 6 || !digits(strings.TrimSpace(a.PIN), len(strings.TrimSpace(a.PIN))):
		return nil, nil, errors.New("pin must be 4-6 digits")
	}
	dob, err := time.Parse("2006-01-02", strings.TrimSpace(a.DOB))
	if err != nil {
		return nil, nil, errors.New("dob must be YYYY-MM-DD")
	}
	passwordHash, err := auth.HashPassword(a.Password)
	if err != nil {
		return nil, nil, err
	}
	pinHash, err := auth.HashPassword(strings.TrimSpace(a.PIN))
	if err != nil {
		return nil, nil, err
	}
	accountName := strings.TrimSpace(a.AccountName)
	if accountName == "" {
		accountName = strings.ToUpper(first + " " + last)
	}
	address := strings.TrimSpace(a.Address)

	now := time.Now().UTC()
	capExpires := now.Add(24 * time.Hour)
	userID, internalWalletID := uuid.NewString(), uuid.NewString()
	user := &models.User{
		ID:                     userID,
		WalletID:               internalWalletID,
		Phone:                  phone,
		Email:                  &email,
		Address:                &address,
		FirstName:              first,
		LastName:               last,
		PasswordHash:           passwordHash,
		PinHash:                pinHash,
		DOB:                    dob,
		BVN:                    bvn,
		NIN:                    nin,
		IsEmailVerified:        true,
		IsPhoneVerified:        true,
		IsBvnVerified:          true,
		IsNinVerified:          true, // Wema validates the NIN when it opens a tier 2 account
		IsNotificationsEnabled: true,
		ActivationCapExpiresAt: &capExpires,
	}
	w := &wallet.CustomerWallet{
		ID:               uuid.NewString(),
		InternalWalletID: internalWalletID,
		MobileUserID:     userID,
		PhoneNumber:      phone,
		WalletCustomerID: userID,
		Metadata:         types.JSONMap{},
		Currency:         "NGN",
		DateOfBirth:      strings.TrimSpace(a.DOB),
		FirstName:        first,
		LastName:         last,
		Email:            email,
		Address:          address,
		Tier:             "2",
		WalletID:         acct,
		BankName:         wemaBankName,
		BankCode:         wemaBankCode,
		Provider:         "wema",
		AccountNumber:    acct,
		AccountName:      accountName,
		Status:           "active",
		CreatedAt:        now,
	}
	return user, w, nil
}

// alreadyExists reports what clashes with an existing row ("" if nothing).
func alreadyExists(ctx context.Context, db *gorm.DB, u *models.User, w *wallet.CustomerWallet) (string, error) {
	checks := []struct {
		what, query string
		arg         any
	}{
		{"user with this phone", "phone = ?", u.Phone},
		{"user with this email", "email = ?", *u.Email},
		{"user with this BVN", "bvn_hash = ?", crypto.Hash(u.BVN)},
		{"user with this NIN", "nin_hash = ?", crypto.Hash(u.NIN)},
	}
	for _, c := range checks {
		var n int64
		if err := db.WithContext(ctx).Model(&models.User{}).Where(c.query, c.arg).Count(&n).Error; err != nil {
			return "", err
		}
		if n > 0 {
			return c.what, nil
		}
	}
	var n int64
	if err := db.WithContext(ctx).Model(&wallet.CustomerWallet{}).Where("account_number = ?", w.AccountNumber).Count(&n).Error; err != nil {
		return "", err
	}
	if n > 0 {
		return "wallet for this account number", nil
	}
	return "", nil
}

// save writes the user and wallet in one transaction.
func save(ctx context.Context, db *gorm.DB, cipher *crypto.FieldCipher, u *models.User, w *wallet.CustomerWallet) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := auth.NewRespository(tx, cipher).CreateUser(ctx, u); err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if err := wallet.NewRepository(tx).CreateWallet(ctx, w); err != nil {
			return fmt.Errorf("create wallet: %w", err)
		}
		return nil
	})
}

func digits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func maskPhone(p string) string {
	p = strings.TrimSpace(p)
	if len(p) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(p)-4) + p[len(p)-4:]
}
