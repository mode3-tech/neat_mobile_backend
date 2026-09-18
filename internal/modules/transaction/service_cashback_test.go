package transaction

import (
	"testing"
	"time"
)

func TestToTransactionResponseIncludesCashbackUsage(t *testing.T) {
	resp := toTransactionResponse(Transaction{
		ID:             "tx-1",
		Type:           TransactionTypeDebit,
		Description:    "Airtime",
		Reference:      "ref-1",
		Status:         TransactionStatusSuccessful,
		Amount:         7500,
		CashbackAmount: 2500,
		UsedCashback:   true,
		CreatedAt:      time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
	})

	if !resp.UsedCashback {
		t.Fatal("expected cashback usage to be exposed")
	}
	if resp.CashbackAmount != 25 {
		t.Fatalf("cashback amount = %v, want 25", resp.CashbackAmount)
	}
	if resp.ActualAmount != 100 {
		t.Fatalf("actual amount = %v, want 100", resp.ActualAmount)
	}
}
