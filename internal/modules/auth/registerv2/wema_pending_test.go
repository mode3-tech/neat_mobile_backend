package registerv2

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"neat_mobile_app_backend/internal/crypto"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/modules/auth"
	phoneutil "neat_mobile_app_backend/internal/phone"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Pending Wema registrations: /register hands off to a pending row when ALAT
// is still queueing the account, and the webhook, sweep or status poll
// completes it.

const pendingID = "pend-1"

type tokenIssuer struct{ issued int }

func (t *tokenIssuer) IssueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*auth.VerifiedDeviceResponse, error) {
	t.issued++
	return &auth.VerifiedDeviceResponse{AccessToken: "access-" + userID, RefreshToken: "refresh-" + userID}, nil
}
func (t *tokenIssuer) ValidateBVN(ctx context.Context, bvn string) (*auth.BVNInfo, error) {
	return nil, nil
}
func (t *tokenIssuer) ValidateBVNWithFace(ctx context.Context, p auth.BVNWithFaceValidationRequest) (*auth.BVNWithFaceInfo, error) {
	return nil, nil
}

type pendingHarness struct {
	s       *Service
	mock    sqlmock.Sqlmock
	alat    *mockAlatValidator
	tokens  *tokenIssuer
	cipher  *crypto.FieldCipher
	persist int // persistRegistration calls
}

