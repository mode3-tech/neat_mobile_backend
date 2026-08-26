package referrals

import (
	"context"
	"errors"
	appErr "neat_mobile_app_backend/internal/errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockService(t *testing.T) (*Service, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		DisableAutomaticPing: true,
	})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("open gorm db: %v", err)
	}

	repo := NewRepository(gormDB)
	cleanup := func() {
		_ = sqlDB.Close()
	}

	return NewService(repo), mock, cleanup
}

func findReferralByCodePattern() string {
	return regexp.QuoteMeta(`SELECT * FROM "wallet_referral_codes" WHERE code = $1 ORDER BY "wallet_referral_codes"."id" LIMIT $2`)
}

func findRedemptionByReferredUserPattern() string {
	return regexp.QuoteMeta(`SELECT * FROM "wallet_referral_redemptions" WHERE referred_user_id = $1 ORDER BY "wallet_referral_redemptions"."id" LIMIT $2`)
}

func redeemReferralInsertPattern() string {
	return regexp.QuoteMeta(`INSERT INTO "wallet_referral_redemptions"`)
}

func TestRedeemReferralCode_RejectsSelfReferral(t *testing.T) {
	service, mock, cleanup := newMockService(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"id", "code", "mobile_user_id", "created_at"}).
		AddRow("code-1", "SELF10", "user-1", time.Now())

	mock.ExpectQuery(findReferralByCodePattern()).
		WillReturnRows(rows)

	err := service.RedeemReferralCode(context.Background(), "user-1", "SELF10")
	if !errors.Is(err, appErr.ErrSelfReferral) {
		t.Fatalf("expected ErrSelfReferral, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRedeemReferralCode_DuplicateSameCode_Idempotent(t *testing.T) {
	service, mock, cleanup := newMockService(t)
	defer cleanup()

	referralRows := sqlmock.NewRows([]string{"id", "code", "mobile_user_id", "created_at"}).
		AddRow("code-1", "ABC123", "referrer-1", time.Now())
	mock.ExpectQuery(findReferralByCodePattern()).WillReturnRows(referralRows)

	mock.ExpectBegin()
	existingRows := sqlmock.NewRows([]string{"id", "referrer_user_id", "referred_user_id", "cashback_status", "created_at"}).
		AddRow("redemption-1", "referrer-1", "referred-1", "credited", time.Now())
	mock.ExpectQuery(findRedemptionByReferredUserPattern()).WillReturnRows(existingRows)
	mock.ExpectCommit()

	err := service.RedeemReferralCode(context.Background(), "referred-1", "ABC123")
	if err != nil {
		t.Fatalf("expected idempotent no-op, got error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRedeemReferralCode_DuplicateDifferentCode_ReturnsConflict(t *testing.T) {
	service, mock, cleanup := newMockService(t)
	defer cleanup()

	referralRows := sqlmock.NewRows([]string{"id", "code", "mobile_user_id", "created_at"}).
		AddRow("code-2", "XYZ789", "referrer-2", time.Now())
	mock.ExpectQuery(findReferralByCodePattern()).WillReturnRows(referralRows)

	mock.ExpectBegin()
	existingRows := sqlmock.NewRows([]string{"id", "referrer_user_id", "referred_user_id", "cashback_status", "created_at"}).
		AddRow("redemption-1", "referrer-1", "referred-1", "credited", time.Now())
	mock.ExpectQuery(findRedemptionByReferredUserPattern()).WillReturnRows(existingRows)
	mock.ExpectRollback()

	err := service.RedeemReferralCode(context.Background(), "referred-1", "XYZ789")
	if !errors.Is(err, appErr.ErrReferralAlreadyRedeemed) {
		t.Fatalf("expected ErrReferralAlreadyRedeemed, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

// TestRedeemReferralCode_CreditFailureLeavesRedemptionPending verifies that a
// failure crediting the referrer's cashback (e.g. a transient DB error) does
// not roll back the redemption itself. The credit attempt runs in its own
// savepoint, so only that nested work is undone; the outer transaction still
// commits with the redemption row left at CashbackStatusPending for the retry
// sweep to pick up later.
func TestRedeemReferralCode_CreditFailureLeavesRedemptionPending(t *testing.T) {
	service, mock, cleanup := newMockService(t)
	defer cleanup()

	referralRows := sqlmock.NewRows([]string{"id", "code", "mobile_user_id", "created_at"}).
		AddRow("code-3", "DEF456", "referrer-3", time.Now())
	mock.ExpectQuery(findReferralByCodePattern()).WillReturnRows(referralRows)

	mock.ExpectBegin()
	mock.ExpectQuery(findRedemptionByReferredUserPattern()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec(redeemReferralInsertPattern()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec(`^SAVEPOINT sp\d+$`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id","wallet_id" FROM "wallet_users" WHERE id = $1 ORDER BY "wallet_users"."id" LIMIT $2 FOR UPDATE`)).
		WillReturnError(errors.New("db timeout"))
	mock.ExpectExec(`^ROLLBACK TO SAVEPOINT sp\d+$`).WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectCommit()

	err := service.RedeemReferralCode(context.Background(), "referred-3", "DEF456")
	if err != nil {
		t.Fatalf("expected redemption to succeed despite credit failure, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}
