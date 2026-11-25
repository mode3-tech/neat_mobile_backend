package wallet

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/middleware"
	"neat_mobile_app_backend/internal/response"

	"github.com/gin-gonic/gin"
)

// alatAuthRequest / alatAuthResponse are ALAT's Authentication Callback
// models (playground.alat.ng/get-started).
type alatAuthRequest struct {
	TransactionReference string `json:"transactionReference"`
	SecurityInfo         string `json:"securityInfo"`
}

type alatAuthResponse struct {
	TransactionReference string `json:"transactionReference"`
	Authorized           bool   `json:"authorized"`
}

// HandleAlatAuthentication is ALAT's Authentication Callback. It always
// answers 200 with authorized true/false - ALAT reads the flag, not the
// status code.
func (h *Handler) HandleAlatAuthentication(c *gin.Context) {
	var req alatAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("wallet handler: alat auth callback: invalid body: %v", err)
		c.JSON(http.StatusOK, alatAuthResponse{Authorized: false})
		return
	}
	ok := h.service.AuthorizeAlatTransaction(c.Request.Context(), req.TransactionReference, req.SecurityInfo)
	c.JSON(http.StatusOK, alatAuthResponse{TransactionReference: req.TransactionReference, Authorized: ok})
}

// HandleAlatAccountCreation is ALAT's account creation webhook, sent when a
// wallet requested via tier 2 account creation is generated. It hands the
// phone number to the registration listener, which finishes any registration
// waiting on that account - in the background, so ALAT gets its 200 at once.
// The listener confirms the account with ALAT itself; this payload is never
// trusted on its own.
// It always answers 200 once the body is read, so ALAT does not retry an
// event we have already seen.
func (h *Handler) HandleAlatAccountCreation(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR event=account_creation: read body failed: %v", err)
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}

	var ev AlatWebhook[AlatAccountCreationData]
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		log.Printf("[ALAT webhook] level=ERROR event=account_creation: invalid JSON: %v", err)
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}

	if ev.Data == nil {
		log.Printf("[ALAT webhook] level=WARN event=account_creation request_type=%d title=%q message=%q: no data",
			ev.RequestType, ev.Title, ev.Message)
	} else {
		log.Printf("[ALAT webhook] level=INFO event=account_creation request_type=%d title=%q message=%q account=%s status=%q type=%d phone=%s",
			ev.RequestType, ev.Title, ev.Message, maskAccount(ev.Data.Nuban), ev.Data.NubanStatus, ev.Data.Type, maskAccount(ev.Data.PhoneNumber))
		if phone := strings.TrimSpace(ev.Data.PhoneNumber); phone != "" && h.alatAccountListener != nil {
			listener := h.alatAccountListener
			ctx := context.WithoutCancel(c.Request.Context())
			go func() {
				ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				listener.OnAlatAccountCreated(ctx, phone)
			}()
		}
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Webhook received",
	})
}

// HandleAlatTransaction is ALAT's Transaction Callback, sent when a debit
// (ProcessClientTransfer) or payout (FundWallet) changes status. The payload
// only triggers settlement - the service confirms the status with ALAT. It
// answers 200 once the body is read; a transaction it can't settle now is
// left pending for the reconciliation sweep.
func (h *Handler) HandleAlatTransaction(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR event=transaction: read body failed: %v", err)
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}

	var ev AlatWebhook[AlatTransactionCallbackData]
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		log.Printf("[ALAT webhook] level=ERROR event=transaction: invalid JSON: %v", err)
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}

	if ev.Data == nil {
		log.Printf("[ALAT webhook] level=WARN event=transaction request=%d request_type=%d title=%q message=%q: no data",
			ev.Request, ev.RequestType, ev.Title, ev.Message)
	} else {
		log.Printf("[ALAT webhook] level=INFO event=transaction request=%d request_type=%d ref=%s platform_ref=%s stan=%s status=%q message=%q",
			ev.Request, ev.RequestType, ev.Data.TransactionReference, ev.Data.PlatformTransactionReference,
			ev.Data.TransactionStan, ev.Data.Status, ev.Data.Message)
		h.service.HandleAlatTransactionCallback(c.Request.Context(), ev.Data.TransactionReference)
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Webhook received",
	})
}

