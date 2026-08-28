package registerversion

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

func (h *Handler) GetRegisterVersion(c *gin.Context) {
	preference, err := h.service.GetRegisterVersion()
	if err != nil {
		mapped := response.MapError(err)
		c.AbortWithStatusJSON(mapped.Status, response.APIResponse[any]{
			Status: "error",
			Error:  &mapped.Error,
		})
		return
	}
	c.JSON(http.StatusOK, response.APIResponse[RegisterVersionResponse]{
		Status: "success",
		Data:   &RegisterVersionResponse{Version: preference.PreferenceValue},
	})
}