func newPendingHarness(t *testing.T) *pendingHarness {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	cipher, err := crypto.NewFieldCipherFromBase64(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	h := &pendingHarness{mock: mock, alat: &mockAlatValidator{}, tokens: &tokenIssuer{}, cipher: cipher}
	h.s = &Service{
		repo:               NewRepository(gormDB, cipher),
		providerPreference: &mockProviderPref{provider: "wema"},
		alat:               h.alat,
		sessionIssuer:      h.tokens,
		wemaPollInterval:   time.Millisecond,
		lockOverride:       func(ctx context.Context, keys ...string) (func(), bool, error) { return func() {}, true, nil },
	}
	// The real save is one large transaction this harness doesn't mock; the
	// fake still runs inTx so the pending row's completion is exercised.
	h.s.persistOverride = func(ctx context.Context, rl *opLogger, reg *registration, wr *auth.WalletResponse, provider string, inTx func(*gorm.DB) error) error {
		h.persist++
		if inTx != nil {
			return inTx(gormDB)
		}
		return nil
	}
	return h
}

var pendingCols = []string{"id", "mobile_user_id", "phone", "email", "first_name", "last_name", "dob",
	"bvn_encrypted", "nin_encrypted", "device_id", "bvn_verification_id", "claim_token_hash", "status", "last_attempt_at", "tracking_id"}

func (h *pendingHarness) pendingRow(t *testing.T, status, claimToken string, lastAttempt *time.Time) []driver.Value {
	t.Helper()
	phone, _ := phoneutil.NormalizeNigerianNumber(testPhone)
	bvn, _ := h.cipher.Encrypt(testBVN)
	nin, _ := h.cipher.Encrypt(testNIN)
	var la driver.Value
	if lastAttempt != nil {
		la = *lastAttempt
	}
	return []driver.Value{pendingID, "user-1", phone, testEmail, "Test", "User", "1990-01-01",
		bvn, nin, "device-1", "bvn-1", hashClaimToken(claimToken), status, la, "track-1"}
}

func (h *pendingHarness) expectGetPending(row []driver.Value) {
	rows := sqlmock.NewRows(pendingCols)
	if row != nil {
		rows.AddRow(row...)
	}
	h.mock.ExpectQuery(`SELECT \* FROM "wallet_pending_wema_registrations" WHERE id = \$1`).WillReturnRows(rows)
}

func (h *pendingHarness) expectActiveByPhone(row []driver.Value) {
	rows := sqlmock.NewRows(pendingCols)
	if row != nil {
		rows.AddRow(row...)
	}
	h.mock.ExpectQuery(`SELECT \* FROM "wallet_pending_wema_registrations" WHERE phone = \$1 AND status IN`).WillReturnRows(rows)
}

func (h *pendingHarness) expectUpdate(pattern string, affected int64) {
	h.mock.ExpectBegin()
	h.mock.ExpectExec(pattern).WillReturnResult(sqlmock.NewResult(0, affected))
	h.mock.ExpectCommit()
}

func (h *pendingHarness) expectStartAttempt(affected int64) {
	h.expectUpdate(`UPDATE "wallet_pending_wema_registrations" SET "attempts"=attempts \+ 1`, affected)
}

func (h *pendingHarness) expectStatusUpdate(status string) {
	h.mock.ExpectBegin()
	h.mock.ExpectExec(`UPDATE "wallet_pending_wema_registrations" SET "failure_reason"=\$1,"status"=\$2`).
		WithArgs(sqlmock.AnyArg(), status, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	h.mock.ExpectCommit()
}

func (h *pendingHarness) expectRelease() {
	h.mock.ExpectBegin()
	h.mock.ExpectExec(`UPDATE "wallet_pending_wema_registrations" SET "failure_reason"=\$1,"status"=\$2`).
		WithArgs(sqlmock.AnyArg(), PendingWemaStatusPending, sqlmock.AnyArg(), pendingID, PendingWemaStatusProcessing).
		WillReturnResult(sqlmock.NewResult(0, 1))
	h.mock.ExpectCommit()
}

func (h *pendingHarness) expectBVNRecord(name string) {
	cols := append(append([]string{}, verificationCols...), "verified_name")
	h.mock.ExpectQuery(`SELECT \* FROM "wallet_verification_records" WHERE id = \$1 LIMIT \$2`).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(append(verifiedBVNRow("bvn-1"), name)...))
}

func (h *pendingHarness) done(t *testing.T) {
	t.Helper()
	if err := h.mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func alatNotFound(ctx context.Context, phone string) (*AlatAccountDetails, error) {
	return nil, &appErr.AlatError{Message: "account not found"}
}

func alatAccount(ctx context.Context, phone string) (*AlatAccountDetails, error) {
	return &AlatAccountDetails{AccountNumber: "0123456789", FirstName: "TEST", LastName: "USER"}, nil
}

// --- /register hand-off -----------------------------------------------------

func expectRegisterUpToALAT(t *testing.T, h *pendingHarness) {
	expectRegisterPreamble(t, h.mock, 0)
	expectMatchedFaceForBVN(h.mock, "bvn-1")
	expectNoPendingWemaRegistration(h.mock)
}

func TestRegisterWema_AccountStillQueued_SavesPendingWhenClientSupportsIt(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatNotFound
	expectRegisterUpToALAT(t, h)
	h.mock.ExpectBegin()
	h.mock.ExpectExec(`INSERT INTO "wallet_pending_wema_registrations"`).WillReturnResult(sqlmock.NewResult(0, 1))
	h.mock.ExpectCommit()

	req := wemaRegisterRequest()
	req.SupportsPending = true
	resp, err := h.s.Register(context.Background(), req, "127.0.0.1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.Status != RegistrationStatusPending || resp.RegistrationID == "" || resp.ClaimToken == "" || resp.AccessToken != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if h.alat.tier2Calls != 1 || h.alat.pndCalls != 0 || h.persist != 0 {
		t.Fatalf("tier2=%d pnd=%d persist=%d; want 1/0/0", h.alat.tier2Calls, h.alat.pndCalls, h.persist)
	}
	h.done(t)
}

func TestRegisterWema_AccountStillQueued_OldClientGetsTryAgain(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatNotFound
	expectRegisterUpToALAT(t, h)

	_, err := h.s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1") // SupportsPending false
	if !errors.Is(err, errWemaAccountNotReady) {
		t.Fatalf("err = %v, want errWemaAccountNotReady", err)
	}
	h.done(t) // no pending row inserted
}

func TestRegisterWema_AccountReadyWhilePolling_CompletesInline(t *testing.T) {
	h := newPendingHarness(t)
	calls := 0
	h.alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) {
		calls++
		if calls == 1 {
			return alatNotFound(ctx, phone) // before creation
		}
		return alatAccount(ctx, phone)
	}
	expectRegisterUpToALAT(t, h)

	req := wemaRegisterRequest()
	req.SupportsPending = true
	resp, err := h.s.Register(context.Background(), req, "127.0.0.1")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.Status != RegistrationStatusCompleted || resp.AccessToken == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if h.persist != 1 || h.alat.pndCalls != 1 {
		t.Fatalf("persist=%d pnd=%d; want 1/1", h.persist, h.alat.pndCalls)
	}
	h.done(t)
}

// A retry with a different BVN can't take over the pending registration
// whose ALAT account was requested with the first one.
func TestRegisterWema_ResumeWithDifferentBVNRejected(t *testing.T) {
	h := newPendingHarness(t)
	expectRegisterPreamble(t, h.mock, 0)
	expectMatchedFaceForBVN(h.mock, "bvn-1")
	row := h.pendingRow(t, PendingWemaStatusPending, "old-token", nil)
	other, _ := h.cipher.Encrypt("99999999999")
	row[7] = other // bvn_encrypted
	h.expectActiveByPhone(row)

	req := wemaRegisterRequest()
	req.SupportsPending = true
	_, err := h.s.Register(context.Background(), req, "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "different BVN or NIN") {
		t.Fatalf("err = %v, want different-BVN rejection", err)
	}
	if h.alat.getStep3Calls != 0 || h.alat.tier2Calls != 0 {
		t.Fatal("no ALAT call expected")
	}
	h.done(t)
}

// --- Completion: webhook, sweep, status poll ----------------------------------

func TestTryCompletePendingWema_Completes(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatAccount
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectBVNRecord("Test Middle User")
	h.expectUpdate(`UPDATE "wallet_pending_wema_registrations" SET "account_number"=\$1,"completed_at"=\$2`, 1)

	done, err := h.s.tryCompletePendingWema(context.Background(), pendingID)
	if err != nil || !done {
		t.Fatalf("done=%v err=%v; want completed", done, err)
	}
	if h.persist != 1 || h.alat.pndCalls != 1 {
		t.Fatalf("persist=%d pnd=%d; want 1/1", h.persist, h.alat.pndCalls)
	}
	h.done(t)
}

// Another worker (webhook vs sweep vs poll) already has it: nothing happens.
func TestTryCompletePendingWema_AlreadyTakenSkips(t *testing.T) {
	h := newPendingHarness(t)
	h.expectStartAttempt(0)

	done, err := h.s.tryCompletePendingWema(context.Background(), pendingID)
	if err != nil || done {
		t.Fatalf("done=%v err=%v; want skipped", done, err)
	}
	if h.alat.getStep3Calls != 0 {
		t.Fatal("no ALAT call expected when the attempt wasn't started")
	}
	h.done(t)
}

func TestTryCompletePendingWema_AccountNotReadyReleases(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) { return nil, nil }
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectRelease()

	done, err := h.s.tryCompletePendingWema(context.Background(), pendingID)
	if err != nil || done {
		t.Fatalf("done=%v err=%v; want released", done, err)
	}
	if h.persist != 0 {
		t.Fatal("nothing should be saved")
	}
	h.done(t)
}