// HandleAlatTransactionNotification is ALAT's Transaction Notification
// webhook (prod only): one event per credit or debit on our wallets. It only
// logs for now. The payload is unsigned and has no reference to dedupe on, so
// it must not move money: our own debits and payouts settle through
// HandleAlatTransaction and the reconciliation sweep.
func (h *Handler) HandleAlatTransactionNotification(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR event=transaction_notification: read body failed: %v", err)
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}
	var ev AlatTransactionNotification
	if err := json.Unmarshal(rawBody, &ev); err != nil {
		log.Printf("[ALAT webhook] level=WARN event=transaction_notification: invalid JSON: %v body=%s", err, truncateForLog(rawBody))
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}
	log.Printf("[ALAT webhook] level=INFO event=transaction_notification account=%s type=%s amount=%.2f date=%s",
		maskAccount(ev.AccountNumber), ev.TransactionType, ev.Amount, ev.TransactionDate)
	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Webhook received",
	})
}

// wemaFaceVerificationPayload is what Wema's face biometric web app POSTs to
// our cb_uri after the customer's face check.
type wemaFaceVerificationPayload struct {
	Success       bool   `json:"success"`
	CorrelationID string `json:"c_id"`
	ID            string `json:"id"`
	IDType        string `json:"id_type"`
}

// HandleAlatFacialVerification receives the result of Wema's face biometric
// web app. On success it saves the correlationId (c_id) against the BVN/NIN it
// was done for - ALAT's tier 2 account creation needs that exact ID. The
// payload is unsigned, so nothing is trusted beyond storing it: ALAT rejects a
// correlationId it didn't issue when the account is created. It answers 200
// once the body is read, so Wema does not retry; BVN/NIN values are never
// logged in full.
func (h *Handler) HandleAlatFacialVerification(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		log.Printf("[ALAT webhook] level=ERROR event=facial_verification: read body failed: %v", err)
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}

	// Log how it arrived first: whether Wema's servers or a browser call this
	// decides if the IP allowlist can apply, and Wema's payload shape isn't
	// guaranteed. The BVN/NIN is masked below; c_id is not sensitive.
	log.Printf("[ALAT webhook] level=INFO event=facial_verification received peer=%s xff=%q origin=%q referer=%q user_agent=%q content_type=%q bytes=%d fields=%v",
		c.Request.RemoteAddr, c.GetHeader("X-Forwarded-For"), c.GetHeader("Origin"), c.GetHeader("Referer"),
		c.GetHeader("User-Agent"), c.GetHeader("Content-Type"), len(rawBody), jsonKeys(rawBody))

	var p wemaFaceVerificationPayload
	if err := json.Unmarshal(rawBody, &p); err != nil {
		log.Printf("[ALAT webhook] level=ERROR event=facial_verification: invalid JSON: %v (body not logged: may hold a BVN/NIN)", err)
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}
	p.CorrelationID, p.ID, p.IDType = strings.TrimSpace(p.CorrelationID), strings.TrimSpace(p.ID), strings.ToLower(strings.TrimSpace(p.IDType))

	switch {
	case !p.Success:
		log.Printf("[ALAT webhook] level=WARN event=facial_verification id_type=%s id=%s c_id=%q: face verification was not successful (success=false) - nothing saved", p.IDType, maskAccount(p.ID), p.CorrelationID)
	case p.CorrelationID == "" || p.ID == "":
		log.Printf("[ALAT webhook] level=WARN event=facial_verification id_type=%s has_c_id=%t has_id=%t: success without c_id or id - nothing saved, check Wema's payload shape", p.IDType, p.CorrelationID != "", p.ID != "")
	case h.wemaFaceRecorder == nil:
		log.Printf("[ALAT webhook] level=WARN event=facial_verification: no recorder wired, dropping result")
	default:
		if err := h.wemaFaceRecorder.RecordWemaFaceVerification(c.Request.Context(), p.IDType, p.ID, p.CorrelationID); err != nil {
			log.Printf("[ALAT webhook] level=ERROR event=facial_verification id_type=%s id=%s: save failed: %v", p.IDType, maskAccount(p.ID), err)
		} else {
			log.Printf("[ALAT webhook] level=INFO event=facial_verification id_type=%s id=%s c_id=%s: saved", p.IDType, maskAccount(p.ID), p.CorrelationID)
		}
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status:  "success",
		Message: "Webhook received",
	})
}

