package wallet

import (
	"encoding/json"
	"log"
	appErr "neat_mobile_app_backend/internal/errors"
	"neat_mobile_app_backend/internal/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) HandleBaaSEvent(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		log.Printf("baas webhook: failed to ready body: %v", err)
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	var envelope BaaSEventEnvelope
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		log.Printf("baas webhook: invalid JSON: %v", err)
		mapped := response.MapError(appErr.ErrInvalidRequestBody)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}

	ctx := c.Request.Context()

	switch envelope.Event {
	case "customer_bank_transfer":
		var ev CustomerBankTransferWebhook
		if err := json.Unmarshal(rawBody, &ev); err != nil {
			mapped := response.MapError(appErr.ErrInvalidRequestBody)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}
		if err := h.service.ProcessCustomerBankTransfer(ctx, &ev.Data); err != nil {
			log.Printf("baas webhook: failed to process customer bank transfer: %v", err)
		}
	case "account_funded":
		log.Printf("baas webhook: account funded: %s", rawBody)
		var ev AccountFundedWebhook
		if err := json.Unmarshal(rawBody, &ev); err != nil {
			mapped := response.MapError(appErr.ErrInvalidRequestBody)
			c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
				Status: "error",
				Error:  &mapped.Error,
			})
			return
		}
		if err := h.service.ProcessAccountFunded(ctx, &ev.Data); err != nil {
			log.Printf("baas webhook: failed to process account funded: %v", err)
		}
	default:
		log.Printf("baas webhook: unknown event: %s", envelope.Event)
	}

	c.JSON(http.StatusOK, response.APIResponse[any]{
		Status: "success",
	})
}