func TestTryCompletePendingWema_ALATUnreachableReleases(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) {
		return nil, &appErr.AlatError{Message: "alat request failed", NoResponse: true}
	}
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectRelease()

	if done, err := h.s.tryCompletePendingWema(context.Background(), pendingID); err != nil || done {
		t.Fatalf("done=%v err=%v; want released", done, err)
	}
	h.done(t)
}

// The webhook is unsigned: an account that isn't the applicant's is never
// attached to them.
func TestTryCompletePendingWema_AccountNotApplicantsFails(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatAccount // TEST USER
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectBVNRecord("Chidi Okafor")
	h.expectStatusUpdate(PendingWemaStatusFailed)

	if done, err := h.s.tryCompletePendingWema(context.Background(), pendingID); err != nil || done {
		t.Fatalf("done=%v err=%v; want failed", done, err)
	}
	if h.alat.pndCalls != 0 || h.persist != 0 {
		t.Fatal("no PND lift or save for someone else's account")
	}
	h.done(t)
}

func TestTryCompletePendingWema_PNDFailureReleases(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatAccount
	h.alat.pndStep4Func = func(ctx context.Context, pndType, acct string) error { return errors.New("pnd down") }
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectBVNRecord("Test User")
	h.expectRelease()

	if done, err := h.s.tryCompletePendingWema(context.Background(), pendingID); err != nil || done {
		t.Fatalf("done=%v err=%v; want released", done, err)
	}
	if h.persist != 0 {
		t.Fatal("nothing should be saved before PND is lifted")
	}
	h.done(t)
}

// --- Webhook ------------------------------------------------------------------

func TestOnAlatAccountCreated_UnknownPhoneDoesNothing(t *testing.T) {
	h := newPendingHarness(t)
	h.expectActiveByPhone(nil)

	h.s.OnAlatAccountCreated(context.Background(), "07011223344")
	if h.alat.getStep3Calls != 0 {
		t.Fatal("no ALAT call expected without a pending registration")
	}
	h.done(t)
}

