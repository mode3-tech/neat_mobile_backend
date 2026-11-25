package registerv2

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"neat_mobile_app_backend/models"
)

// Wema's face biometric web app identifies the document it checked as "bvn"
// or "nin".
const (
	wemaFaceIDTypeBVN = "bvn"
	wemaFaceIDTypeNIN = "nin"
)

var errWemaFaceRequired = &userError{msg: "face biometric verification with wema is required - please complete it and try again"}

// SetWemaFaceConfig sets Wema's face biometric web app: its base URL, the
// x-api-key it takes as x_tk, and the public URL of our facial-verification
// callback (cb_uri), which Wema has whitelisted. Wema reports the result there
// and we use the c_id it sends; the client never supplies one.
func (s *Service) SetWemaFaceConfig(baseURL, apiKey, callbackURL string) {
	s.wemaFaceBaseURL = strings.TrimSpace(baseURL)
	s.wemaFaceAPIKey = strings.TrimSpace(apiKey)
	s.wemaFaceCallbackURL = strings.TrimSpace(callbackURL)
}

// WemaFaceURL builds the URL the app opens so the customer can do Wema's face
// biometric check for their BVN or NIN. cb_uri tells Wema where to report the
// result.
func (s *Service) WemaFaceURL(idType, id string) (string, error) {
	idType = strings.ToLower(strings.TrimSpace(idType))
	id = strings.TrimSpace(id)
	if idType != wemaFaceIDTypeBVN && idType != wemaFaceIDTypeNIN {
		return "", userErrorf("id_type must be bvn or nin")
	}
	if id == "" {
		return "", userErrorf("id is required")
	}
	if s.wemaFaceBaseURL == "" || s.wemaFaceAPIKey == "" || s.wemaFaceCallbackURL == "" {
		log.Printf("[registerv2] level=ERROR op=wema_face_url: not configured (base_url_set=%t api_key_set=%t callback_url_set=%t) - set ALAT_FACE_VERIFICATION_URL, ALAT_API_KEY and ALAT_FACE_CALLBACK_URL (the URL Wema whitelisted)",
			s.wemaFaceBaseURL != "", s.wemaFaceAPIKey != "", s.wemaFaceCallbackURL != "")
		return "", userErrorf("wema face verification is not available right now")
	}
	log.Printf("[registerv2] level=INFO op=wema_face_url id_type=%s id=%s base_url=%s cb_uri=%s",
		idType, maskIdentityNumber(id), s.wemaFaceBaseURL, s.wemaFaceCallbackURL)
	q := url.Values{}
	q.Set(idType, id)
	q.Set("x_tk", s.wemaFaceAPIKey)
	q.Set("cb_uri", s.wemaFaceCallbackURL)
	return strings.TrimRight(s.wemaFaceBaseURL, "/") + "/?" + q.Encode(), nil
}

// RecordWemaFaceVerification saves a result Wema's face web app reported to
// the facial-verification callback for a BVN or NIN: a verified one carries the
// correlationId registerWema needs; a failed one carries Wema's error text,
// for the status endpoint to show.
func (s *Service) RecordWemaFaceVerification(ctx context.Context, idType, id, correlationID string, success bool, errorMessage string) error {
	idType = strings.ToLower(strings.TrimSpace(idType))
	if idType != wemaFaceIDTypeBVN && idType != wemaFaceIDTypeNIN {
		return userErrorf("unsupported id_type %q", idType)
	}
	id, correlationID, errorMessage = strings.TrimSpace(id), strings.TrimSpace(correlationID), strings.TrimSpace(errorMessage)
	if id == "" {
		return userErrorf("id is required")
	}
	status := models.WemaFaceStatusVerified
	if !success {
		status = models.WemaFaceStatusFailed
	} else if correlationID == "" {
		return userErrorf("correlation id is required for a successful face verification")
	}
	if err := s.repo.CreateWemaFaceVerification(ctx, idType, id, correlationID, status, errorMessage, time.Now().UTC()); err != nil {
		log.Printf("[registerv2] level=ERROR op=wema_face_record id_type=%s id=%s status=%s c_id=%s: save failed: %v", idType, maskIdentityNumber(id), status, correlationID, err)
		return err
	}
	return nil
}

// Wema face check states the status endpoint reports.
const (
	WemaFaceStatusPending  = "pending"
	WemaFaceStatusVerified = models.WemaFaceStatusVerified
	WemaFaceStatusFailed   = models.WemaFaceStatusFailed
)

// WemaFaceStatus reports where the customer's Wema face check stands, from the
// latest unexpired result Wema's callback delivered for the BVN or NIN:
// "pending" if none has arrived yet, otherwise "verified" or "failed" (with
// Wema's error text as the message). It never reveals the c_id.
func (s *Service) WemaFaceStatus(ctx context.Context, idType, id string) (status, message string, err error) {
	idType = strings.ToLower(strings.TrimSpace(idType))
	id = strings.TrimSpace(id)
	if idType != wemaFaceIDTypeBVN && idType != wemaFaceIDTypeNIN {
		return "", "", userErrorf("id_type must be bvn or nin")
	}
	if id == "" {
		return "", "", userErrorf("id is required")
	}
	rec, err := s.repo.GetLatestWemaFaceVerification(ctx, idType, id, "", time.Now().UTC())
	if err != nil {
		return "", "", err
	}
	switch {
	case rec == nil:
		status, message = WemaFaceStatusPending, "waiting for wema's face verification result"
	case rec.Status == models.WemaFaceStatusFailed:
		status, message = WemaFaceStatusFailed, rec.ErrorMessage
		if message == "" {
			message = "face verification was not successful, please try again"
		}
	default:
		status, message = WemaFaceStatusVerified, "face verification completed"
	}
	log.Printf("[registerv2] level=INFO op=wema_face_status id_type=%s id=%s status=%s", idType, maskIdentityNumber(id), status)
	return status, message, nil
}

// wemaFaceCorrelation returns Wema's latest unexpired, verified correlationId
// for the applicant's BVN, or else NIN, as delivered to our callback. It isn't
// consumed: a retry (resuming a pending registration, reusing an account) must
// not need another face check.
func (s *Service) wemaFaceCorrelation(ctx context.Context, rl *opLogger, bvn, nin string) (string, error) {
	now := time.Now().UTC()
	for _, c := range []struct{ idType, id string }{{wemaFaceIDTypeBVN, bvn}, {wemaFaceIDTypeNIN, nin}} {
		if strings.TrimSpace(c.id) == "" {
			continue
		}
		rec, err := s.repo.GetLatestWemaFaceVerification(ctx, c.idType, c.id, models.WemaFaceStatusVerified, now)
		if err != nil {
			return "", err
		}
		if rec != nil {
			rl.Infof("using wema face verification: id_type=%s c_id=%s saved_at=%s age=%s expires_at=%s",
				c.idType, rec.CorrelationID, rec.CreatedAt.Format(time.RFC3339), now.Sub(rec.CreatedAt).Round(time.Second), rec.ExpiresAt.Format(time.RFC3339))
			return rec.CorrelationID, nil
		}
		rl.Infof("no unexpired verified wema face verification for id_type=%s (ttl=%s)", c.idType, wemaFaceVerificationTTL)
	}
	return "", errWemaFaceRequired
}
