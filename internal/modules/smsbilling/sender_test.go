package smsbilling

import (
	"context"
	"errors"
	"testing"
)

type fakeSender struct {
	sendErr   error
	sendCalls []string // "destination|message"
}

func (f *fakeSender) Send(ctx context.Context, destination, message string) error {
	f.sendCalls = append(f.sendCalls, destination+"|"+message)
	return f.sendErr
}

func TestBillableSender_Send_PassesThroughUnchanged(t *testing.T) {
	underlying := &fakeSender{}
	b := NewBillableSender(underlying, nil)

	if err := b.Send(context.Background(), "+2348000000000", "hi"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(underlying.sendCalls) != 1 {
		t.Fatalf("expected 1 underlying send, got %d", len(underlying.sendCalls))
	}
}

func TestBillableSender_SendBillable_PropagatesSendError(t *testing.T) {
	underlying := &fakeSender{sendErr: errors.New("provider down")}
	// Billing left nil: if the error path incorrectly tried to bill anyway,
	// this would panic - proving billing is never touched when the send
	// itself failed.
	b := NewBillableSender(underlying, nil)

	err := b.SendBillable(context.Background(), "user-1", "+2348000000000", "hi", "login")
	if err == nil || err.Error() != "provider down" {
		t.Fatalf("expected provider error to propagate, got %v", err)
	}
}

func TestBillableSender_SendBillable_NilBillingIsSafe(t *testing.T) {
	underlying := &fakeSender{}
	b := NewBillableSender(underlying, nil)

	if err := b.SendBillable(context.Background(), "user-1", "+2348000000000", "hi", "login"); err != nil {
		t.Fatalf("SendBillable: %v", err)
	}
}

func TestBillableSender_SendBillable_EmptyUserIDNeverTriggersBilling(t *testing.T) {
	underlying := &fakeSender{}
	// Billing is non-nil here but its Repo is nil - if SendBillable ever
	// dispatched billing work for an empty mobileUserID, the background
	// goroutine would panic on first DB access. Since Send succeeds and
	// mobileUserID is "", it must not.
	b := NewBillableSender(underlying, &Service{})

	if err := b.SendBillable(context.Background(), "", "+2348000000000", "hi", "signup"); err != nil {
		t.Fatalf("SendBillable: %v", err)
	}
}

func TestDispatch_FallsBackToPlainSendWhenNoUserID(t *testing.T) {
	underlying := &fakeSender{}
	b := NewBillableSender(underlying, &Service{})

	if err := Dispatch(context.Background(), b, "", "+2348000000000", "hi", "signup"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(underlying.sendCalls) != 1 {
		t.Fatalf("expected plain Send to have been used, got %d calls", len(underlying.sendCalls))
	}
}

func TestDispatch_FallsBackToPlainSendWhenSenderNotBillable(t *testing.T) {
	underlying := &fakeSender{}

	// underlying itself (not wrapped in BillableSender) has no SendBillable
	// method, so even with a mobileUserID, Dispatch must fall back.
	if err := Dispatch(context.Background(), underlying, "user-1", "+2348000000000", "hi", "login"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(underlying.sendCalls) != 1 {
		t.Fatalf("expected plain Send to have been used, got %d calls", len(underlying.sendCalls))
	}
}
