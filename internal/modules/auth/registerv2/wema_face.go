package registerv2

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"
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
// callback. The callback is not passed to the app for now (the client reads
// the c_id from the web app's final URL and sends it to /register), so
// callbackURL is only logged; the callback route itself stays available.
func (s *Service) SetWemaFaceConfig(baseURL, apiKey, callbackURL string) {
	s.wemaFaceBaseURL = strings.TrimSpace(baseURL)
	s.wemaFaceAPIKey = strings.TrimSpace(apiKey)
	s.wemaFaceCallbackURL = strings.TrimSpace(callbackURL)
}

// WemaFaceURL builds the URL the app opens so the customer can do Wema's face
// biometric check for their BVN or NIN.
func (s *Service) WemaFaceURL(idType, id string) (string, error) {
	idType = strings.ToLower(strings.TrimSpace(idType))
	id = strings.TrimSpace(id)
	if idType != wemaFaceIDTypeBVN && idType != wemaFaceIDTypeNIN {
		return "", userErrorf("id_type must be bvn or nin")
	}
	if id == "" {
		return "", userErrorf("id is required")
	}
	if s.wemaFaceBaseURL == "" || s.wemaFaceAPIKey == "" {
		log.Printf("[registerv2] level=ERROR op=wema_face_url: not configured (base_url_set=%t api_key_set=%t) - set ALAT_FACE_VERIFICATION_URL and ALAT_API_KEY",
			s.wemaFaceBaseURL != "", s.wemaFaceAPIKey != "")
		return "", userErrorf("wema face verification is not available right now")
	}
	log.Printf("[registerv2] level=INFO op=wema_face_url id_type=%s id=%s base_url=%s (no cb_uri/rd_uri: the client reads c_id from the web app's final URL)",
		idType, maskIdentityNumber(id), s.wemaFaceBaseURL)
	q := url.Values{}
	q.Set(idType, id)
	q.Set("x_tk", s.wemaFaceAPIKey)
	return strings.TrimRight(s.wemaFaceBaseURL, "/") + "/?" + q.Encode(), nil
}

// RecordWemaFaceVerification saves the correlationId Wema's face web app
// reported for a BVN or NIN, for registerWema to use. Called by the
// facial-verification callback.
func (s *Service) RecordWemaFaceVerification(ctx context.Context, idType, id, correlationID string) error {
	idType = strings.ToLower(strings.TrimSpace(idType))
	if idType != wemaFaceIDTypeBVN && idType != wemaFaceIDTypeNIN {
		return userErrorf("unsupported id_type %q", idType)
	}
	id, correlationID = strings.TrimSpace(id), strings.TrimSpace(correlationID)
	if id == "" || correlationID == "" {
		return userErrorf("id and correlation id are required")
	}
	if err := s.repo.CreateWemaFaceVerification(ctx, idType, id, correlationID, time.Now().UTC()); err != nil {
		log.Printf("[registerv2] level=ERROR op=wema_face_record id_type=%s id=%s c_id=%s: save failed: %v", idType, maskIdentityNumber(id), correlationID, err)
		return err
	}
	return nil
}

// resolveWemaFaceCorrelation picks the correlationId for ALAT's tier 2
// creation: the one the client read from Wema's face web app and sent in the
// /register body, else the latest one Wema's callback stored. We can't check
// a client-sent ID ourselves; ALAT rejects one it didn't issue for this
// BVN/NIN when the account is created. A client-sent ID is also saved so a
// retry that omits it still works.
func (s *Service) resolveWemaFaceCorrelation(ctx context.Context, rl *opLogger, fromRequest, bvn, nin string) (string, error) {
	if cid := strings.TrimSpace(fromRequest); cid != "" {
		rl.Infof("using wema face correlation id sent by the client: c_id=%s", cid)
		if err := s.RecordWemaFaceVerification(ctx, wemaFaceIDTypeBVN, bvn, cid); err != nil {
			rl.Warnf("could not save the client-sent wema c_id for retries: %v", err)
		}
		return cid, nil
	}
	rl.Infof("no wema c_id in the request - looking for one stored from wema's callback")
	return s.wemaFaceCorrelation(ctx, rl, bvn, nin)
}

// wemaFaceCorrelation returns Wema's latest unexpired correlationId for the
// applicant's BVN, or else NIN. It isn't consumed: a retry (resuming a pending
// registration, reusing an account) must not need another face check.
func (s *Service) wemaFaceCorrelation(ctx context.Context, rl *opLogger, bvn, nin string) (string, error) {
	now := time.Now().UTC()
	for _, c := range []struct{ idType, id string }{{wemaFaceIDTypeBVN, bvn}, {wemaFaceIDTypeNIN, nin}} {
		if strings.TrimSpace(c.id) == "" {
			continue
		}
		rec, err := s.repo.GetLatestWemaFaceVerification(ctx, c.idType, c.id, now)
		if err != nil {
			return "", err
		}
		if rec != nil {
			rl.Infof("using wema face verification: id_type=%s c_id=%s saved_at=%s age=%s expires_at=%s",
				c.idType, rec.CorrelationID, rec.CreatedAt.Format(time.RFC3339), now.Sub(rec.CreatedAt).Round(time.Second), rec.ExpiresAt.Format(time.RFC3339))
			return rec.CorrelationID, nil
		}
		rl.Infof("no unexpired wema face verification for id_type=%s (ttl=%s)", c.idType, wemaFaceVerificationTTL)
	}
	return "", errWemaFaceRequired
}