func TestOnAlatAccountCreated_InvalidPhoneIgnored(t *testing.T) {
	h := newPendingHarness(t)
	h.s.OnAlatAccountCreated(context.Background(), "not-a-phone")
	h.done(t) // no query at all
}

func TestOnAlatAccountCreated_CompletesPendingRegistration(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatAccount
	h.expectActiveByPhone(h.pendingRow(t, PendingWemaStatusPending, "tok", nil))
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectBVNRecord("Test User")
	h.expectUpdate(`UPDATE "wallet_pending_wema_registrations" SET "account_number"=\$1`, 1)

	h.s.OnAlatAccountCreated(context.Background(), testPhone)
	if h.persist != 1 {
		t.Fatalf("persist=%d; want 1", h.persist)
	}
	h.done(t)
}

// A repeated webhook after completion finds no active row.
func TestOnAlatAccountCreated_DuplicateAfterCompletionDoesNothing(t *testing.T) {
	h := newPendingHarness(t)
	h.expectActiveByPhone(nil) // completed rows aren't active

	h.s.OnAlatAccountCreated(context.Background(), testPhone)
	if h.persist != 0 || h.alat.getStep3Calls != 0 {
		t.Fatal("a repeated webhook must not complete anything again")
	}
	h.done(t)
}

// --- Status endpoint --------------------------------------------------------------

func TestRegistrationStatus_WrongTokenIsNotFound(t *testing.T) {
	h := newPendingHarness(t)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusCompleted, "right", nil))

	if _, err := h.s.RegistrationStatus(context.Background(), pendingID, "wrong", "127.0.0.1"); !errors.Is(err, errRegistrationNotFound) {
		t.Fatalf("err = %v, want errRegistrationNotFound", err)
	}
	if h.tokens.issued != 0 {
		t.Fatal("no session for a wrong token")
	}
	h.done(t)
}

func TestRegistrationStatus_UnknownIDIsNotFound(t *testing.T) {
	h := newPendingHarness(t)
	h.expectGetPending(nil)
	if _, err := h.s.RegistrationStatus(context.Background(), "nope", "tok", "127.0.0.1"); !errors.Is(err, errRegistrationNotFound) {
		t.Fatalf("err = %v, want errRegistrationNotFound", err)
	}
	h.done(t)
}

// Recently attempted: the poll reports pending without hitting ALAT.
func TestRegistrationStatus_PendingRecentlyAttempted(t *testing.T) {
	h := newPendingHarness(t)
	recent := time.Now().UTC()
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusPending, "tok", &recent))

	resp, err := h.s.RegistrationStatus(context.Background(), pendingID, "tok", "127.0.0.1")
	if err != nil || resp.Status != RegistrationStatusPending {
		t.Fatalf("resp=%+v err=%v; want pending", resp, err)
	}
	if h.alat.getStep3Calls != 0 {
		t.Fatal("ALAT must not be polled on every status request")
	}
	h.done(t)
}

// Not attempted lately: the poll itself completes it and returns the session.
func TestRegistrationStatus_PollCompletesAndIssuesSession(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatAccount
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusPending, "tok", nil))
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectBVNRecord("Test User")
	h.expectUpdate(`UPDATE "wallet_pending_wema_registrations" SET "account_number"=\$1`, 1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusCompleted, "tok", nil))
	h.expectUpdate(`UPDATE "wallet_pending_wema_registrations" SET "claimed_at"=\$1`, 1)

	resp, err := h.s.RegistrationStatus(context.Background(), pendingID, "tok", "127.0.0.1")
	if err != nil || resp.Status != RegistrationStatusCompleted || resp.AccessToken != "access-user-1" {
		t.Fatalf("resp=%+v err=%v; want completed with tokens", resp, err)
	}
	h.done(t)
}

func TestRegistrationStatus_SessionIssuedOnlyOnce(t *testing.T) {
	h := newPendingHarness(t)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusCompleted, "tok", nil))
	h.expectUpdate(`UPDATE "wallet_pending_wema_registrations" SET "claimed_at"=\$1`, 0) // already claimed

	if _, err := h.s.RegistrationStatus(context.Background(), pendingID, "tok", "127.0.0.1"); !errors.Is(err, errRegistrationClaimed) {
		t.Fatalf("err = %v, want errRegistrationClaimed", err)
	}
	if h.tokens.issued != 0 {
		t.Fatal("a claimed registration must not issue another session")
	}
	h.done(t)
}