// jsonKeys lists a JSON object's top-level keys (for logging the shape of a
// payload without its values); nil if it isn't an object.
func jsonKeys(raw []byte) []string {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// truncateForLog caps a webhook body in logs.
func truncateForLog(b []byte) string {
	const max = 2048
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "...(truncated)"
	}
	return s
}

type wemaHistoryQuery struct {
	From    string `form:"from" binding:"required"` // YYYY-MM-DD
	To      string `form:"to" binding:"required"`   // YYYY-MM-DD
	Keyword string `form:"keyword"`
}

type wemaStatementRequest struct {
	DateFrom string `json:"date_from" binding:"required"` // YYYY-MM-DD or RFC 3339
	DateTo   string `json:"date_to" binding:"required"`
}

type wemaStatementInitiated struct {
	StatementID string `json:"statement_id"`
}

// GetWemaTransactionHistory returns ALAT's transaction history for the
// caller's Wema wallet.
func (h *Handler) GetWemaTransactionHistory(c *gin.Context) {
	mobileUserID, ok := requireUserID(c)
	if !ok {
		return
	}
	var q wemaHistoryQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		abortWithError(c, appErr.ErrMissingRequiredQueryParameter)
		return
	}
	from, to, err := parseDateRange(q.From, q.To)
	if err != nil {
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}
	items, err := h.service.GetWemaTransactionHistory(c.Request.Context(), mobileUserID, from, to, q.Keyword)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[[]ProviderTransaction]{
		Status:  "success",
		Message: "Transaction history fetched successfully",
		Data:    &items,
	})
}

// InitiateWemaStatement starts an ALAT statement for the caller's Wema wallet.
func (h *Handler) InitiateWemaStatement(c *gin.Context) {
	mobileUserID, ok := requireUserID(c)
	if !ok {
		return
	}
	var req wemaStatementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}
	from, to, err := parseDateRange(req.DateFrom, req.DateTo)
	if err != nil {
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}
	id, err := h.service.InitiateWemaStatement(c.Request.Context(), mobileUserID, from, to)
	if err != nil {
		abortWithError(c, err)
		return
	}
	data := wemaStatementInitiated{StatementID: id}
	c.JSON(http.StatusOK, response.APIResponse[wemaStatementInitiated]{
		Status:  "success",
		Message: "Statement requested successfully",
		Data:    &data,
	})
}

// GetWemaStatement returns the transactions of a statement started by
// InitiateWemaStatement.
func (h *Handler) GetWemaStatement(c *gin.Context) {
	mobileUserID, ok := requireUserID(c)
	if !ok {
		return
	}
	statementID := strings.TrimSpace(c.Param("statement_id"))
	if statementID == "" {
		abortWithError(c, appErr.ErrMissingRequiredPathParameter)
		return
	}
	items, err := h.service.GetWemaStatement(c.Request.Context(), mobileUserID, statementID)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[[]ProviderTransaction]{
		Status:  "success",
		Message: "Statement fetched successfully",
		Data:    &items,
	})
}

// FundWemaWallet is the internal (HMAC-authenticated) payout endpoint.
func (h *Handler) FundWemaWallet(c *gin.Context) {
	var req FundWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, appErr.ErrInvalidRequestBody)
		return
	}
	result, err := h.service.FundWemaWallet(c.Request.Context(), &req)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, response.APIResponse[*FundWalletResult]{
		Status:  "success",
		Message: "Wallet funding initiated",
		Data:    &result,
	})
}

func requireUserID(c *gin.Context) (string, bool) {
	id := strings.TrimSpace(c.GetString(middleware.UserIDContextKey))
	if id == "" {
		abortWithError(c, appErr.ErrUnauthorized)
		return "", false
	}
	return id, true
}

func abortWithError(c *gin.Context, err error) {
	mapped := response.MapError(err)
	c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
		Status: "error",
		Error:  &mapped.Error,
	})
}

// parseDateRange accepts YYYY-MM-DD or RFC 3339 dates. A date-only "to" is
// taken as the end of that day.
func parseDateRange(fromRaw, toRaw string) (time.Time, time.Time, error) {
	from, _, err := parseFlexibleDate(fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, toDateOnly, err := parseFlexibleDate(toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if toDateOnly {
		to = to.Add(24*time.Hour - time.Second)
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, appErr.ErrInvalidRequestBody
	}
	return from, to, nil
}

func parseFlexibleDate(raw string) (time.Time, bool, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), true, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, err
	}
	return t.UTC(), false, nil
}
