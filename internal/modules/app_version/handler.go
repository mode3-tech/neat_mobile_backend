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

	var dto AppVersionInfoResponse
	switch appVersion.AppOS {
	case "android":
		dto.Android = &android{
			MinBuild:    appVersion.MinBuild,
			LatestBuild: appVersion.LatestBuild,
			StoreURL:    appVersion.StoreURL,
		}
	case "ios":
		dto.Ios = &ios{
			MinBuild:    appVersion.MinBuild,
			LatestBuild: appVersion.LatestBuild,
			StoreURL:    appVersion.StoreURL,
		}

	}

	c.JSON(http.StatusOK, response.APIResponse[AppVersionInfoResponse]{
		Status:  "success",
		Message: "App version info successfully fetched",
		Data:    &dto,
	})
}