func TestRegistrationStatus_FailedAndExpired(t *testing.T) {
	for _, status := range []string{PendingWemaStatusFailed, PendingWemaStatusExpired} {
		h := newPendingHarness(t)
		h.expectGetPending(h.pendingRow(t, status, "tok", nil))
		resp, err := h.s.RegistrationStatus(context.Background(), pendingID, "tok", "127.0.0.1")
		if err != nil || resp.Status != status || resp.Message == "" || resp.AccessToken != "" {
			t.Fatalf("%s: resp=%+v err=%v", status, resp, err)
		}
		h.done(t)
	}
}

// --- Sweep ----------------------------------------------------------------------

func TestSweepPendingWemaRegistrations(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = func(ctx context.Context, phone string) (*AlatAccountDetails, error) { return nil, nil }

	h.expectUpdate(`UPDATE "wallet_pending_wema_registrations" SET "status"=\$1,"updated_at"=\$2 WHERE status = \$3 AND last_attempt_at`, 1) // stuck reset
	h.mock.ExpectBegin()
	h.mock.ExpectQuery(`UPDATE "wallet_pending_wema_registrations" SET "failure_reason"=\$1,"status"=\$2.* RETURNING`).
		WillReturnRows(sqlmock.NewRows(pendingCols).AddRow(h.pendingRow(t, PendingWemaStatusExpired, "tok", nil)...))
	h.mock.ExpectCommit()
	h.mock.ExpectQuery(`SELECT \* FROM "wallet_pending_wema_registrations" WHERE status = \$1 AND \(last_attempt_at IS NULL OR last_attempt_at < \$2\)`).
		WillReturnRows(sqlmock.NewRows(pendingCols).AddRow(h.pendingRow(t, PendingWemaStatusPending, "tok", nil)...))
	// The due row is attempted: not ready yet, released.
	h.expectStartAttempt(1)
	h.expectGetPending(h.pendingRow(t, PendingWemaStatusProcessing, "tok", nil))
	h.expectRelease()

	if err := h.s.SweepPendingWemaRegistrations(context.Background(), 20); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if h.alat.getStep3Calls != 1 {
		t.Fatalf("ALAT lookups = %d; want 1 (the due row)", h.alat.getStep3Calls)
	}
	h.done(t)
}

type flakyIssuer struct {
	tokenIssuer
	failures int
}

func (f *flakyIssuer) IssueSessionTokens(ctx context.Context, userID, deviceID, ip string) (*auth.VerifiedDeviceResponse, error) {
	if f.failures > 0 {
		f.failures--
		return nil, errors.New("redis down")
	}
	return f.tokenIssuer.IssueSessionTokens(ctx, userID, deviceID, ip)
}

// Once the user is saved, a session failure must not be reported as a failed
// registration.
func TestIssueSession_RetriesThenReportsLoginRequired(t *testing.T) {
	rl := newOpLogger("test")

	once := &flakyIssuer{failures: 1}
	resp, err := (&Service{sessionIssuer: once}).issueSession(context.Background(), rl, "user-1", "dev", "ip")
	if err != nil || resp.Status != RegistrationStatusCompleted || resp.AccessToken == "" || resp.LoginRequired {
		t.Fatalf("one failure: resp=%+v err=%v; want tokens after retry", resp, err)
	}

	always := &flakyIssuer{failures: 5}
	resp, err = (&Service{sessionIssuer: always}).issueSession(context.Background(), rl, "user-1", "dev", "ip")
	if err != nil || resp.Status != RegistrationStatusCompleted || !resp.LoginRequired || resp.AccessToken != "" {
		t.Fatalf("persistent failure: resp=%+v err=%v; want completed + login_required", resp, err)
	}
}

// Selfies are stored encrypted; the repository hands back plaintext, and rows
// written before encryption still read.
func TestFaceCheckRecord_ImageDecryptedOnRead(t *testing.T) {
	h := newPendingHarness(t)
	enc, err := h.cipher.Encrypt("live-image-b64")
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range []string{enc, "legacy-plain-b64"} {
		h.mock.ExpectQuery(`SELECT \* FROM "wallet_face_check_records" WHERE id = \$1`).
			WillReturnRows(sqlmock.NewRows(faceCheckCols).AddRow("face-1", "bvn-1", true, stored))
		fc, err := h.s.repo.GetFaceCheckRecord(context.Background(), "face-1")
		if err != nil {
			t.Fatal(err)
		}
		want := "live-image-b64"
		if stored == "legacy-plain-b64" {
			want = stored
		}
		if fc.FaceImageProvided == nil || *fc.FaceImageProvided != want {
			t.Fatalf("stored %q: got %v, want %q", stored, fc.FaceImageProvided, want)
		}
	}
	h.done(t)
}

