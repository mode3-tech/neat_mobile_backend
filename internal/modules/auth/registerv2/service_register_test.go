package registerv2

import (
	"context"
	"testing"
)

// These cover the validation steps that run before any repository access, so
// a zero-value Service (nil repo, nil provider preference, etc.) is safe to
// call directly. Register's later steps (verification lookups, wallet
// generation, user creation) need a real DB via *tx.Transactor and aren't
// covered here - same DB-testing gap noted for wallet.Service elsewhere in
// this codebase.
func TestRegister_RejectsWeakPassword(t *testing.T) {
	s := &Service{}
	_, err := s.Register(context.Background(), OptimusRegisterRequest{
		Password:              "weak",
		ConfirmPassword:       "weak",
		TransactionPin:        "1234",
		ConfirmTransactionPin: "1234",
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected an error for a weak password")
	}
}

func TestRegister_RejectsPasswordMismatch(t *testing.T) {
	s := &Service{}
	_, err := s.Register(context.Background(), OptimusRegisterRequest{
		Password:              "Str0ng!Pass",
		ConfirmPassword:       "Different!1",
		TransactionPin:        "1234",
		ConfirmTransactionPin: "1234",
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected an error for mismatched password confirmation")
	}
}

func TestRegister_RejectsInvalidPin(t *testing.T) {
	s := &Service{}
	_, err := s.Register(context.Background(), OptimusRegisterRequest{
		Password:              "Str0ng!Pass",
		ConfirmPassword:       "Str0ng!Pass",
		TransactionPin:        "12ab",
		ConfirmTransactionPin: "12ab",
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected an error for a non-numeric pin")
	}
}

func TestRegister_RejectsPinMismatch(t *testing.T) {
	s := &Service{}
	_, err := s.Register(context.Background(), OptimusRegisterRequest{
		Password:              "Str0ng!Pass",
		ConfirmPassword:       "Str0ng!Pass",
		TransactionPin:        "1234",
		ConfirmTransactionPin: "4321",
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected an error for mismatched pin confirmation")
	}
}
