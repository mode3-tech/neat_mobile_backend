package tier

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	appErr "neat_mobile_app_backend/internal/errors"
	auditlog "neat_mobile_app_backend/internal/modules/audit_log"
)

type fakeAccounts struct {
	number string
	ok     bool
}

func (f fakeAccounts) WemaAccountNumber(context.Context, string) (string, bool, error) {
	return f.number, f.ok, nil
}

type fakeUpgrader struct {
	tier3Account string
	tier3Addr    ResidentialAddress
	err          error
}

func (f *fakeUpgrader) UpgradeToTier2(context.Context, string, string, string, string) error {
	return f.err
}
func (f *fakeUpgrader) UpgradeToTier3(_ context.Context, acct string, addr ResidentialAddress) error {
	f.tier3Account, f.tier3Addr = acct, addr
	return f.err
}
func (f *fakeUpgrader) KYCStatus(context.Context, string) (json.RawMessage, error) {
	return json.RawMessage(`{"tier":2}`), nil
}

type noopAudit struct{}

func (noopAudit) CreateAuditLog(context.Context, *auditlog.AuditLog) error { return nil }

func TestUpgradeWemaTier3(t *testing.T) {
	ctx := context.Background()

	s := &Service{auditLogger: noopAudit{}}
	if err := s.UpgradeWemaTier3(ctx, "u1", ResidentialAddress{}); !errors.Is(err, appErr.ErrWemaUnavailable) {
		t.Errorf("unconfigured: err = %v, want ErrWemaUnavailable", err)
	}

	up := &fakeUpgrader{}
	s.ConfigureWema(fakeAccounts{ok: false}, up)
	if err := s.UpgradeWemaTier3(ctx, "u1", ResidentialAddress{}); !errors.Is(err, appErr.ErrWemaWalletRequired) {
		t.Errorf("non-wema user: err = %v, want ErrWemaWalletRequired", err)
	}

	s.ConfigureWema(fakeAccounts{number: "0123", ok: true}, up)
	addr := ResidentialAddress{Street: "Broad St"}
	if err := s.UpgradeWemaTier3(ctx, "u1", addr); err != nil {
		t.Fatal(err)
	}
	if up.tier3Account != "0123" || up.tier3Addr.Street != "Broad St" {
		t.Errorf("upgrader got account=%q addr=%+v", up.tier3Account, up.tier3Addr)
	}

	data, err := s.GetWemaKYCStatus(ctx, "u1")
	if err != nil || string(data) != `{"tier":2}` {
		t.Errorf("GetWemaKYCStatus = %s, %v", data, err)
	}
}

func TestUpgradeWemaTier2_SkippedForNonWemaUsers(t *testing.T) {
	s := &Service{auditLogger: noopAudit{}}
	// Not configured: no-op.
	if err := s.upgradeWemaTier2(context.Background(), "u1", "enc", "nin", "img"); err != nil {
		t.Fatal(err)
	}
	// Configured but the user isn't on Wema: no-op, the BVN is never decrypted.
	s.ConfigureWema(fakeAccounts{ok: false}, &fakeUpgrader{err: errors.New("must not be called")})
	if err := s.upgradeWemaTier2(context.Background(), "u1", "enc", "nin", "img"); err != nil {
		t.Fatal(err)
	}
}
