package appversion

import (
	"neat_mobile_app_backend/internal/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetAppVersion(c *gin.Context) {
	os := c.Param("os")

	appVersion, err := h.service.GetAppVersion(c.Request.Context(), os)
	if err != nil {
		mapped := response.MapError(err)
		c.JSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}
	c.JSON(http.StatusOK, appVersion)
}
