package registerv2

import (
	"fmt"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/models"
	"strings"
	"time"

	"github.com/google/uuid"
)

// opLogger gives every log line from one registration-flow call the same
// shape and a shared short request ID, so a single /register (or
// /validate/bvn, /otp/verify, ...) can be traced end to end with one grep:
//
//	[registerv2] level=ERROR op=register req=1a2b3c4d stage=alat_account_lookup provider=wema phone=*********5678 dur=812ms | register failed: ...
//
// Only masked identifiers go into fields - never raw BVN/NIN, full phone or
// email, passwords/PINs, OTPs, or face images.
type opLogger struct {
	op      string
	id      string
	stage   string
	started time.Time
	keys    []string
	values  map[string]string
}

func newOpLogger(op string) *opLogger {
	return &opLogger{
		op:      op,
		id:      strings.ReplaceAll(uuid.NewString(), "-", "")[:8],
		started: time.Now(),
		values:  map[string]string{},
	}
}

// step records which part of the flow is running, so a failure log says
// where it happened without every return site having to say so.
func (l *opLogger) step(stage string) { l.stage = stage }

// with attaches a field to every subsequent line. Empty values are skipped.
func (l *opLogger) with(key, value string) *opLogger {
	value = strings.TrimSpace(value)
	if value == "" {
		return l
	}
	if _, ok := l.values[key]; !ok {
		l.keys = append(l.keys, key)
	}
	l.values[key] = value
	return l
}

func (l *opLogger) Infof(format string, args ...any)  { l.printf("INFO", format, args...) }
func (l *opLogger) Warnf(format string, args ...any)  { l.printf("WARN", format, args...) }
func (l *opLogger) Errorf(format string, args ...any) { l.printf("ERROR", format, args...) }

// Criticalf is for states that need a human: e.g. a provider account that
// exists with nothing saved locally.
func (l *opLogger) Criticalf(format string, args ...any) { l.printf("CRITICAL", format, args...) }

// finish is deferred with a pointer to the caller's named error result, and
// logs the outcome of the whole call once.
func (l *opLogger) finish(errp *error) {
	if errp != nil && *errp != nil {
		l.Errorf("%s failed: %v", l.op, *errp)
		return
	}
	l.Infof("%s succeeded", l.op)
}

func (l *opLogger) printf(level, format string, args ...any) {
	var b strings.Builder
	fmt.Fprintf(&b, "[registerv2] level=%s op=%s req=%s", level, l.op, l.id)
	if l.stage != "" {
		fmt.Fprintf(&b, " stage=%s", l.stage)
	}
	for _, k := range l.keys {
		fmt.Fprintf(&b, " %s=%s", k, l.values[k])
	}
	fmt.Fprintf(&b, " dur=%s | ", time.Since(l.started).Round(time.Millisecond))
	fmt.Fprintf(&b, format, args...)
	log.Print(b.String())
}

// maskPhone keeps the last four digits, like maskIdentityNumber.
func maskPhone(phone string) string { return maskIdentityNumber(strings.TrimSpace(phone)) }

func timeValue(t *time.Time) string {
	if t == nil {
		return "<nil>"
	}
	return t.UTC().Format(time.RFC3339)
}

// The describe* helpers render records for log lines: IDs, statuses and
// timestamps only, never the raw identity values they protect.

func describeContactRecord(rec *models.VerificationRecord, wantType string, valueMatches bool) string {
	if rec == nil {
		return "record not found"
	}
	return fmt.Sprintf("record=%s type=%s(want %s) status=%s(want verified) value_matches_request=%t expires_at=%s",
		rec.ID, rec.Type, wantType, rec.Status, valueMatches, timeValue(rec.ExpiresAt))
}

func describeBVNRecord(rec *models.VerificationRecord) string {
	if rec == nil {
		return "bvn_record=<none>"
	}
	return fmt.Sprintf("bvn_record=%s status=%s provider=%s created_at=%s expires_at=%s",
		rec.ID, rec.Status, rec.Provider, rec.CreatedAt.UTC().Format(time.RFC3339), timeValue(rec.ExpiresAt))
}

func describeFaceRecord(fc *models.FaceCheckRecord) string {
	if fc == nil {
		return "<none>"
	}
	hasImage := fc.FaceImageProvided != nil && strings.TrimSpace(*fc.FaceImageProvided) != ""
	return fmt.Sprintf("{id=%s verification_record=%s matched=%t has_image=%t provider=%s}",
		fc.ID, fc.VerificationRecordID, fc.Matched, hasImage, fc.Provider)
}

func describeAlatError(e *appErr.AlatError) string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("alat status_code=%s code=%s message=%q errors=%v", e.StatusCode, e.Code, e.Message, e.Errors)
}

// describeIdentityMismatch says which fields differ, masking both sides.
func describeIdentityMismatch(d *AlatAccountDetails, email, firstName, lastName string) string {
	var parts []string
	if v := strings.TrimSpace(d.Email); v != "" && !strings.EqualFold(v, strings.TrimSpace(email)) {
		parts = append(parts, fmt.Sprintf("email alat=%s applicant=%s", maskEmail(strings.ToLower(v)), maskEmail(email)))
	}
	if v := normalizeName(d.FirstName); v != "" && v != normalizeName(firstName) {
		parts = append(parts, "first_name differs")
	}
	if v := normalizeName(d.LastName); v != "" && v != normalizeName(lastName) {
		parts = append(parts, "last_name differs")
	}
	if len(parts) == 0 {
		return "no differing field"
	}
	return strings.Join(parts, "; ")
}
