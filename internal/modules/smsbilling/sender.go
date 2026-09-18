package smsbilling

import "context"

// sender is the minimal shape every existing SMS sender interface in this
// codebase already satisfies (wallet.SmsSender, loanproduct.SMSSender,
// notify.SMSSender). Kept local rather than importing one of theirs, so this
// package can't create an import cycle with any of its callers.
type sender interface {
	Send(ctx context.Context, destination, message string) error
}

// BillableSMSSender is implemented by BillableSender. Call sites that have a
// mobile_user_id in scope should go through Dispatch (or SendBillable
// directly) rather than calling Send, so the send gets billed.
type BillableSMSSender interface {
	sender
	SendBillable(ctx context.Context, mobileUserID, destination, message, purpose string) error
}

// Dispatch sends through s, billing the customer for it when both s supports
// billing and a mobileUserID is available. Pre-account flows (e.g. signup
// phone/email verification, where no wallet exists yet) pass an empty
// mobileUserID and fall back to a plain, unbilled send.
func Dispatch(ctx context.Context, s sender, mobileUserID, destination, message, purpose string) error {
	if mobileUserID != "" {
		if b, ok := s.(BillableSMSSender); ok {
			return b.SendBillable(ctx, mobileUserID, destination, message, purpose)
		}
	}
	return s.Send(ctx, destination, message)
}

// BillableSender wraps a real SMS provider so plain Send calls (existing
// unbilled flows, or a billable flow with no user in scope yet) pass through
// unchanged, while SendBillable also records and attempts collection of an
// SMS charge after a successful send. Wire this in place of the raw provider
// wherever billing should be available, then call Dispatch (or SendBillable)
// at call sites that have a mobile_user_id.
type BillableSender struct {
	Underlying sender
	Billing    *Service
}

func NewBillableSender(underlying sender, billing *Service) *BillableSender {
	return &BillableSender{Underlying: underlying, Billing: billing}
}

func (b *BillableSender) Send(ctx context.Context, destination, message string) error {
	return b.Underlying.Send(ctx, destination, message)
}

func (b *BillableSender) SendBillable(ctx context.Context, mobileUserID, destination, message, purpose string) error {
	if err := b.Underlying.Send(ctx, destination, message); err != nil {
		return err
	}
	if b.Billing != nil && mobileUserID != "" {
		go b.Billing.ChargeAndCollect(context.Background(), mobileUserID, destination, message, purpose)
	}
	return nil
}