// A second /register for the same person while one is running stops before
// the duplicate checks and before ALAT.
func TestRegister_ConcurrentRegistrationRejected(t *testing.T) {
	h := newPendingHarness(t)
	var keys []string
	h.s.lockOverride = func(ctx context.Context, k ...string) (func(), bool, error) {
		keys = k
		return nil, false, nil
	}
	phone, _ := phoneutil.NormalizeNigerianNumber(testPhone)
	expectVerificationByID(h.mock, testPhoneVer, []driver.Value{testPhoneVer, "phone", "verified", "h", phone, nil, nil})
	expectVerificationByID(h.mock, testEmailVer, []driver.Value{testEmailVer, "email", "verified", "h", nil, testEmail, nil})

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	_, err := h.s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("err = %v, want in-progress rejection", err)
	}
	// The payload this request would have sent is logged, redacted.
	out := logs.String()
	for _, want := range []string{"would have sent to alat", "payload={", "***", "correlationId", "not-sent"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
	for _, secret := range []string{testBVN, testNIN} {
		if strings.Contains(out, secret) {
			t.Errorf("log leaks %q", secret)
		}
	}
	if len(keys) != 2 || keys[0] != "phone:"+phone || keys[1] != "bvn:"+bvnSubjectHash(testBVN) {
		t.Fatalf("lock keys = %v; want phone and bvn", keys)
	}
	if h.alat.getStep3Calls != 0 || h.alat.tier2Calls != 0 {
		t.Fatal("no ALAT call while another registration holds the lock")
	}
	h.done(t)
}

func TestLockRegistration(t *testing.T) {
	ids := []int64{registrationLockID("phone:a"), registrationLockID("bvn:b")}
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}

	t.Run("acquires both and releases both", func(t *testing.T) {
		h := newPendingHarness(t)
		h.mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(ids[0]).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
		h.mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(ids[1]).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
		h.mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(ids[0]).WillReturnResult(sqlmock.NewResult(0, 0))
		h.mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(ids[1]).WillReturnResult(sqlmock.NewResult(0, 0))

		release, ok, err := h.s.repo.LockRegistration(context.Background(), "phone:a", "bvn:b")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		release()
		h.done(t)
	})

	t.Run("busy second lock releases the first", func(t *testing.T) {
		h := newPendingHarness(t)
		h.mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(ids[0]).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
		h.mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(ids[1]).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(false))
		h.mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(ids[0]).WillReturnResult(sqlmock.NewResult(0, 0))

		_, ok, err := h.s.repo.LockRegistration(context.Background(), "phone:a", "bvn:b")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v; want not acquired", ok, err)
		}
		h.done(t)
	})
}

// ALAT answering a retry with "already being generated" leads to the pending
// page, not a creation failure.
func TestRegisterWema_ALATAlreadyGenerating_GoesPending(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatNotFound
	h.alat.tier2Func = func(ctx context.Context, req AlatTier2FaceRequest) (string, error) {
		return "", ErrAlatAccountGenerationPending
	}
	expectRegisterUpToALAT(t, h)
	h.mock.ExpectBegin()
	h.mock.ExpectExec(`INSERT INTO "wallet_pending_wema_registrations"`).WillReturnResult(sqlmock.NewResult(0, 1))
	h.mock.ExpectCommit()

	req := wemaRegisterRequest()
	req.SupportsPending = true
	resp, err := h.s.Register(context.Background(), req, "127.0.0.1")
	if err != nil || resp.Status != RegistrationStatusPending || resp.ClaimToken == "" {
		t.Fatalf("resp=%+v err=%v; want pending", resp, err)
	}
	h.done(t)
}

func TestRegisterWema_ALATAlreadyGenerating_OldClientGetsTryAgain(t *testing.T) {
	h := newPendingHarness(t)
	h.alat.getStep3Func = alatNotFound
	h.alat.tier2Func = func(ctx context.Context, req AlatTier2FaceRequest) (string, error) {
		return "", ErrAlatAccountGenerationPending
	}
	expectRegisterUpToALAT(t, h)

	_, err := h.s.Register(context.Background(), wemaRegisterRequest(), "127.0.0.1")
	if !errors.Is(err, errWemaAccountNotReady) {
		t.Fatalf("err = %v, want errWemaAccountNotReady (not a creation failure)", err)
	}
	h.done(t)
}
