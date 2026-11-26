package main

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func validAccount() account {
	return account{
		Phone: "08012345678", Email: " Jane@Example.com ", FirstName: "Jane", LastName: "Doe",
		DOB: "1990-01-31", BVN: "22222222222", NIN: "33333333333", Address: "12 Broad Street, Lagos",
		Password: "a-password", PIN: "1234", AccountNumber: "0123456789",
	}
}

func TestBuild_ProducesLoginReadyRows(t *testing.T) {
	u, w, err := build(validAccount())
	if err != nil {
		t.Fatal(err)
	}
	if u.Phone != "2348012345678" || w.PhoneNumber != u.Phone {
		t.Fatalf("phone not normalized the way login looks it up: user=%q wallet=%q", u.Phone, w.PhoneNumber)
	}
	if *u.Email != "jane@example.com" {
		t.Fatalf("email = %q", *u.Email)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("a-password")) != nil ||
		bcrypt.CompareHashAndPassword([]byte(u.PinHash), []byte("1234")) != nil {
		t.Fatal("password and pin must be stored bcrypt-hashed")
	}
	if w.MobileUserID != u.ID || w.InternalWalletID != u.WalletID {
		t.Fatal("wallet must point at the user")
	}
	if w.Provider != "wema" || w.BankCode != "000017" || w.AccountNumber != "0123456789" || w.WalletID != "0123456789" || w.Tier != "2" || w.Status != "active" {
		t.Fatalf("unexpected wallet: %+v", w)
	}
	if w.AccountName != "JANE DOE" {
		t.Fatalf("account name = %q, want it defaulted from the names", w.AccountName)
	}
	if !u.IsPhoneVerified || !u.IsEmailVerified || !u.IsBvnVerified || u.ClosedAt != nil {
		t.Fatalf("user must be verified and open: %+v", u)
	}
}

func TestBuild_Rejects(t *testing.T) {
	cases := map[string]func(*account){
		"bad phone":         func(a *account) { a.Phone = "123" },
		"no email":          func(a *account) { a.Email = "" },
		"short bvn":         func(a *account) { a.BVN = "123" },
		"non-numeric nin":   func(a *account) { a.NIN = "3333333333a" },
		"short account":     func(a *account) { a.AccountNumber = "012345" },
		"short password":    func(a *account) { a.Password = "short" },
		"non-numeric pin":   func(a *account) { a.PIN = "12ab" },
		"too-long pin":      func(a *account) { a.PIN = "1234567" },
		"bad dob":           func(a *account) { a.DOB = "31/01/1990" },
		"missing last name": func(a *account) { a.LastName = " " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := validAccount()
			mutate(&a)
			if _, _, err := build(a); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestMaskPhone(t *testing.T) {
	if got := maskPhone("08012345678"); !strings.HasSuffix(got, "5678") || strings.Contains(got, "0801") {
		t.Fatalf("maskPhone = %q", got)
	}
}
